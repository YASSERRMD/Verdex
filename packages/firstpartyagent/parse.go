package firstpartyagent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// modelArgumentResponse mirrors the JSON schema documented in
// templates/argument_construction.go's prompt body.
type modelArgumentResponse struct {
	Arguments []modelArgument `json:"arguments"`
}

type modelArgument struct {
	IssueNodeID       string   `json:"issue_node_id"`
	Claim             string   `json:"claim"`
	SupportingFactIDs []string `json:"supporting_fact_ids"`
	SupportingRuleIDs []string `json:"supporting_rule_ids"`
	Counterarguments  []string `json:"counterarguments"`
	Confidence        float64  `json:"confidence"`
}

// parseModelArgumentResponse extracts the JSON object from content
// (tolerating leading/trailing prose or a ```json code fence, mirroring
// packages/issueagent/parse.go's extractJSONObject exactly) and
// unmarshals it into a modelArgumentResponse.
//
// Returns ErrMalformedModelOutput wrapping the underlying decode error
// when no valid JSON object can be found.
func parseModelArgumentResponse(content string) (modelArgumentResponse, error) {
	jsonBody := extractJSONObject(content)
	if jsonBody == "" {
		return modelArgumentResponse{}, fmt.Errorf("%w: no JSON object found in model response", ErrMalformedModelOutput)
	}

	var resp modelArgumentResponse
	if err := json.Unmarshal([]byte(jsonBody), &resp); err != nil {
		return modelArgumentResponse{}, fmt.Errorf("%w: %v", ErrMalformedModelOutput, err)
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

// byIssueNodeID groups a modelArgumentResponse's entries by
// IssueNodeID, preserving the model's own per-issue ordering.
func (r modelArgumentResponse) byIssueNodeID() map[string][]modelArgument {
	out := make(map[string][]modelArgument, len(r.Arguments))
	for _, a := range r.Arguments {
		if a.IssueNodeID == "" {
			continue
		}
		out[a.IssueNodeID] = append(out[a.IssueNodeID], a)
	}
	return out
}
