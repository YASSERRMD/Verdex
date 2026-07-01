package prompts

import (
	"bytes"
	"fmt"
	"text/template"
)

const nonBindingDisclaimer = `

---
DISCLAIMER: The above analysis is AI-generated legal reasoning produced by
the Verdex platform. It is provided for informational purposes only and does
NOT constitute legal advice. It should not be relied upon as a substitute for
consultation with a qualified legal professional. The analysis may contain
errors, omissions, or inaccuracies and must not be presented as a legal
opinion in any proceeding.`

// Render executes a PromptTemplate with the supplied variable map and returns
// the rendered string.
//
// Variable processing order:
//  1. For each VariableSpec, if Sanitize is true, SanitizeValue is called.
//  2. If Required is true and the value is absent (or empty after sanitation),
//     ErrMissingVariable is returned.
//  3. If MaxLen > 0 and the value length exceeds MaxLen, ErrVariableTooLong
//     is returned.
//  4. The sanitised values are passed to text/template Execute.
//  5. If NonBindingLabel is true, the nonBindingDisclaimer is appended.
func Render(t *PromptTemplate, vars map[string]string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("%w: nil template", ErrInvalidTemplate)
	}

	// Build a processed variable map.
	data := make(map[string]string, len(vars))
	for k, v := range vars {
		data[k] = v
	}

	for _, spec := range t.Variables {
		val, present := data[spec.Name]

		if spec.Sanitize || spec.MaxLen > 0 {
			maxLen := spec.MaxLen
			if !spec.Sanitize {
				// Still run length check even when Sanitize is false.
				if maxLen > 0 && len([]rune(val)) > maxLen {
					return "", fmt.Errorf("%w: variable %q length %d exceeds limit %d",
						ErrVariableTooLong, spec.Name, len([]rune(val)), maxLen)
				}
			} else {
				cleaned, err := SanitizeValue(val, maxLen)
				if err != nil {
					// Distinguish injection vs length errors; wrap with variable name.
					return "", fmt.Errorf("variable %q: %w", spec.Name, err)
				}
				data[spec.Name] = cleaned
				val = cleaned
			}
		}

		if spec.Required && (!present || val == "") {
			return "", fmt.Errorf("%w: %q", ErrMissingVariable, spec.Name)
		}
	}

	tmpl, err := template.New(t.ID).Parse(t.Body)
	if err != nil {
		return "", fmt.Errorf("%w: body parse error: %v", ErrInvalidTemplate, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render execute error: %w", err)
	}

	result := buf.String()

	if t.NonBindingLabel {
		result += nonBindingDisclaimer
	}

	return result, nil
}
