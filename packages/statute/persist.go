package statute

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// PersistRules persists every rule's irac.RuleNode in rules via
// store.CreateNode, scoped/keyed by jurisdictionCode (rules whose own
// JurisdictionCode differs from jurisdictionCode are still persisted —
// jurisdictionCode selects which corpus this call is ingesting on
// behalf of, mirroring packages/fact's PersistFacts convention of
// persisting every node handed to it rather than silently filtering).
//
// Returns the persisted irac.RuleNodes (in the same order as rules) and
// ErrPersistFailed (wrapping the underlying store error) if any
// CreateNode call fails; nodes already persisted before the failing
// call are not rolled back, mirroring packages/fact/persist.go's
// PersistFacts.
func PersistRules(ctx context.Context, store graph.GraphStore, jurisdictionCode string, rules []EmbeddedRule) ([]irac.RuleNode, error) {
	persisted := make([]irac.RuleNode, 0, len(rules))
	for _, r := range rules {
		if err := store.CreateNode(ctx, r.Node.Node); err != nil {
			return persisted, wrapPersistError(err)
		}
		persisted = append(persisted, r.Node)
	}
	return persisted, nil
}

// LoadRulesForJurisdiction fetches every previously persisted rule node
// listed in ruleIDs from store, returning ErrRuleNotFound (wrapping the
// underlying store error) for the first ID that cannot be found.
//
// GraphStore has no jurisdiction-scoped query, so callers are expected
// to track which rule IDs belong to a jurisdiction themselves (e.g. from
// the ids returned by PersistRules) and pass them in explicitly.
func LoadRulesForJurisdiction(ctx context.Context, store graph.GraphStore, ruleIDs []string) ([]irac.RuleNode, error) {
	out := make([]irac.RuleNode, 0, len(ruleIDs))
	for _, id := range ruleIDs {
		node, err := store.GetNode(ctx, id)
		if err != nil {
			return out, wrapLookupError(err)
		}
		out = append(out, irac.RuleNode{Node: node})
	}
	return out, nil
}

// wrapPersistError wraps err with ErrPersistFailed so callers can test
// errors.Is(err, ErrPersistFailed) regardless of the underlying
// graph.GraphStore implementation's own error value, mirroring
// packages/fact/persist.go's wrapPersistError.
func wrapPersistError(err error) error {
	return &persistError{underlying: err}
}

type persistError struct {
	underlying error
}

func (e *persistError) Error() string {
	return ErrPersistFailed.Error() + ": " + e.underlying.Error()
}

func (e *persistError) Unwrap() []error {
	return []error{ErrPersistFailed, e.underlying}
}

// wrapLookupError wraps err with ErrRuleNotFound so callers can test
// errors.Is(err, ErrRuleNotFound) regardless of the underlying
// graph.GraphStore implementation's own error value.
func wrapLookupError(err error) error {
	return &lookupError{underlying: err}
}

type lookupError struct {
	underlying error
}

func (e *lookupError) Error() string {
	return ErrRuleNotFound.Error() + ": " + e.underlying.Error()
}

func (e *lookupError) Unwrap() []error {
	return []error{ErrRuleNotFound, e.underlying}
}
