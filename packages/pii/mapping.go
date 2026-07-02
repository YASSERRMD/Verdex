package pii

import (
	"context"
	"fmt"
	"sync"
)

// AccessPolicy gates whether a requester may reverse a pseudonymization,
// i.e. look up the original value behind a pseudonym token.
//
// Implementations typically check the requester's role, tenant scope, or a
// break-glass audit justification before allowing reveal. There is no
// default permissive implementation in this package: callers must supply
// one, making "no policy configured" a compile-time decision rather than an
// accidental leak.
type AccessPolicy interface {
	// CanReveal reports whether requester is authorized to reveal (reverse)
	// pseudonym mappings. requester is an opaque identifier (user ID, role
	// name, service name) meaningful to the implementation.
	CanReveal(ctx context.Context, requester string) bool
}

// AccessPolicyFunc adapts a plain function to the AccessPolicy interface.
type AccessPolicyFunc func(ctx context.Context, requester string) bool

// CanReveal implements AccessPolicy.
func (f AccessPolicyFunc) CanReveal(ctx context.Context, requester string) bool {
	return f(ctx, requester)
}

// DenyAllAccessPolicy is an AccessPolicy that never permits reveal. Useful
// as an explicit, safe default and in tests asserting that access control
// is actually enforced.
type DenyAllAccessPolicy struct{}

// CanReveal implements AccessPolicy.
func (DenyAllAccessPolicy) CanReveal(context.Context, string) bool { return false }

// mappingEntry is the stored original<->token pairing for one pseudonymized
// entity.
type mappingEntry struct {
	category PIICategory
	original string
	token    string
	// irreversible marks a token allocated under ModeIrreversibleRedact: no
	// original value is stored (see TokenFor), and Reveal must always
	// return ErrAlreadyIrreversible for it regardless of AccessPolicy.
	irreversible bool
}

// PseudonymMap stores the original<->pseudonym-token mapping produced by a
// Redactor operating in ModePseudonymize, gated behind an AccessPolicy so
// only authorized callers can reverse a pseudonymization back to the
// original value.
//
// The same original text value always maps to the same token within a
// given category, for the lifetime of the PseudonymMap, so repeated
// occurrences of the same entity (e.g. the same person's name appearing
// multiple times in a document) are pseudonymized consistently.
type PseudonymMap struct {
	mu       sync.RWMutex
	policy   AccessPolicy
	counters map[PIICategory]int
	byToken  map[string]*mappingEntry
	byValue  map[string]*mappingEntry // key: category + "\x00" + original
}

// NewPseudonymMap constructs an empty PseudonymMap gated by policy. If
// policy is nil, DenyAllAccessPolicy is used so reveal is denied by default
// rather than silently permissive.
func NewPseudonymMap(policy AccessPolicy) *PseudonymMap {
	if policy == nil {
		policy = DenyAllAccessPolicy{}
	}
	return &PseudonymMap{
		policy:   policy,
		counters: make(map[PIICategory]int),
		byToken:  make(map[string]*mappingEntry),
		byValue:  make(map[string]*mappingEntry),
	}
}

// tokenPrefix returns the token prefix used for a category (e.g. "PERSON"
// for CategoryName), matching the "PERSON_1"-style token format.
func tokenPrefix(category PIICategory) string {
	switch category {
	case CategoryName:
		return "PERSON"
	case CategoryContact:
		return "CONTACT"
	case CategoryIdentifier:
		return "ID"
	case CategoryAddress:
		return "ADDRESS"
	case CategoryFinancial:
		return "FINANCIAL"
	default:
		return "PII"
	}
}

func valueKey(category PIICategory, original string) string {
	return string(category) + "\x00" + original
}

// TokenFor returns the stable pseudonym token for (category, original),
// allocating a new one on first use and recording the mapping. Subsequent
// calls with the same (category, original) pair return the same token.
func (p *PseudonymMap) TokenFor(category PIICategory, original string) string {
	key := valueKey(category, original)

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.byValue[key]; ok {
		return existing.token
	}

	p.counters[category]++
	token := fmt.Sprintf("%s_%d", tokenPrefix(category), p.counters[category])

	entry := &mappingEntry{category: category, original: original, token: token}
	p.byValue[key] = entry
	p.byToken[token] = entry
	return token
}

// TokenForIrreversible allocates a stable pseudonym token for (category,
// original) exactly like TokenFor, but records no recoverable mapping: the
// original value is discarded immediately rather than stored. Reveal always
// returns ErrAlreadyIrreversible for tokens allocated this way, regardless
// of AccessPolicy. Used by Redactor/PIIService under ModeIrreversibleRedact
// when a stable-but-unreversible token (rather than a flat "[REDACTED:...]"
// placeholder) is desired; see redact.go and policy.go for the more common
// placeholder-based irreversible path.
func (p *PseudonymMap) TokenForIrreversible(category PIICategory, original string) string {
	key := valueKey(category, original)

	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.byValue[key]; ok && existing.irreversible {
		return existing.token
	}

	p.counters[category]++
	token := fmt.Sprintf("%s_%d", tokenPrefix(category), p.counters[category])

	entry := &mappingEntry{category: category, original: "", token: token, irreversible: true}
	p.byValue[key] = entry
	p.byToken[token] = entry
	return token
}

// Reveal returns the original value behind token, if requester is
// authorized per the configured AccessPolicy and the token was not
// allocated irreversibly.
//
// Returns ErrAccessDenied if the policy denies requester, ErrUnknownToken
// if token is not present in this map, or ErrAlreadyIrreversible if token
// was allocated via TokenForIrreversible (no original value was ever
// stored).
func (p *PseudonymMap) Reveal(ctx context.Context, requester, token string) (string, error) {
	p.mu.RLock()
	entry, ok := p.byToken[token]
	p.mu.RUnlock()

	if !ok {
		return "", ErrUnknownToken
	}
	if entry.irreversible {
		return "", ErrAlreadyIrreversible
	}
	if !p.policy.CanReveal(ctx, requester) {
		return "", ErrAccessDenied
	}
	return entry.original, nil
}

// Len returns the number of distinct entities currently tracked (both
// reversible and irreversible).
func (p *PseudonymMap) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.byToken)
}
