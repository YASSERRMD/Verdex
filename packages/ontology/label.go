package ontology

// defaultLanguageCode is the fallback language used by Concept.Label when
// the requested language code is absent from Concept.Labels.
const defaultLanguageCode = "en"

// Label returns the display label for c in languageCode. Resolution
// order:
//
//  1. c.Labels[languageCode], if present.
//  2. c.Labels[defaultLanguageCode] ("en"), if present and languageCode
//     was not "en" itself.
//  3. c.Name, as the final fallback.
//
// This mirrors packages/irac's non-binding-guardrail style of always
// returning a usable value rather than an error: a missing translation
// should never block rendering a concept's label.
func (c Concept) Label(languageCode string) string {
	if c.Labels != nil {
		if label, ok := c.Labels[languageCode]; ok && label != "" {
			return label
		}
		if label, ok := c.Labels[defaultLanguageCode]; ok && label != "" {
			return label
		}
	}
	return c.Name
}

// SetLabel sets c's label for languageCode, creating c.Labels if needed.
// Returns the updated Concept (Concept is a value type, so callers must
// reassign, e.g. c = c.SetLabel("ar", "...")).
func (c Concept) SetLabel(languageCode, label string) Concept {
	if c.Labels == nil {
		c.Labels = make(map[string]string)
	}
	c.Labels[languageCode] = label
	return c
}
