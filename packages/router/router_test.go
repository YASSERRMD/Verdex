package router

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// --- test helpers ---

// stubbedProvider is a minimal LLMProvider that either returns a preset
// response or a preset error.
type stubbedProvider struct {
	id      string
	chatErr error
	content string
}

func (s *stubbedProvider) ID() string { return s.id }
func (s *stubbedProvider) Capabilities() provider.Capability {
	return provider.Capability{ProviderID: s.id}
}
func (s *stubbedProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return &provider.ChatResponse{Content: s.content}, nil
}
func (s *stubbedProvider) ChatStream(_ context.Context, _ provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Delta: s.content}
	ch <- provider.StreamChunk{FinishReason: "end_turn", Done: true}
	close(ch)
	return ch, nil
}
func (s *stubbedProvider) Embed(_ context.Context, _ provider.EmbedRequest) (*provider.EmbedResponse, error) {
	if s.chatErr != nil {
		return nil, s.chatErr
	}
	return &provider.EmbedResponse{}, nil
}
func (s *stubbedProvider) HealthCheck(_ context.Context) error { return nil }

// buildRouter is a test helper that wires up a Router from a policy and a set
// of providers.
func buildRouter(t *testing.T, policy RoutingPolicy, providers ...*stubbedProvider) *Router {
	t.Helper()
	reg := provider.NewRegistry()
	for _, p := range providers {
		if err := reg.Register(p.id, p); err != nil {
			t.Fatalf("register %q: %v", p.id, err)
		}
	}
	r, err := NewRouter(RouterConfig{Registry: reg, Policy: policy})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	return r
}

// --- tests ---

// TestRouter_RoutesToCorrectProviderByTaskType ensures that the router picks the
// provider listed in the task-specific route.
func TestRouter_RoutesToCorrectProviderByTaskType(t *testing.T) {
	p1 := &stubbedProvider{id: "p1", content: "hello from p1"}
	p2 := &stubbedProvider{id: "p2", content: "hello from p2"}

	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"p1"}
	policy.TaskRoutes[TaskEmbed] = []string{"p2"}

	r := buildRouter(t, policy, p1, p2)
	ctx := context.Background()

	resp, err := r.Chat(ctx, "tenant1", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if resp.Content != "hello from p1" {
		t.Errorf("expected p1 content, got %q", resp.Content)
	}

	embedResp, err := r.Embed(ctx, "tenant1", provider.EmbedRequest{Texts: []string{"test"}})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if embedResp == nil {
		t.Fatal("expected non-nil EmbedResponse")
	}
}

// TestRouter_FallsBackOnFailure verifies that when the first provider errors,
// the router transparently falls back to the second.
func TestRouter_FallsBackOnFailure(t *testing.T) {
	failing := &stubbedProvider{id: "failing", chatErr: errors.New("upstream down")}
	ok := &stubbedProvider{id: "ok", content: "fallback response"}

	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"failing", "ok"}

	r := buildRouter(t, policy, failing, ok)

	resp, err := r.Chat(context.Background(), "t1", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("expected fallback success, got error: %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("expected fallback content, got %q", resp.Content)
	}
}

// TestRouter_RespectsCircuitBreaker confirms that a provider whose circuit
// breaker is open is skipped.
func TestRouter_RespectsCircuitBreaker(t *testing.T) {
	p1 := &stubbedProvider{id: "p1", chatErr: errors.New("broken")}
	p2 := &stubbedProvider{id: "p2", content: "from p2"}

	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"p1", "p2"}

	reg := provider.NewRegistry()
	_ = reg.Register("p1", p1)
	_ = reg.Register("p2", p2)

	cbReg := NewCircuitBreakerRegistry()
	cb := cbReg.Get("p1")
	// Manually open p1's circuit breaker.
	cb.failureThreshold = 1
	cb.Trip() // now open

	r, err := NewRouter(RouterConfig{
		Registry:   reg,
		Policy:     policy,
		CBRegistry: cbReg,
	})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}

	resp, err := r.Chat(context.Background(), "t1", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("expected p2 to serve request, got: %v", err)
	}
	if resp.Content != "from p2" {
		t.Errorf("expected p2 content, got %q", resp.Content)
	}
}

