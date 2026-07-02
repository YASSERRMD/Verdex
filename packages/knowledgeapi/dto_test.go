package knowledgeapi_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// TestGetTreeResponse_JSONRoundTrip proves GetTreeResponse (and its
// nested NodeDTO/EdgeDTO/PageMeta) survives a JSON marshal/unmarshal
// round trip unchanged, so this package's wire contract is safe to
// serve over HTTP or store verbatim.
func TestGetTreeResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := knowledgeapi.GetTreeResponse{
		Version: knowledgeapi.APIVersionV1,
		CaseID:  "case-a",
		Nodes: []knowledgeapi.NodeDTO{
			{
				Version:    knowledgeapi.APIVersionV1,
				ID:         "issue-1",
				Type:       "issue",
				CaseID:     "case-a",
				Text:       "Was notice given?",
				Confidence: 0.87,
				CreatedAt:  time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			},
		},
		Edges: []knowledgeapi.EdgeDTO{
			{Version: knowledgeapi.APIVersionV1, Type: "governs", FromID: "rule-1", ToID: "issue-1"},
		},
		Meta: knowledgeapi.PageMeta{Page: 1, PerPage: 20, Total: 1, TotalPages: 1},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded knowledgeapi.GetTreeResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

// TestRetrieveResponse_JSONRoundTrip proves RetrieveResponse round-trips
// through JSON, including its omitempty AnchorNodeID field.
func TestRetrieveResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := knowledgeapi.RetrieveResponse{
		Version: knowledgeapi.APIVersionV1,
		CaseID:  "case-a",
		Items: []knowledgeapi.RetrievedItemDTO{
			{
				NodeID:        "rule-1",
				NodeType:      "rule",
				Text:          "Notice must be in writing.",
				Path:          "graph",
				CombinedScore: 0.42,
				AnchorNodeID:  "issue-1",
				Explanation:   "graph expansion from issue-1",
			},
		},
		VectorHitCount:     3,
		ExpansionSeedCount: 1,
		ExpansionSkipped:   false,
		ExpansionTruncated: true,
		Meta:               knowledgeapi.PageMeta{Page: 1, PerPage: 20, Total: 1, TotalPages: 1},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded knowledgeapi.RetrieveResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

// TestCitationDTO_JSONRoundTrip proves CitationDTO round-trips through
// JSON unchanged.
func TestCitationDTO_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := knowledgeapi.ResolveCitationResponse{
		Version: knowledgeapi.APIVersionV1,
		Citation: knowledgeapi.CitationDTO{
			Version:            knowledgeapi.APIVersionV1,
			NodeID:             "rule-1",
			CaseID:             "case-a",
			Citation:           "Act 12, s.5(a)",
			Origin:             "statute",
			Certainty:          "exact",
			VerificationStatus: "verified",
			Verified:           true,
			ConfidenceScore:    0.93,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded knowledgeapi.ResolveCitationResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}

// TestValidationStatusResponse_JSONRoundTrip proves
// ValidationStatusResponse (and its nested FindingDTO, including the
// omitempty NodeID field) round-trips through JSON unchanged.
func TestValidationStatusResponse_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	original := knowledgeapi.ValidationStatusResponse{
		Version:     knowledgeapi.APIVersionV1,
		CaseID:      "case-a",
		CanFinalize: false,
		Summary:     "1 critical, 0 warning, 0 info (1 total)",
		Findings: []knowledgeapi.FindingDTO{
			{Severity: "critical", Code: "orphan_node", Message: "node has no edges", NodeID: "issue-1"},
			{Severity: "info", Code: "generic_note", Message: "no node concerned"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var decoded knowledgeapi.ValidationStatusResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("round trip mismatch:\n  original: %+v\n  decoded:  %+v", original, decoded)
	}
}
