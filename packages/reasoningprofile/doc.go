// Package reasoningprofile is the canonical, jurisdiction-parameterized
// reasoning-weight profile for Verdex: one engine, reweighted by legal
// family per deployment, per the implementation plan's Phase 058
// cross-cutting note. It defines a single Weights shape spanning both
// dimensions the reasoning pipeline already models separately —
// testimony-vs-documentary evidence weighting (packages/evidenceweighing)
// and statute-vs-precedent authority weighting (packages/lawapplication)
// — and a canonical profile for each of the four legal families
// packages/jurisdiction recognizes: common_law, civil_law, mixed, and
// islamic_law.
//
// # Composes with, does not duplicate
//
// This package does not reimplement or replace either sibling package's
// weighing/application logic:
//
//   - packages/jurisdiction (legal_family.go) already defines the
//     canonical LegalFamily type and its four values. This package's
//     Family type is a re-declaration with identical string values (not
//     an alias, to keep this package's public surface self-contained the
//     way packages/evidenceweighing.LegalFamily and
//     packages/lawapplication.LegalFamily already do for their own
//     two-family subset), and ResolveFamily converts a
//     jurisdiction.Jurisdiction's LegalFamily field into this package's
//     Family directly.
//
//   - packages/evidenceweighing (jurisdiction.go) already defines
//     JurisdictionProfile with a Testimony/Documentary shape for
//     common_law and civil_law, defaulting everything else — including
//     mixed and islamic_law — to NeutralProfile. This phase extends that
//     switch with MixedProfile and IslamicLawProfile constructors whose
//     Testimony/Documentary values are derived directly from this
//     package's MixedWeights/IslamicLawWeights, so the two packages
//     cannot silently drift apart. packages/evidenceweighing does not
//     import this package (it stays dependency-light); this package's
//     doc/jurisdiction-reasoning.md records the derivation instead.
//
//   - packages/lawapplication (jurisdiction.go) already defines
//     OriginProfile with a Statute/Precedent shape, same two-family
//     coverage and same silent-fallback gap. This phase extends it the
//     same way, with the same non-import relationship.
//
//   - packages/guardrail (audit.go) already established the
//     Event/AlertSink/NoOpAlertSink audit convention this package's
//     override.go mirrors exactly for per-case Family overrides.
//
// # Primary type
//
// Weights is the single struct capturing all four emphasis dimensions:
//
//	type Weights struct {
//	    TestimonyEmphasis   float64
//	    DocumentaryEmphasis float64
//	    StatuteEmphasis     float64
//	    PrecedentEmphasis   float64
//	}
//
// WeightsForFamily(family Family) Weights resolves the canonical profile
// for any of the four recognized families, exhaustively — there is no
// silent fallback to a fifth "neutral" case in this package (see
// errors.go: an unrecognized Family is a validation error, not a silent
// default). ResolveFamily(j jurisdiction.Jurisdiction) Family is a thin
// wrapper reading j.LegalFamily. SetOverride/OverrideFor let a deployment
// override the resolved family for a specific case, with every override
// recorded via the audit mechanism in override.go.
//
// # What this package deliberately does not do
//
// This package provides WEIGHTS only. It does not itself call
// packages/evidenceweighing.ScoreFact/Weigh or
// packages/lawapplication.WeightByOrigin/Apply — it has no notion of a
// FactRef, a RuleRef, or a case's evidence/rule set at all. Threading a
// resolved Family and its Weights through an actual end-to-end case run
// is Phase 059's reasoning-orchestration pipeline, not this package.
package reasoningprofile
