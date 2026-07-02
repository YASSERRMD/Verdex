package guardrail

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// OutputLabel identifies the guardrail label a reasoning output carries.
// The only value this package ever considers valid is DraftAnalysisLabel;
// OutputLabel is still an exported, distinct type (rather than a bare
// string) so a caller's own output type can declare a
// `Label() guardrail.OutputLabel` method without stringly-typed drift.
type OutputLabel string

// DraftAnalysisLabel is Verdex's mandatory non-binding-guardrail label,
// re-exporting irac.DraftAnalysisLabel so every package that depends on
// guardrail (rather than irac directly) has one canonical source for the
// label value. The two constants are — and must remain — equal; see
// TestDraftAnalysisLabelMatchesIrac.
const DraftAnalysisLabel OutputLabel = OutputLabel(irac.DraftAnalysisLabel)

// Labeled is satisfied by any reasoning-output type that can report its
// own guardrail label. irac.ConclusionNode satisfies an equivalent shape
// today via IsDraftAnalysis; a type that instead exposes a raw label
// string satisfies Labeled directly and can be passed to ValidateLabeled.
type Labeled interface {
	// Label returns the guardrail label this value carries.
	Label() string
}

// RequireLabel is the hard check that label is exactly the mandatory
// DraftAnalysisLabel. It returns ErrMissingLabel — never a bare bool — so
// a caller cannot compile a call site that silently ignores a failed
// check; the only way to proceed past a failure is to handle the
// returned error.
func RequireLabel(label string) error {
	if label != string(DraftAnalysisLabel) {
		return fmt.Errorf("%w: got %q, want %q", ErrMissingLabel, label, DraftAnalysisLabel)
	}
	return nil
}

// ValidateLabeled is RequireLabel applied to any Labeled value. It is the
// preferred entry point for a reasoning-output type that already exposes
// a Label() method, so callers need not extract the raw string
// themselves.
func ValidateLabeled(x Labeled) error {
	if x == nil {
		return fmt.Errorf("%w: labeled value is nil", ErrMissingLabel)
	}
	return RequireLabel(x.Label())
}

// iracConclusionLabel adapts irac.ConclusionNode to Labeled without this
// package importing irac.ConclusionNode by name in its exported surface —
// ConclusionNode.Label is already a plain string field per
// packages/irac/node.go, so this helper simply forwards it. Exposed as a
// named type (rather than requiring callers to write their own adapter)
// because irac.ConclusionNode is the one existing reasoning-output type
// in the codebase today that already carries a label field.
type iracConclusionLabel struct {
	node irac.ConclusionNode
}

// Label implements Labeled by returning the wrapped ConclusionNode's
// Label field.
func (w iracConclusionLabel) Label() string {
	return w.node.Label
}

// WrapConclusionNode adapts an irac.ConclusionNode to Labeled so it can
// be passed to ValidateLabeled. Every irac.ConclusionNode constructed via
// irac.NewConclusionNode already satisfies RequireLabel by construction
// (see packages/irac/guardrail.go); this wrapper exists so that guarantee
// can also be re-verified defensively at this package's boundary — e.g.
// after a ConclusionNode has round-tripped through untrusted
// serialization — without every caller writing its own one-line adapter.
func WrapConclusionNode(node irac.ConclusionNode) Labeled {
	return iracConclusionLabel{node: node}
}
