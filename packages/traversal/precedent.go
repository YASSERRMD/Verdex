package traversal

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// PrecedentResolver resolves the "rule -> controlling precedent" hop
// (HopKindControllingPrecedent) for a single RuleNode. Implementations
// decide what "controls" a rule means for their domain — e.g. "this
// statute rule was interpreted by these precedent rules" or "this
// precedent rule's holding controls this other precedent rule under
// stare decisis" — this package only defines the shape of the question
// and where it fits in a traversal, not the legal semantics of
// "controlling".
//
// This indirection exists so packages/traversal never imports
// packages/application, packages/statute, or packages/precedent
// directly: those packages carry the actual Origin/OriginatedRule
// vocabulary and authority metadata (see packages/application/origin.go),
// and a caller wiring this package into a real retrieval pipeline is
// expected to supply a PrecedentResolver backed by that vocabulary
// without this package needing to know about it. This mirrors how
// packages/application itself avoids importing packages/statute or
// packages/precedent (see its doc.go) by defining a local Origin enum
// instead.
type PrecedentResolver func(ctx context.Context, rule irac.RuleNode) ([]irac.RuleNode, error)

// NoPrecedents is a PrecedentResolver that always resolves to no
// controlling precedents. Useful as a default when a caller's Query never
// uses ViaControllingPrecedent, or in tests exercising the other hop
// kinds in isolation.
func NoPrecedents(_ context.Context, _ irac.RuleNode) ([]irac.RuleNode, error) {
	return nil, nil
}
