package stt_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := stt.NewRegistry()
	p := stt.DefaultNoOpSTTProvider()

	if err := r.Register(p.ID(), p); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	got, err := r.Get(p.ID())
	if err != nil {
		t.Fatalf("Get() unexpected error: %v", err)
	}
	if got.ID() != p.ID() {
		t.Errorf("Get() returned provider with ID %q, want %q", got.ID(), p.ID())
	}
}

func TestRegistry_GetMissing_ReturnsErrProviderNotFound(t *testing.T) {
	r := stt.NewRegistry()

	_, err := r.Get("does-not-exist")
	if err == nil {
		t.Fatal("Get() on missing provider expected error, got nil")
	}
	if !errors.Is(err, stt.ErrProviderNotFound) {
		t.Errorf("Get() error %v does not wrap ErrProviderNotFound", err)
	}
}

func TestRegistry_List_ReturnsRegisteredIDs(t *testing.T) {
	r := stt.NewRegistry()

	providers := []*stt.NoOpSTTProvider{
		{FixedText: "a"},
		{FixedText: "b"},
	}

	if err := r.Register("alpha", providers[0]); err != nil {
		t.Fatalf("Register(alpha): %v", err)
	}
	if err := r.Register("beta", providers[1]); err != nil {
		t.Fatalf("Register(beta): %v", err)
	}

	ids := r.List()
	if len(ids) != 2 {
		t.Fatalf("List() returned %d IDs, want 2", len(ids))
	}
	if ids[0] != "alpha" || ids[1] != "beta" {
		t.Errorf("List() returned %v, want [alpha beta]", ids)
	}
}

func TestRegistry_RegisterDuplicate_ReturnsError(t *testing.T) {
	r := stt.NewRegistry()
	p := stt.DefaultNoOpSTTProvider()

	if err := r.Register(p.ID(), p); err != nil {
		t.Fatalf("first Register() unexpected error: %v", err)
	}
	if err := r.Register(p.ID(), p); err == nil {
		t.Fatal("second Register() with same ID expected error, got nil")
	}
}

func TestRegistry_RegisterEmptyID_ReturnsError(t *testing.T) {
	r := stt.NewRegistry()
	p := stt.DefaultNoOpSTTProvider()

	err := r.Register("", p)
	if err == nil {
		t.Fatal("Register with empty ID expected error, got nil")
	}
	if !errors.Is(err, stt.ErrInvalidRequest) {
		t.Errorf("Register error %v does not wrap ErrInvalidRequest", err)
	}
}

func TestRegistry_RegisterNilProvider_ReturnsError(t *testing.T) {
	r := stt.NewRegistry()

	err := r.Register("nil-provider", nil)
	if err == nil {
		t.Fatal("Register with nil provider expected error, got nil")
	}
	if !errors.Is(err, stt.ErrInvalidRequest) {
		t.Errorf("Register error %v does not wrap ErrInvalidRequest", err)
	}
}

func TestRegistry_MustGet_PanicsOnMissing(t *testing.T) {
	r := stt.NewRegistry()

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("MustGet() on missing provider expected panic, but did not panic")
		}
	}()
	r.MustGet("nonexistent")
}
