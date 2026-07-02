package reasoningtrace

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportJSON renders trace as indented JSON, suitable for archival or
// for a future UI to fetch and render client-side.
func ExportJSON(trace Trace) ([]byte, error) {
	b, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("reasoningtrace: export json: %w", err)
	}
	return b, nil
}

// ExportMarkdown renders trace as a human-readable Markdown document:
// the narrative prose, a per-stage step/tool-call summary, the
// retrieval log, and each conclusion's authority trail.
func ExportMarkdown(trace Trace) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "# Reasoning trace for case %s\n\n", trace.CaseID)
	fmt.Fprintf(&b, "Generated at %s.\n\n", trace.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))

	b.WriteString("## Narrative\n\n")
	if trace.Narrative == "" {
		b.WriteString("_No narrative available._\n\n")
	} else {
		b.WriteString(trace.Narrative)
		b.WriteString("\n\n")
	}

	b.WriteString("## Steps\n\n")
	if len(trace.Steps) == 0 {
		b.WriteString("_No steps recorded._\n\n")
	} else {
		for _, step := range trace.Steps {
			fmt.Fprintf(&b, "- [%s] step %d: model_called=%t concluded=%t tool_calls=%d duration=%s",
				step.Stage, step.Index, step.ModelCalled, step.Concluded, step.ToolCallCount, step.Duration)
			if step.Err != nil {
				fmt.Fprintf(&b, " err=%q", step.Err.Error())
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## Retrieved nodes and citations\n\n")
	if len(trace.Retrievals) == 0 {
		b.WriteString("_No retrieval events recorded._\n\n")
	} else {
		for _, r := range trace.Retrievals {
			fmt.Fprintf(&b, "- [%s] %s(%v) -> %s\n", r.Stage, r.ToolName, r.Args, r.ResultSummary)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Authority trails\n\n")
	if len(trace.AuthorityTrails) == 0 {
		b.WriteString("_No authority trails available._\n\n")
	} else {
		for _, trail := range trace.AuthorityTrails {
			fmt.Fprintf(&b, "- Issue `%s`\n", trail.IssueNodeID)
			for _, factID := range trail.SupportingFactIDs {
				fmt.Fprintf(&b, "  - Fact `%s`\n", factID)
			}
			for _, c := range trail.Citations {
				fmt.Fprintf(&b, "  - Rule `%s`: %s (resolved=%t verified=%t)\n", c.RuleID, c.Citation, c.Resolved, c.Verified)
			}
		}
	}

	return b.String(), nil
}
