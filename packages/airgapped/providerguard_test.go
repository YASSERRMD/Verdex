package airgapped_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// stubProvider is a minimal provider.LLMProvider for guard tests; only
// ID/Capabilities matter here.
type stubProvider struct{ id string }

func (s *stubProvider) ID() string { return s.id }
func (s *stubProvider) Capabilities() provider.Capability {
	return provider.Capability{ProviderID: s.id}
}
func (s *stubProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	return nil, nil
}
func (s *stubProvider) ChatStream(ctx context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (s *stubProvider) Embed(ctx context.Context, req provider.EmbedRequest) (*provider.EmbedResponse, error) {
	return nil, nil
}
func (s *stubProvider) HealthCheck(ctx context.Context) error { return nil }

func TestIsLocalProviderID(t *testing.T) {
	cases := map[string]bool{
		"local:llama3:8b":  true,
		"local:":           true,
		"openai:gpt-4":     false,
		"anthropic:claude": false,
		"":                 false,
	}
	for id, want := range cases {
		if got := airgapped.IsLocalProviderID(id); got != want {
			t.Errorf("IsLocalProviderID(%q) = %v, want %v", id, got, want)
		}
	}
}

func TestGuardRegister_AllowsLocalProvider(t *testing.T) {
	profile := validProfile(t)
	p := &stubProvider{id: "local:llama3"}
	if err := airgapped.GuardRegister(profile, "local:llama3", p); err != nil {
		t.Fatalf("GuardRegister() = %v, want nil", err)
	}
}

func TestGuardRegister_RejectsNonLocalProvider(t *testing.T) {
	profile := validProfile(t)
	p := &stubProvider{id: "openai:gpt-4"}
	err := airgapped.GuardRegister(profile, "openai:gpt-4", p)
	if !errors.Is(err, airgapped.ErrNonLocalProvider) {
		t.Fatalf("GuardRegister() = %v, want ErrNonLocalProvider", err)
	}
}

func TestGuardRegister_NilProfile(t *testing.T) {
	p := &stubProvider{id: "local:llama3"}
	err := airgapped.GuardRegister(nil, "local:llama3", p)
	if !errors.Is(err, airgapped.ErrNilProfile) {
		t.Fatalf("GuardRegister() = %v, want ErrNilProfile", err)
	}
}

func TestGuardRegistry_RegistersOnlyLocalProviders(t *testing.T) {
	profile := validProfile(t)
	reg := provider.NewRegistry()

	if err := airgapped.GuardRegistry(profile, reg, "local:llama3", &stubProvider{id: "local:llama3"}); err != nil {
		t.Fatalf("GuardRegistry() (local) = %v, want nil", err)
	}
	if err := airgapped.GuardRegistry(profile, reg, "openai:gpt-4", &stubProvider{id: "openai:gpt-4"}); !errors.Is(err, airgapped.ErrNonLocalProvider) {
		t.Fatalf("GuardRegistry() (non-local) = %v, want ErrNonLocalProvider", err)
	}

	ids := reg.List()
	if len(ids) != 1 || ids[0] != "local:llama3" {
		t.Fatalf("registry.List() = %v, want [local:llama3]", ids)
	}
}

func TestGuardRegistry_NilRegistry(t *testing.T) {
	profile := validProfile(t)
	err := airgapped.GuardRegistry(profile, nil, "local:llama3", &stubProvider{id: "local:llama3"})
	if !errors.Is(err, airgapped.ErrNilRegistry) {
		t.Fatalf("GuardRegistry() = %v, want ErrNilRegistry", err)
	}
}

func TestAuditRegistry_ReportsNonLocalViolations(t *testing.T) {
	reg := provider.NewRegistry()
	mustRegister(t, reg, "local:llama3", &stubProvider{id: "local:llama3"})
	mustRegister(t, reg, "openai:gpt-4", &stubProvider{id: "openai:gpt-4"})

	violations := airgapped.AuditRegistry(reg)
	if len(violations) != 1 || violations[0] != "openai:gpt-4" {
		t.Fatalf("AuditRegistry() = %v, want [openai:gpt-4]", violations)
	}
}

func TestAuditRegistry_AllLocalIsClean(t *testing.T) {
	reg := provider.NewRegistry()
	mustRegister(t, reg, "local:llama3", &stubProvider{id: "local:llama3"})

	if violations := airgapped.AuditRegistry(reg); len(violations) != 0 {
		t.Fatalf("AuditRegistry() = %v, want empty", violations)
	}
}

func TestAuditRegistry_NilRegistry(t *testing.T) {
	if violations := airgapped.AuditRegistry(nil); violations != nil {
		t.Fatalf("AuditRegistry(nil) = %v, want nil", violations)
	}
}

func mustRegister(t *testing.T, reg *provider.Registry, id string, p provider.LLMProvider) {
	t.Helper()
	if err := reg.Register(id, p); err != nil {
		t.Fatalf("Register(%q): %v", id, err)
	}
}
