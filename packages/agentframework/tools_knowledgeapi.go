package agentframework

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

// Tool names for the concrete knowledgeapi-backed tools this package
// provides. Downstream agents (Phases 050-056) should reference these
// constants rather than hardcoding the string literal, so a rename here
// is a compile error at every call site instead of a silent runtime
// ErrToolNotFound.
const (
	ToolSearchCaseKnowledge = "search_case_knowledge"
	ToolGetNode             = "get_node"
	ToolLookupPaths         = "lookup_paths"
	ToolResolveCitation     = "resolve_citation"
	ToolValidationStatus    = "validation_status"
)

// NewKnowledgeAPIToolSet builds a ToolSet whose tools wrap every read
// method of api: hybrid retrieval (search_case_knowledge), single-node
// lookup (get_node), materialized-path lookup (lookup_paths), citation
// resolution (resolve_citation), and tree validation status
// (validation_status). This is the sanctioned way an agent built on this
// framework touches case knowledge — none of these tools re-derive
// retrieval, citation, or validation logic; they only translate a model's
// tool-call args into the matching knowledgeapi request DTO and back.
//
// Returns ErrNilKnowledgeAPI if api is nil.
func NewKnowledgeAPIToolSet(api *knowledgeapi.KnowledgeAPI) (*ToolSet, error) {
	if api == nil {
		return nil, ErrNilKnowledgeAPI
	}

	ts := NewToolSet()
	ts.MustRegister(searchCaseKnowledgeTool(api))
	ts.MustRegister(getNodeTool(api))
	ts.MustRegister(lookupPathsTool(api))
	ts.MustRegister(resolveCitationTool(api))
	ts.MustRegister(validationStatusTool(api))
	return ts, nil
}

// argString extracts a required string argument, returning
// ErrMalformedOutput if absent or the wrong type.
func argString(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("%w: missing argument %q", ErrMalformedOutput, key)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%w: argument %q must be a string", ErrMalformedOutput, key)
	}
	return s, nil
}

// argInt extracts an optional integer argument, returning def if absent.
// Accepts int and float64 (the common shape for args decoded from JSON).
func argInt(args map[string]any, key string, def int) int {
	v, ok := args[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return def
	}
}

// argStringSlice extracts an optional []string argument, returning nil if
// absent or malformed. Accepts []string directly or []any of strings (the
// common shape for args decoded from JSON).
func argStringSlice(args map[string]any, key string) []string {
	v, ok := args[key]
	if !ok {
		return nil
	}
	switch s := v.(type) {
	case []string:
		return s
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func searchCaseKnowledgeTool(api *knowledgeapi.KnowledgeAPI) Tool {
	return Tool{
		Name:        ToolSearchCaseKnowledge,
		Description: "Run a fused semantic-plus-structural search over this case's knowledge tree, anchored on a node and/or expanded along named hops (governing_rule, controlling_precedent, distinguishing_facts). Wraps knowledgeapi.KnowledgeAPI.Retrieve.",
		Parameters: []ParamSchema{
			{Name: "anchor_node_id", Type: "string", Description: "Node ID to anchor structural expansion from.", Required: true},
			{Name: "expansion_hops", Type: "array", Description: "Named expansion hops to follow from the anchor."},
			{Name: "top_k", Type: "integer", Description: "Maximum number of items to return."},
		},
		Invoke: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			anchorNodeID, err := argString(args, "anchor_node_id")
			if err != nil {
				return ToolResult{}, err
			}
			resp, err := api.Retrieve(ctx, knowledgeapi.RetrieveRequest{
				CaseID:        api.CaseID(),
				AnchorNodeID:  anchorNodeID,
				ExpansionHops: argStringSlice(args, "expansion_hops"),
				TopK:          argInt(args, "top_k", 0),
			})
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{
				Content: fmt.Sprintf("found %d item(s) for anchor %s", len(resp.Items), anchorNodeID),
				Data:    resp,
			}, nil
		},
	}
}

func getNodeTool(api *knowledgeapi.KnowledgeAPI) Tool {
	return Tool{
		Name:        ToolGetNode,
		Description: "Fetch a single node by ID from this case's knowledge tree. Wraps knowledgeapi.KnowledgeAPI.GetNode.",
		Parameters: []ParamSchema{
			{Name: "node_id", Type: "string", Description: "Node ID to fetch.", Required: true},
		},
		Invoke: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			nodeID, err := argString(args, "node_id")
			if err != nil {
				return ToolResult{}, err
			}
			resp, err := api.GetNode(ctx, knowledgeapi.GetNodeRequest{
				CaseID: api.CaseID(),
				NodeID: nodeID,
			})
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{
				Content: fmt.Sprintf("[%s] %s: %s", resp.Node.Type, resp.Node.ID, resp.Node.Text),
				Data:    resp,
			}, nil
		},
	}
}