// TestRouter_TenantOverrideApplied checks that a per-tenant override takes
// precedence over the global task route.
func TestRouter_TenantOverrideApplied(t *testing.T) {
	global := &stubbedProvider{id: "global", content: "global"}
	tenantSpecific := &stubbedProvider{id: "tenant-specific", content: "tenant"}

	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"global"}
	policy.TenantOverrides["tenant-vip"] = map[TaskType][]string{
		TaskChat: {"tenant-specific"},
	}

	r := buildRouter(t, policy, global, tenantSpecific)
	ctx := context.Background()

	// VIP tenant should get tenant-specific provider.
	resp, err := r.Chat(ctx, "tenant-vip", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("VIP Chat: %v", err)
	}
	if resp.Content != "tenant" {
		t.Errorf("VIP: expected %q, got %q", "tenant", resp.Content)
	}

	// Regular tenant should get global provider.
	resp, err = r.Chat(ctx, "other-tenant", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("regular Chat: %v", err)
	}
	if resp.Content != "global" {
		t.Errorf("regular: expected %q, got %q", "global", resp.Content)
	}
}

// TestRouter_AllProvidersFailed verifies the error returned when every
// provider in the chain fails.
func TestRouter_AllProvidersFailed(t *testing.T) {
	p1 := &stubbedProvider{id: "p1", chatErr: errors.New("err1")}
	p2 := &stubbedProvider{id: "p2", chatErr: errors.New("err2")}

	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"p1", "p2"}

	r := buildRouter(t, policy, p1, p2)
	_, err := r.Chat(context.Background(), "t1", provider.ChatRequest{})
	if !errors.Is(err, ErrAllProvidersFailed) {
		t.Errorf("expected ErrAllProvidersFailed, got: %v", err)
	}
}

// TestRouter_NoProvidersAvailable verifies the error returned when the policy
// has no route for the task.
func TestRouter_NoProvidersAvailable(t *testing.T) {
	policy := DefaultPolicy()
	// No route for TaskEmbed.
	r := buildRouter(t, policy)
	_, err := r.Embed(context.Background(), "t1", provider.EmbedRequest{Texts: []string{"x"}})
	if !errors.Is(err, ErrNoProvidersAvailable) {
		t.Errorf("expected ErrNoProvidersAvailable, got: %v", err)
	}
}

// TestRouter_AirGappedViolation confirms ErrAirGappedViolation is returned
// when the only providers in the chain are not local.
func TestRouter_AirGappedViolation(t *testing.T) {
	remote := &stubbedProvider{id: "remote-provider", content: "remote"}
	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"remote-provider"}
	policy.AirGappedOnly = true

	r := buildRouter(t, policy, remote)
	_, err := r.Chat(context.Background(), "t1", provider.ChatRequest{})
	if !errors.Is(err, ErrAirGappedViolation) {
		t.Errorf("expected ErrAirGappedViolation, got: %v", err)
	}
}

// TestRouter_LocalProviderPassesAirGap confirms that a "local:" prefixed
// provider is allowed through in air-gapped mode.
func TestRouter_LocalProviderPassesAirGap(t *testing.T) {
	local := &stubbedProvider{id: "local:llama3", content: "local response"}
	policy := DefaultPolicy()
	policy.TaskRoutes[TaskChat] = []string{"local:llama3"}
	policy.AirGappedOnly = true

	r := buildRouter(t, policy, local)
	resp, err := r.Chat(context.Background(), "t1", provider.ChatRequest{})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if resp.Content != "local response" {
		t.Errorf("expected local response, got %q", resp.Content)
	}
}
