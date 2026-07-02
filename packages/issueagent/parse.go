package issueagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// modelFramingResponse mirrors the JSON schema documented in
// templates/issue_framing.go's prompt body.
type modelFramingResponse struct {
	FramedIssues []modelFramedIssue `json:"framed_issues"`
}

type modelFramedIssue struct {
	SourceIssueNodeID  string   `json:"source_issue_node_id"`
	MaterialityScore   float64  `json:"materiality_score"`
	GoverningQuestions []string `json:"governing_questions"`
	Ambiguities        []string `json:"ambiguities"`
	Confidence         float64  `json:"confidence"`
}

// parseModelFramingResponse extracts the JSON object from content
// (tolerating leading/trailing prose or a ```json code fence, which some
// providers wrap structured output in despite being asked for raw JSON)
// and unmarshals it into a modelFramingResponse.
//
// Returns ErrMalformedModelOutput wrapping the underlying decode error
// when no valid JSON object can be found.
func parseModelFramingResponse(content string) (modelFramingResponse, error) {
	jsonBody := extractJSONObject(content)
	if jsonBody == "" {
		return modelFramingResponse{}, fmt.Errorf("%w: no JSON object found in model response", ErrMalformedModelOutput)
	}

	var resp modelFramingResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		return modelFramingResponse{}, fmt.Errorf("%w: %v", ErrMalformedModelOutput, err)
	}
	return resp, nil
}

// extractJSONObject returns the substring of content spanning the first
// '{' through its matching final '}', or "" if content contains no
// balanced brace pair. This tolerates a model wrapping its JSON payload
// in a markdown code fence or explanatory prose around it.
func extractJSONObject(content string) string {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return content[start : end+1]
}

// byIssueNodeID indexes a modelFramingResponse's entries by
// SourceIssueNodeID for lookup while assembling FramedIssues.
func (r modelFramingResponse) byIssueNodeID() map[string]modelFramedIssue {
	out := make(map[string]modelFramedIssue, len(r.FramedIssues))
	for _, fi := range r.FramedIssues {
		if fi.SourceIssueNodeID == "" {
			continue
		}
		out[fi.SourceIssueNodeID] = fi
	}
	return out
}
