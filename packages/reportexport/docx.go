package reportexport

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// docxContentTypes, docxRootRels, and docxDocumentRels are the fixed,
// minimal set of OOXML package parts every valid .docx file needs
// besides word/document.xml itself: the package's content-type
// manifest, the package-level relationship pointing at the main
// document part, and that document part's own (empty) relationship
// list. None of these vary per Report, so they are declared once as
// constants rather than rebuilt on every RenderDOCX call.
const (
	docxContentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`

	docxRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`

	docxDocumentRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`
)

// RenderDOCX renders r as a real, valid Office Open XML (.docx)
// document: a title, one section per issue (analysis, favored party,
// weakest link, supporting facts, citations), a skipped-issues
// section, a reasoning-trace appendix (if present), and the mandatory
// non-binding disclaimer.
//
// A .docx file is a standard PK zip archive containing a small,
// well-defined set of XML parts (see docx*Content constants above);
// this function builds that archive directly via the standard
// library's archive/zip and encoding/xml, rather than depending on a
// third-party DOCX library, since the OOXML subset a plain-paragraph
// report needs is small and stable. The returned bytes always begin
// with the "PK" zip signature every valid .docx file carries.
func RenderDOCX(r *Report) ([]byte, error) {
	if r == nil {
		return nil, ErrNilCase
	}

	var paragraphs []string
	paragraphs = append(paragraphs, fmt.Sprintf("Draft Case Report - %s", reportTitle(r)))
	if r.CaseReference != "" {
		paragraphs = append(paragraphs, "Reference: "+r.CaseReference)
	}
	paragraphs = append(paragraphs, "Generated "+r.AssembledAt.Format("2006-01-02T15:04:05Z07:00"))

	paragraphs = append(paragraphs, "Issues and Analysis")
	if len(r.Issues) == 0 {
		paragraphs = append(paragraphs, "No issues addressed.")
	}
	for _, issue := range r.Issues {
		paragraphs = append(paragraphs, docxIssueParagraphs(issue)...)
	}

	if len(r.SkippedIssueNodeIDs) > 0 {
		paragraphs = append(paragraphs, "Skipped Issues")
		paragraphs = append(paragraphs, "The following issues had no grounded conclusion and were omitted from analysis:")
		for _, id := range r.SkippedIssueNodeIDs {
			paragraphs = append(paragraphs, "- "+id)
		}
	}

	if r.TraceAppendix != "" {
		paragraphs = append(paragraphs, "Appendix: Reasoning Trace")
		for _, line := range strings.Split(r.TraceAppendix, "\n") {
			paragraphs = append(paragraphs, line)
		}
	}

	paragraphs = append(paragraphs, "Disclaimer")
	for _, line := range strings.Split(guardrail.RequireDisclaimer(""), "\n") {
		paragraphs = append(paragraphs, line)
	}

	documentXML := buildDocxDocumentXML(paragraphs)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if err := writeZipEntry(zw, "[Content_Types].xml", docxContentTypes); err != nil {
		return nil, wrapf("RenderDOCX", err)
	}
	if err := writeZipEntry(zw, "_rels/.rels", docxRootRels); err != nil {
		return nil, wrapf("RenderDOCX", err)
	}
	if err := writeZipEntry(zw, "word/_rels/document.xml.rels", docxDocumentRels); err != nil {
		return nil, wrapf("RenderDOCX", err)
	}
	if err := writeZipEntry(zw, "word/document.xml", documentXML); err != nil {
		return nil, wrapf("RenderDOCX", err)
	}
	if err := zw.Close(); err != nil {
		return nil, wrapf("RenderDOCX", err)
	}
	return buf.Bytes(), nil
}

func docxIssueParagraphs(issue ReportIssue) []string {
	out := []string{fmt.Sprintf("Issue %s", issue.IssueNodeID)}
	out = append(out, "Draft analysis (non-binding): "+issue.Analysis)
	if issue.FavoredParty != "" {
		out = append(out, fmt.Sprintf("Currently favors: %s (confidence %.0f%%)", issue.FavoredParty, issue.Confidence*100))
	} else {
		out = append(out, fmt.Sprintf("Currently favors: unresolved on the record (confidence %.0f%%)", issue.Confidence*100))
	}
	if issue.WeakestLink != "" {
		out = append(out, "Weakest link: "+issue.WeakestLink)
	}
	for _, id := range issue.SupportingFactIDs {
		out = append(out, "Fact: "+id)
	}
	for _, c := range issue.Citations {
		out = append(out, fmt.Sprintf("Citation: %s (resolved=%t verified=%t)", c.Text, c.Resolved, c.Verified))
	}
	return out
}

// buildDocxDocumentXML renders paragraphs as word/document.xml's
// body: one <w:p> per paragraph, each holding a single run whose text
// is XML-escaped via encoding/xml so PII placeholders, citation
// punctuation, and any other report text can never break the XML
// structure. Empty paragraphs are preserved as empty <w:p/> elements
// (blank lines), matching how a Word document represents them.
func buildDocxDocumentXML(paragraphs []string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	for _, p := range paragraphs {
		if p == "" {
			b.WriteString("<w:p/>")
			continue
		}
		b.WriteString("<w:p><w:r><w:t xml:space=\"preserve\">")
		b.WriteString(escapeXMLText(p))
		b.WriteString("</w:t></w:r></w:p>")
	}
	b.WriteString(`</w:body></w:document>`)
	return b.String()
}

// escapeXMLText escapes s for safe inclusion inside a <w:t> element's
// text content using encoding/xml's own escaper, so this package never
// hand-rolls XML entity substitution.
func escapeXMLText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func writeZipEntry(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}
