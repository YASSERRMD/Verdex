package reportexport_test

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestRenderDOCX_HasValidZipSignature(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderDOCX(report)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}

	// Every valid .docx (and any zip archive) begins with the "PK"
	// local-file-header signature (0x50 0x4B 0x03 0x04).
	if !bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		t.Fatalf("RenderDOCX output does not start with PK zip signature; got %x", data[:4])
	}
}

func TestRenderDOCX_IsAValidZipWithExpectedParts(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderDOCX(report)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	names := make(map[string]bool)
	for _, f := range zr.File {
		names[f.Name] = true
	}
	for _, want := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels"} {
		if !names[want] {
			t.Errorf("RenderDOCX zip missing expected part %q", want)
		}
	}
}

func TestRenderDOCX_DocumentXMLContainsExpectedContent(t *testing.T) {
	c := newTestCase(uuid.New())
	analysis := "UNIQUEDOCXANALYSIS9911 favors the second party"
	opinion := newTestOpinion(c.ID, analysis)
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderDOCX(report)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}

	documentXML := readZipFile(t, data, "word/document.xml")

	if !bytes.Contains(documentXML, []byte("UNIQUEDOCXANALYSIS9911")) {
		t.Errorf("word/document.xml does not contain the issue's analysis text")
	}
	if !bytes.Contains(documentXML, []byte(c.Title)) {
		t.Errorf("word/document.xml does not contain the case title %q", c.Title)
	}
	if !bytes.Contains(documentXML, []byte("<w:document")) || !bytes.Contains(documentXML, []byte("</w:document>")) {
		t.Errorf("word/document.xml is not well-formed WordprocessingML")
	}
}

func TestRenderDOCX_ContainsDisclaimer(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderDOCX(report)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}

	documentXML := readZipFile(t, data, "word/document.xml")
	if !bytes.Contains(documentXML, []byte("DRAFT ANALYSIS")) || !bytes.Contains(documentXML, []byte("NON-BINDING")) {
		t.Errorf("word/document.xml does not contain the mandatory non-binding disclaimer")
	}
}

func TestRenderDOCX_EscapesSpecialXMLCharacters(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, `Analysis with <tags> & "quotes" & ampersands`)
	report := newAssembledReport(t, c, opinion)

	data, err := reportexport.RenderDOCX(report)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}

	// A well-formed XML document must not contain a raw, unescaped "<"
	// or "&" inside text content; readZipFile's underlying
	// zip.NewReader call already proves the archive itself parses, so
	// this test focuses on the escaping inside document.xml.
	documentXML := readZipFile(t, data, "word/document.xml")
	if !bytes.Contains(documentXML, []byte("&lt;tags&gt;")) {
		t.Errorf("word/document.xml did not escape '<tags>'; got: %s", documentXML)
	}
	if !bytes.Contains(documentXML, []byte("&amp;")) {
		t.Errorf("word/document.xml did not escape '&'; got: %s", documentXML)
	}
}

func TestRenderDOCX_NilReport(t *testing.T) {
	if _, err := reportexport.RenderDOCX(nil); err != reportexport.ErrNilCase {
		t.Errorf("RenderDOCX(nil) err = %v, want ErrNilCase", err)
	}
}

// readZipFile extracts the content of a named entry from a zip
// archive's bytes, failing the test if the archive or entry cannot be
// read.
func readZipFile(t *testing.T, zipBytes []byte, name string) []byte {
	t.Helper()

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer rc.Close()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return content
	}
	t.Fatalf("zip archive missing entry %q", name)
	return nil
}
