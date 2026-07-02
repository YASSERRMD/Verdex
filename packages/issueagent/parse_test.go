package issueagent

import (
	"errors"
	"testing"
)

func TestParseModelFramingResponse_PlainJSON(t *testing.T) {
	content := `{"framed_issues": [{"source_issue_node_id": "issue-1", "materiality_score": 0.8, "governing_questions": ["Q1"], "confidence": 0.7}]}`
	resp, err := parseModelFramingResponse(content)
	if err != nil {
		t.Fatalf("parseModelFramingResponse: %v", err)
	}
	if len(resp.FramedIssues) != 1 {
		t.Fatalf("len(FramedIssues) = %d, want 1", len(resp.FramedIssues))
	}
	if resp.FramedIssues[0].SourceIssueNodeID != "issue-1" {
		t.Fatalf("SourceIssueNodeID = %q, want issue-1", resp.FramedIssues[0].SourceIssueNodeID)
	}
}

func TestParseModelFramingResponse_MarkdownFenced(t *testing.T) {
	content := "Sure, here you go:\n```json\n{\"framed_issues\": []}\n```\nLet me know if you need more."
	resp, err := parseModelFramingResponse(content)
	if err != nil {
		t.Fatalf("parseModelFramingResponse: %v", err)
	}
	if len(resp.FramedIssues) != 0 {
		t.Fatalf("len(FramedIssues) = %d, want 0", len(resp.FramedIssues))
	}
}

func TestParseModelFramingResponse_NoJSON_ReturnsErr(t *testing.T) {
	_, err := parseModelFramingResponse("I refuse to answer in JSON.")
	if !errors.Is(err, ErrMalformedModelOutput) {
		t.Fatalf("parseModelFramingResponse() error = %v, want ErrMalformedModelOutput", err)
	}
}

func TestParseModelFramingResponse_InvalidJSON_ReturnsErr(t *testing.T) {
	_, err := parseModelFramingResponse(`{"framed_issues": [this is not valid]}`)
	if !errors.Is(err, ErrMalformedModelOutput) {
		t.Fatalf("parseModelFramingResponse() error = %v, want ErrMalformedModelOutput", err)
	}
}

func TestModelFramingResponse_ByIssueNodeID_SkipsEmptyIDs(t *testing.T) {
	resp := modelFramingResponse{
		FramedIssues: []modelFramedIssue{
			{SourceIssueNodeID: "a"},
			{SourceIssueNodeID: ""},
		},
	}
	got := resp.byIssueNodeID()
	if len(got) != 1 {
		t.Fatalf("byIssueNodeID() len = %d, want 1", len(got))
	}
	if _, ok := got["a"]; !ok {
		t.Fatal("byIssueNodeID() missing key \"a\"")
	}
}
