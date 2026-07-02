package agentframework

import "strconv"

// SeedMetadataKey is the provider.ChatRequest.Metadata key a Runner sets
// when a Config carries a non-zero Seed, so a downstream provider adapter
// (packages/adapters) that supports deterministic sampling can read it
// off the request it already receives, without this package or
// packages/router needing a dedicated seed field on ChatRequest itself.
//
// provider.ChatRequest.Metadata is a map[string]string (see
// packages/provider/types.go), so the seed is carried as its base-10
// string representation.
const SeedMetadataKey = "agentframework.seed"

// DeterministicMetadataKey is the provider.ChatRequest.Metadata key a
// Runner sets to "true" when a Config requests deterministic mode,
// independent of whether a specific Seed value was also supplied. Some
// providers support a deterministic/reproducible mode without exposing a
// raw integer seed (e.g. temperature pinned to 0 plus a provider-side
// cache key); this flag lets an adapter make that choice even when Seed
// is zero.
const DeterministicMetadataKey = "agentframework.deterministic"

// Seed configures deterministic-mode behavior for a Runner's model calls.
//
// Not all providers honor a seed or a deterministic-mode flag — this is a
// best-effort reproducibility aid for tests and eval harnesses, not a
// guarantee. A provider adapter that ignores unknown Metadata keys (the
// documented contract for ChatRequest.Metadata) will simply produce
// non-deterministic output despite Seed being set; callers that need a
// hard guarantee should pair this with a fixed/mock provider (e.g.
// provider.NoOpProvider) in tests.
type Seed struct {
	// Value is the deterministic seed to request, propagated as
	// SeedMetadataKey. Zero is a valid seed value in its own right, so
	// Enabled (not Value != 0) governs whether it is actually attached.
	Value int64

	// Enabled turns on deterministic mode. When true, DeterministicMetadataKey
	// is always set to "true"; SeedMetadataKey is set additionally when
	// Value has been explicitly provided (see WithValue).
	Enabled bool

	// valueSet distinguishes "Value: 0 was explicitly requested" from
	// "no seed value given, deterministic-mode-only". Set via WithValue.
	valueSet bool
}

// NewSeed returns a Seed with Enabled true and Value set to value.
func NewSeed(value int64) Seed {
	return Seed{Value: value, Enabled: true, valueSet: true}
}

// DeterministicOnly returns a Seed with Enabled true but no explicit seed
// value attached — only DeterministicMetadataKey is set on requests.
func DeterministicOnly() Seed {
	return Seed{Enabled: true}
}

// applyTo sets this Seed's metadata keys on md, creating md if nil, and
// returns the (possibly newly allocated) map.
func (s Seed) applyTo(md map[string]string) map[string]string {
	if !s.Enabled {
		return md
	}
	if md == nil {
		md = make(map[string]string, 2)
	}
	md[DeterministicMetadataKey] = "true"
	if s.valueSet {
		md[SeedMetadataKey] = strconv.FormatInt(s.Value, 10)
	}
	return md
}
