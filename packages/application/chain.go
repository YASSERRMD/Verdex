package application

// RuleChain is an ordered sequence of OriginatedRules that must be
// applied together to resolve an issue — e.g. a statute section that
// cross-references another section, or a precedent whose holding only
// makes sense read alongside a companion rule. Order matters: index 0 is
// applied first, and later rules in the chain are understood to build on
// or qualify earlier ones.
type RuleChain struct {
	// Rules is the ordered sequence of rules forming this chain.
	Rules []OriginatedRule
}

// Validate reports whether c is well-formed: non-empty, and free of
// cycles. A cycle here means the same underlying irac.RuleNode.ID
// appears more than once in the chain — a rule cannot (directly or, in
// this flat-sequence representation, indirectly) depend on itself.
// Returns ErrEmptyInput if c.Rules is empty, or ErrCyclicChain if any
// rule ID repeats.
func (c RuleChain) Validate() error {
	if len(c.Rules) == 0 {
		return ErrEmptyInput
	}
	seen := make(map[string]struct{}, len(c.Rules))
	for _, r := range c.Rules {
		if _, ok := seen[r.Rule.ID]; ok {
			return ErrCyclicChain
		}
		seen[r.Rule.ID] = struct{}{}
	}
	return nil
}

// Len returns the number of rules in the chain.
func (c RuleChain) Len() int {
	return len(c.Rules)
}
