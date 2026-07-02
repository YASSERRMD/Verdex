package traversal

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// DistinguishingFactResolver resolves the "precedent -> distinguishing
// facts" hop (HopKindDistinguishingFacts) for a single precedent-origin
// RuleNode within the scope of one case. Implementations decide which of
// the case's FactNodes diverge from the precedent's typical fact pattern
// in a legally significant way — the classic common-law "distinguishing"
// move.
//
// packages/application already models this exact concept as
// DistinguishingFact (Fact + OriginatedRule + Rationale, see
// packages/application/distinguish.go), but this package deliberately
// does not import packages/application: doing so would pull a hard
// dependency into a package meant to stay a thin, general-purpose
// traversal layer over packages/graph. A caller that already has
// application.DistinguishingFact values (or the logic to derive them) is
// expected to supply a DistinguishingFactResolver that wraps that logic
// and returns bare irac.FactNode values, the same "local abstraction
// instead of a hard import" pattern packages/application itself uses for
// Origin (see origin.go).
type DistinguishingFactResolver func(ctx context.Context, caseID string, precedent irac.RuleNode) ([]irac.FactNode, error)

// NoDistinguishingFacts is a DistinguishingFactResolver that always
// resolves to no distinguishing facts. Useful as a default when a
// caller's Query never uses ViaDistinguishingFacts, or in tests
// exercising the other hop kinds in isolation.
func NoDistinguishingFacts(_ context.Context, _ string, _ irac.RuleNode) ([]irac.FactNode, error) {
	return nil, nil
}
