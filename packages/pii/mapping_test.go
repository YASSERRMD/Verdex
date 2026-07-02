package pii_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pii"
)

func TestPseudonymMap_TokenFor_StablePerEntity(t *testing.T) {
	pmap := pii.NewPseudonymMap(nil)

	t1 := pmap.TokenFor(pii.CategoryName, "Jane Doe")
	t2 := pmap.TokenFor(pii.CategoryName, "Jane Doe")
	t3 := pmap.TokenFor(pii.CategoryName, "John Smith")

	if t1 != t2 {
		t.Errorf("same value produced different tokens: %q vs %q", t1, t2)
	}
	if t1 == t3 {
		t.Errorf("different values produced the same token: %q", t1)
	}
	if pmap.Len() != 2 {
		t.Errorf("Len() = %d, want 2", pmap.Len())
	}
}

func TestPseudonymMap_Reveal_OnlyWhenAccessPolicyAllows(t *testing.T) {
	allow := pii.AccessPolicyFunc(func(context.Context, string) bool { return true })
	deny := pii.AccessPolicyFunc(func(context.Context, string) bool { return false })

	t.Run("allowed", func(t *testing.T) {
		pmap := pii.NewPseudonymMap(allow)
		token := pmap.TokenFor(pii.CategoryName, "Jane Doe")

		got, err := pmap.Reveal(context.Background(), "auditor-1", token)
		if err != nil {
			t.Fatalf("Reveal() error = %v", err)
		}
		if got != "Jane Doe" {
			t.Errorf("Reveal() = %q, want %q", got, "Jane Doe")
		}
	})

	t.Run("denied", func(t *testing.T) {
		pmap := pii.NewPseudonymMap(deny)
		token := pmap.TokenFor(pii.CategoryName, "Jane Doe")

		_, err := pmap.Reveal(context.Background(), "attacker-1", token)
		if !errors.Is(err, pii.ErrAccessDenied) {
			t.Fatalf("Reveal() error = %v, want ErrAccessDenied", err)
		}
	})

	t.Run("default deny-all when no policy given", func(t *testing.T) {
		pmap := pii.NewPseudonymMap(nil)
		token := pmap.TokenFor(pii.CategoryName, "Jane Doe")

		_, err := pmap.Reveal(context.Background(), "anyone", token)
		if !errors.Is(err, pii.ErrAccessDenied) {
			t.Fatalf("Reveal() error = %v, want ErrAccessDenied", err)
		}
	})
}

func TestPseudonymMap_TokenForIrreversible_LeavesNoRecoverableTrace(t *testing.T) {
	allow := pii.AccessPolicyFunc(func(context.Context, string) bool { return true })
	pmap := pii.NewPseudonymMap(allow)

	token := pmap.TokenForIrreversible(pii.CategoryFinancial, "4111-1111-1111-1111")
	if token == "" {
		t.Fatal("TokenForIrreversible() returned empty token")
	}

	// Even with an allow-all AccessPolicy, Reveal must never return the
	// original value for a token allocated irreversibly.
	_, err := pmap.Reveal(context.Background(), "auditor-1", token)
	if !errors.Is(err, pii.ErrAlreadyIrreversible) {
		t.Fatalf("Reveal() error = %v, want ErrAlreadyIrreversible", err)
	}
}

func TestPseudonymMap_TokenForIrreversible_StableToken(t *testing.T) {
	pmap := pii.NewPseudonymMap(nil)

	t1 := pmap.TokenForIrreversible(pii.CategoryFinancial, "4111-1111-1111-1111")
	t2 := pmap.TokenForIrreversible(pii.CategoryFinancial, "4111-1111-1111-1111")
	if t1 != t2 {
		t.Errorf("same value produced different irreversible tokens: %q vs %q", t1, t2)
	}
}

func TestPseudonymMap_Reveal_UnknownToken(t *testing.T) {
	pmap := pii.NewPseudonymMap(pii.AccessPolicyFunc(func(context.Context, string) bool { return true }))

	_, err := pmap.Reveal(context.Background(), "auditor-1", "PERSON_999")
	if !errors.Is(err, pii.ErrUnknownToken) {
		t.Fatalf("Reveal() error = %v, want ErrUnknownToken", err)
	}
}

func TestPseudonymMap_TokenPrefixesByCategory(t *testing.T) {
	pmap := pii.NewPseudonymMap(nil)

	tests := []struct {
		category pii.PIICategory
		prefix   string
	}{
		{pii.CategoryName, "PERSON_"},
		{pii.CategoryContact, "CONTACT_"},
		{pii.CategoryIdentifier, "ID_"},
		{pii.CategoryAddress, "ADDRESS_"},
		{pii.CategoryFinancial, "FINANCIAL_"},
		{pii.CategoryOther, "PII_"},
	}
	for _, tt := range tests {
		token := pmap.TokenFor(tt.category, "value-for-"+string(tt.category))
		if len(token) < len(tt.prefix) || token[:len(tt.prefix)] != tt.prefix {
			t.Errorf("TokenFor(%v) = %q, want prefix %q", tt.category, token, tt.prefix)
		}
	}
}
