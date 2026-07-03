package reportexport_test

import (
	"bytes"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestRenderPDF_HasValidMagicBytes(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "The breach claim is well supported by the delivery logs.")
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderPDF(report)
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}

	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("RenderPDF output does not start with %%PDF- magic bytes; got first bytes: %q", data[:min(20, len(data))])
	}
	if !bytes.Contains(data, []byte("%%EOF")) {
		t.Errorf("RenderPDF output missing %%%%EOF trailer")
	}
}

func TestRenderPDF_ContainsExpectedText(t *testing.T) {
	c := newTestCase(uuid.New())
	analysis := "UNIQUEBREACHANALYSIS7788 favors the first party"
	opinion := newTestOpinion(c.ID, analysis)
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderPDF(report)
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}

	// Compression is disabled specifically so this assertion can grep
	// the raw operator stream for literal text content, rather than
	// requiring a full PDF-parsing library.
	if !bytes.Contains(data, []byte("UNIQUEBREACHANALYSIS7788")) {
		t.Errorf("RenderPDF output does not contain the issue's analysis text")
	}
	if !bytes.Contains(data, []byte(c.Title)) {
		t.Errorf("RenderPDF output does not contain the case title %q", c.Title)
	}
}

func TestRenderPDF_ContainsDisclaimer(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderPDF(report)
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}

	if !bytes.Contains(data, []byte("DRAFT ANALYSIS")) || !bytes.Contains(data, []byte("NON-BINDING")) {
		t.Errorf("RenderPDF output does not contain the mandatory non-binding disclaimer")
	}
}

func TestRenderPDF_NilReport(t *testing.T) {
	if _, err := reportexport.RenderPDF(nil); err != reportexport.ErrNilCase {
		t.Errorf("RenderPDF(nil) err = %v, want ErrNilCase", err)
	}
}