func lookupPathsTool(api *knowledgeapi.KnowledgeAPI) Tool {
	return Tool{
		Name:        ToolLookupPaths,
		Description: "Look up materialized reasoning-tree paths from a starting node, following a given edge type. Wraps knowledgeapi.KnowledgeAPI.LookupPaths.",
		Parameters: []ParamSchema{
			{Name: "from_node_id", Type: "string", Description: "Node ID to start from.", Required: true},
			{Name: "edge_type", Type: "string", Description: "Edge type to follow."},
			{Name: "max_depth", Type: "integer", Description: "Maximum path depth (0 means unbounded)."},
		},
		Invoke: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			fromNodeID, err := argString(args, "from_node_id")
			if err != nil {
				return ToolResult{}, err
			}
			edgeType, _ := args["edge_type"].(string)
			resp, err := api.LookupPaths(ctx, knowledgeapi.LookupPathsRequest{
				CaseID:     api.CaseID(),
				FromNodeID: fromNodeID,
				EdgeType:   edgeType,
				MaxDepth:   argInt(args, "max_depth", 0),
			})
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{
				Content: fmt.Sprintf("found %d path(s) from %s", len(resp.Paths), fromNodeID),
				Data:    resp,
			}, nil
		},
	}
}

func resolveCitationTool(api *knowledgeapi.KnowledgeAPI) Tool {
	return Tool{
		Name:        ToolResolveCitation,
		Description: "Resolve and verify the citation for a single node, guarding against hallucinated authority. Wraps knowledgeapi.KnowledgeAPI.ResolveCitation.",
		Parameters: []ParamSchema{
			{Name: "node_id", Type: "string", Description: "Node ID to resolve a citation for.", Required: true},
		},
		Invoke: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			nodeID, err := argString(args, "node_id")
			if err != nil {
				return ToolResult{}, err
			}
			resp, err := api.ResolveCitation(ctx, knowledgeapi.ResolveCitationRequest{
				CaseID: api.CaseID(),
				NodeID: nodeID,
			})
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{
				Content: fmt.Sprintf("%s (verified=%t, confidence=%.2f)", resp.Citation.Citation, resp.Citation.Verified, resp.Citation.ConfidenceScore),
				Data:    resp,
			}, nil
		},
	}
}

func validationStatusTool(api *knowledgeapi.KnowledgeAPI) Tool {
	return Tool{
		Name:        ToolValidationStatus,
		Description: "Report whether this case's assembled tree currently passes integrity validation and can be finalized. Wraps knowledgeapi.KnowledgeAPI.ValidationStatus.",
		Parameters:  nil,
		Invoke: func(ctx context.Context, _ map[string]any) (ToolResult, error) {
			resp, err := api.ValidationStatus(ctx, knowledgeapi.ValidationStatusRequest{
				CaseID: api.CaseID(),
			})
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{
				Content: resp.Summary,
				Data:    resp,
			}, nil
		},
	}
}
