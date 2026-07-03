package reportexport_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestRenderMarkdown_ContainsDisclaimerAndContent(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "UNIQUEMDANALYSIS4455 favors the first party")
	report := newAssembledReport(t, c, opinion)

	md, err := reportexport.RenderMarkdown(report)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	if !strings.Contains(md, "UNIQUEMDANALYSIS4455") {
		t.Errorf("RenderMarkdown output missing issue analysis text")
	}
	if !strings.Contains(md, c.Title) {
		t.Errorf("RenderMarkdown output missing case title")
	}
	if !strings.Contains(md, "DRAFT ANALYSIS") || !strings.Contains(md, "NON-BINDING") {
		t.Errorf("RenderMarkdown output missing mandatory non-binding disclaimer")
	}
	if !strings.Contains(md, "issue-2") {
		t.Errorf("RenderMarkdown output missing skipped issue id")
	}
}

func TestRenderMarkdown_NilReport(t *testing.T) {
	if _, err := reportexport.RenderMarkdown(nil); err != reportexport.ErrNilCase {
		t.Errorf("RenderMarkdown(nil) err = %v, want ErrNilCase", err)
	}
}

func TestRenderText_StripsMarkdownSyntaxButKeepsContent(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "UNIQUETEXTANALYSIS6677")
	report := newAssembledReport(t, c, opinion)

	text, err := reportexport.RenderText(report)
	if err != nil {
		t.Fatalf("RenderText: %v", err)
	}

	if strings.Contains(text, "##") || strings.Contains(text, "**") {
		t.Errorf("RenderText output still contains Markdown syntax: %q", text)
	}
	if !strings.Contains(text, "UNIQUETEXTANALYSIS6677") {
		t.Errorf("RenderText output missing issue analysis text")
	}
	if !strings.Contains(text, "NON-BINDING") {
		t.Errorf("RenderText output missing disclaimer")
	}
}

func TestRenderMarkdown_TraceAppendixIncluded(t *testing.T) {
	c := newTestCase(uuid.New())
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)
	report.TraceAppendix = "UNIQUETRACEAPPENDIX8899 narrative content"

	md, err := reportexport.RenderMarkdown(report)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}
	if !strings.Contains(md, "UNIQUETRACEAPPENDIX8899") {
		t.Errorf("RenderMarkdown output missing reasoning trace appendix content")
	}
	if !strings.Contains(md, "Appendix: Reasoning Trace") {
		t.Errorf("RenderMarkdown output missing appendix section heading")
	}
}
