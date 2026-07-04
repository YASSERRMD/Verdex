package airgapped_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
)

func TestNetworkPolicy_AllowsLoopback(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}

	for _, addr := range []string{
		"localhost",
		"127.0.0.1",
		"127.0.0.1:11434",
		"http://localhost:11434/v1/chat",
		"::1",
	} {
		if err := policy.CheckAddress(addr); err != nil {
			t.Errorf("CheckAddress(%q) = %v, want nil", addr, err)
		}
	}
}

func TestNetworkPolicy_BlocksDisallowedAddress(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}

	err = policy.CheckAddress("api.openai.com:443")
	if !errors.Is(err, airgapped.ErrDisallowedAddress) {
		t.Fatalf("CheckAddress() = %v, want ErrDisallowedAddress", err)
	}
}

func TestNetworkPolicy_AllowsExplicitAllowlistEntry(t *testing.T) {
	// validProfile configures "192.168.1.50:11434" as an allowed target.
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}

	if err := policy.CheckAddress("192.168.1.50:11434"); err != nil {
		t.Fatalf("CheckAddress(allow-listed) = %v, want nil", err)
	}
	if err := policy.CheckAddress("192.168.1.51:11434"); !errors.Is(err, airgapped.ErrDisallowedAddress) {
		t.Fatalf("CheckAddress(non-allow-listed) = %v, want ErrDisallowedAddress", err)
	}
}

func TestNetworkPolicy_EmptyAddress(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}
	if err := policy.CheckAddress(""); !errors.Is(err, airgapped.ErrEmptyAddress) {
		t.Fatalf("CheckAddress(\"\") = %v, want ErrEmptyAddress", err)
	}
}

func TestNewNetworkPolicy_NilProfile(t *testing.T) {
	if _, err := airgapped.NewNetworkPolicy(nil); !errors.Is(err, airgapped.ErrNilProfile) {
		t.Fatalf("NewNetworkPolicy(nil) error = %v, want ErrNilProfile", err)
	}
}

func TestGuardedDialContext_BlocksDisallowedBeforeDialing(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}

	dialed := false
	fake := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = true
		return nil, nil
	}
	guarded := airgapped.GuardedDialContext(policy, fake)

	_, err = guarded(context.Background(), "tcp", "evil.example.com:443")
	if !errors.Is(err, airgapped.ErrDisallowedAddress) {
		t.Fatalf("guarded dial error = %v, want ErrDisallowedAddress", err)
	}
	if dialed {
		t.Fatal("inner dialer was called for a disallowed address")
	}
}

func TestGuardedDialContext_AllowsLoopbackThroughToInner(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}

	dialed := false
	fake := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = true
		return nil, nil
	}
	guarded := airgapped.GuardedDialContext(policy, fake)

	if _, err := guarded(context.Background(), "tcp", "127.0.0.1:11434"); err != nil {
		t.Fatalf("guarded dial error = %v, want nil", err)
	}
	if !dialed {
		t.Fatal("inner dialer was not called for an allowed address")
	}
}

func TestNewGuardedDialer_Builds(t *testing.T) {
	profile := validProfile(t)
	policy, err := airgapped.NewNetworkPolicy(profile)
	if err != nil {
		t.Fatalf("NewNetworkPolicy: %v", err)
	}
	dialer := airgapped.NewGuardedDialer(policy)
	if dialer == nil {
		t.Fatal("NewGuardedDialer returned nil")
	}
}
