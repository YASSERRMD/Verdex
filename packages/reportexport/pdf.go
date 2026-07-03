package reportexport

import (
	"bytes"
	"fmt"

	"github.com/jung-kurt/gofpdf"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// RenderPDF renders r as a real, valid PDF document: a title page
// section, one section per issue (analysis, favored party, weakest
// link, supporting facts, citations), a skipped-issues section, a
// reasoning-trace appendix (if present), and the mandatory non-binding
// disclaimer — using github.com/jung-kurt/gofpdf, a small, well-known,
// pure-Go PDF generator (no cgo, no external renderer process).
//
// The returned bytes always begin with the "%PDF-" magic header gofpdf
// itself writes as part of a standard PDF file's structure. Page
// content-stream compression is deliberately disabled
// (SetCompression(false)) so every page's operator stream keeps its
// text-showing operators as literal, greppable string content instead
// of zlib-compressed bytes — this is what lets tests (and any other
// downstream consumer) verify a PDF actually contains expected text
// without a full PDF-parsing dependency.
func RenderPDF(r *Report) ([]byte, error) {
	if r == nil {
		return nil, ErrNilCase
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetTitle(fmt.Sprintf("Draft Case Report - %s", reportTitle(r)), true)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	pdf.SetFont("Helvetica", "B", 16)
	pdf.MultiCell(0, 10, fmt.Sprintf("Draft Case Report - %s", reportTitle(r)), "", "L", false)

	pdf.SetFont("Helvetica", "", 10)
	if r.CaseReference != "" {
		pdf.MultiCell(0, 6, fmt.Sprintf("Reference: %s", r.CaseReference), "", "L", false)
	}
	pdf.MultiCell(0, 6, fmt.Sprintf("Generated %s", r.AssembledAt.Format("2006-01-02T15:04:05Z07:00")), "", "L", false)
	pdf.Ln(4)

	pdfSectionHeading(pdf, "Issues and Analysis")
	if len(r.Issues) == 0 {
		pdfBody(pdf, "No issues addressed.")
	}
	for _, issue := range r.Issues {
		pdfWriteIssue(pdf, issue)
	}

	if len(r.SkippedIssueNodeIDs) > 0 {
		pdfSectionHeading(pdf, "Skipped Issues")
		pdfBody(pdf, "The following issues had no grounded conclusion and were omitted from analysis:")
		for _, id := range r.SkippedIssueNodeIDs {
			pdfBody(pdf, "- "+id)
		}
	}

	if r.TraceAppendix != "" {
		pdfSectionHeading(pdf, "Appendix: Reasoning Trace")
		pdfBody(pdf, r.TraceAppendix)
	}

	pdfSectionHeading(pdf, "Disclaimer")
	pdfBody(pdf, guardrail.RequireDisclaimer(""))

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, wrapf("RenderPDF", err)
	}
	return buf.Bytes(), nil
}

func pdfSectionHeading(pdf *gofpdf.Fpdf, text string) {
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.MultiCell(0, 8, text, "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
}

func pdfBody(pdf *gofpdf.Fpdf, text string) {
	pdf.MultiCell(0, 6, text, "", "L", false)
}

func pdfWriteIssue(pdf *gofpdf.Fpdf, issue ReportIssue) {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.MultiCell(0, 7, fmt.Sprintf("Issue %s", issue.IssueNodeID), "", "L", false)

	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(0, 6, "Draft analysis (non-binding): "+issue.Analysis, "", "L", false)

	if issue.FavoredParty != "" {
		pdf.MultiCell(0, 6, fmt.Sprintf("Currently favors: %s (confidence %.0f%%)", issue.FavoredParty, issue.Confidence*100), "", "L", false)
	} else {
		pdf.MultiCell(0, 6, fmt.Sprintf("Currently favors: unresolved on the record (confidence %.0f%%)", issue.Confidence*100), "", "L", false)
	}

	if issue.WeakestLink != "" {
		pdf.MultiCell(0, 6, "Weakest link: "+issue.WeakestLink, "", "L", false)
	}

	for _, id := range issue.SupportingFactIDs {
		pdf.MultiCell(0, 6, "Fact: "+id, "", "L", false)
	}

	for _, c := range issue.Citations {
		pdf.MultiCell(0, 6, fmt.Sprintf("Citation: %s (resolved=%t verified=%t)", c.Text, c.Resolved, c.Verified), "", "L", false)
	}
	pdf.Ln(2)
}
