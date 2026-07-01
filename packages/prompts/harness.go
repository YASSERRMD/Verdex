package prompts

import (
	"fmt"
	"strings"
)

// TestHarness provides a lightweight test utility for verifying that prompt
// templates render correctly and contain expected content.
//
// It is intended for use in unit tests and CI pipelines, not production code.
type TestHarness struct{}

// RunCase renders the template with vars and verifies that the output contains
// every string listed in expectedSubstrings. It returns a non-nil error if
// rendering fails or if any expected substring is absent.
func (h TestHarness) RunCase(tmpl *PromptTemplate, vars map[string]string, expectedSubstrings []string) error {
	if tmpl == nil {
		return fmt.Errorf("TestHarness.RunCase: template is nil")
	}

	rendered, err := Render(tmpl, vars)
	if err != nil {
		return fmt.Errorf("TestHarness.RunCase: render error: %w", err)
	}

	var missing []string
	for _, sub := range expectedSubstrings {
		if !strings.Contains(rendered, sub) {
			missing = append(missing, sub)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("TestHarness.RunCase: output missing expected substrings: %v\n\nRendered output:\n%s",
			missing, rendered)
	}

	return nil
}

// FailsOn renders the template with vars and reports whether Render returned
// an error. It returns (true, nil) when rendering fails (the expected outcome
// for negative tests), and (false, nil) when rendering unexpectedly succeeds.
// A non-nil second return value indicates an internal harness error (not a
// render error).
func (h TestHarness) FailsOn(tmpl *PromptTemplate, vars map[string]string) (bool, error) {
	if tmpl == nil {
		return false, fmt.Errorf("TestHarness.FailsOn: template is nil")
	}

	_, err := Render(tmpl, vars)
	return err != nil, nil
}
