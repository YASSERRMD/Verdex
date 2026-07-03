package reportexport

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// RenderMarkdown renders r as a standalone Markdown document: title,
// facts/issues/analysis/citations per issue, the mandatory non-binding
// disclaimer, and (if present) the reasoning-trace appendix.
//
// The disclaimer is appended via guardrail.RequireDisclaimer — the
// same function every other human-facing output surface in this
// platform uses — so the exact wording stays centralized in
// packages/guardrail rather than duplicated here.
func RenderMarkdown(r *Report) (string, error) {
	if r == nil {
		return "", ErrNilCase
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Draft Case Report — %s\n\n", reportTitle(r))
	if r.CaseReference != "" {
		fmt.Fprintf(&b, "Reference: %s\n\n", r.CaseReference)
	}
	fmt.Fprintf(&b, "Generated %s.\n\n", r.AssembledAt.Format("2006-01-02T15:04:05Z07:00"))

	b.WriteString("## Issues and Analysis\n\n")
	if len(r.Issues) == 0 {
		b.WriteString("_No issues addressed._\n\n")
	} else {
		for _, issue := range r.Issues {
			writeIssueMarkdown(&b, issue)
		}
	}

	if len(r.SkippedIssueNodeIDs) > 0 {
		b.WriteString("## Skipped Issues\n\n")
		b.WriteString("The following issues had no grounded conclusion and were omitted from analysis:\n\n")
		for _, id := range r.SkippedIssueNodeIDs {
			fmt.Fprintf(&b, "- `%s`\n", id)
		}
		b.WriteString("\n")
	}

	if r.TraceAppendix != "" {
		b.WriteString("## Appendix: Reasoning Trace\n\n")
		b.WriteString(r.TraceAppendix)
		b.WriteString("\n")
	}

	return guardrail.RequireDisclaimer(b.String()), nil
}

func writeIssueMarkdown(b *strings.Builder, issue ReportIssue) {
	fmt.Fprintf(b, "### Issue `%s`\n\n", issue.IssueNodeID)
	fmt.Fprintf(b, "**Draft analysis (non-binding):** %s\n\n", issue.Analysis)
	if issue.FavoredParty != "" {
		fmt.Fprintf(b, "**Currently favors:** %s (confidence %.0f%%)\n\n", issue.FavoredParty, issue.Confidence*100)
	} else {
		fmt.Fprintf(b, "**Currently favors:** unresolved on the record (confidence %.0f%%)\n\n", issue.Confidence*100)
	}
	if issue.WeakestLink != "" {
		fmt.Fprintf(b, "**Weakest link:** %s\n\n", issue.WeakestLink)
	}

	if len(issue.SupportingFactIDs) > 0 {
		b.WriteString("**Facts relied upon:**\n\n")
		for _, id := range issue.SupportingFactIDs {
			fmt.Fprintf(b, "- `%s`\n", id)
		}
		b.WriteString("\n")
	}

	if len(issue.Citations) > 0 {
		b.WriteString("**Citations:**\n\n")
		for _, c := range issue.Citations {
			fmt.Fprintf(b, "- %s (resolved=%t verified=%t)\n", c.Text, c.Resolved, c.Verified)
		}
		b.WriteString("\n")
	}
}

// RenderText renders r as plain text: the same content as
// RenderMarkdown with Markdown emphasis/heading syntax stripped,
// mirroring apps/web's opinionToText convention.
func RenderText(r *Report) (string, error) {
	md, err := RenderMarkdown(r)
	if err != nil {
		return "", err
	}
	text := md
	text = stripMarkdownHeadings(text)
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "`", "")
	return text, nil
}

func stripMarkdownHeadings(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		trimmed := strings.TrimLeft(line, "#")
		if trimmed != line {
			lines[i] = strings.TrimPrefix(trimmed, " ")
		}
	}
	return strings.Join(lines, "\n")
}

func reportTitle(r *Report) string {
	if r.CaseTitle != "" {
		return r.CaseTitle
	}
	return r.CaseID.String()
}
