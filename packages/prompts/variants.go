package prompts

import "fmt"

// VariantSelector resolves the best-matching PromptTemplate for a given
// ID, locale, and legal family using a deterministic fallback chain.
type VariantSelector struct{}

// SelectBest returns the highest-version template that best matches the
// requested locale and legalFamily using the following fallback chain:
//
//  1. Exact match:       locale == requested  AND legalFamily == requested
//  2. Locale only:       locale == requested  AND legalFamily == ""
//  3. Legal-family only: locale == ""         AND legalFamily == requested
//  4. Universal:         locale == ""         AND legalFamily == ""
//
// Within each tier, the highest Version wins.
// If no template is found at any tier, ErrNoTemplate is returned (aliased to
// ErrTemplateNotFound for package-level consistency).
func (vs VariantSelector) SelectBest(registry *Registry, id string, locale string, legalFamily string) (*PromptTemplate, error) {
	// Collect all templates for this ID.
	all := registry.List()

	type tier struct {
		localeMatch bool
		familyMatch bool
	}

	// We evaluate four tiers in priority order.
	tiers := []struct {
		wantLocale string
		wantFamily string
		label      string
	}{
		{locale, legalFamily, "exact"},
		{locale, "", "locale-only"},
		{"", legalFamily, "family-only"},
		{"", "", "universal"},
	}

	for _, tr := range tiers {
		var best *PromptTemplate
		for _, t := range all {
			t := t // capture
			if t.ID != id {
				continue
			}
			if t.Locale != tr.wantLocale || t.LegalFamily != tr.wantFamily {
				continue
			}
			if best == nil || t.Version > best.Version {
				best = &t
			}
		}
		if best != nil {
			return best, nil
		}
	}

	return nil, fmt.Errorf("%w: no variant found for id=%s locale=%q legalFamily=%q",
		ErrTemplateNotFound, id, locale, legalFamily)
}
