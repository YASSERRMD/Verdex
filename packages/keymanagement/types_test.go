package keymanagement_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/keymanagement"
)

func TestKeyMetadata_Validate_RequiresID(t *testing.T) {
	m := &keymanagement.KeyMetadata{
		TenantID: uuid.New(),
		Version:  1,
		State:    keymanagement.KeyStateActive,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want ErrEmptyKeyID")
	}
}

func TestKeyMetadata_Validate_RequiresTenantID(t *testing.T) {
	m := &keymanagement.KeyMetadata{
		ID:      "k1",
		Version: 1,
		State:   keymanagement.KeyStateActive,
	}
	if err := m.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want ErrEmptyTenantID")
	}
}

func TestKeyMetadata_Validate_RequiresValidState(t *testing.T) {
	m := &keymanagement.KeyMetadata{
		ID:       "k1",
		TenantID: uuid.New(),
		Version:  1,
		State:    "bogus",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want ErrInvalidKeyState")
	}
}

func TestKeyMetadata_Validate_Success(t *testing.T) {
	m := &keymanagement.KeyMetadata{
		ID:        "k1",
		TenantID:  uuid.New(),
		Version:   1,
		State:     keymanagement.KeyStateActive,
		CreatedAt: time.Now(),
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestKeyMetadata_Validate_NilReceiver(t *testing.T) {
	var m *keymanagement.KeyMetadata
	if err := m.Validate(); err == nil {
		t.Fatal("Validate() on nil receiver error = nil, want ErrNilKeyMetadata")
	}
}

func TestKeyState_IsValid(t *testing.T) {
	valid := []keymanagement.KeyState{
		keymanagement.KeyStateActive,
		keymanagement.KeyStateRotating,
		keymanagement.KeyStateRetired,
		keymanagement.KeyStateRevoked,
	}
	for _, s := range valid {
		if !s.IsValid() {
			t.Errorf("KeyState(%q).IsValid() = false, want true", s)
		}
	}
	if keymanagement.KeyState("bogus").IsValid() {
		t.Error(`KeyState("bogus").IsValid() = true, want false`)
	}
}
