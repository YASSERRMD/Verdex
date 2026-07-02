package knowledgeapi

import (
	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/treeindex"
)

// KnowledgeAPI is the single, stable entrypoint composing every layer of
// Verdex's knowledge/retrieval stack behind one internal contract: tree
// reads (via a case-scoped GraphStore and treeindex), hybrid retrieval,
// citation resolution, and validation status. Future consumers — Part 5's
// reasoning agents, and any future case-workspace UI — should depend on
// KnowledgeAPI instead of importing treeindex/traversal/hybridretrieval/
// citation/treevalidation directly.
//
// A KnowledgeAPI value is scoped to exactly one case: construct one with
// NewKnowledgeAPI per case, using a knowledgeisolation.CaseScopedStore and
// knowledgeisolation.CaseScopedVectorStore already scoped to that case's
// CaseID. This mirrors knowledgeisolation's own per-case construction
// pattern and keeps the cross-case isolation guarantee structural (a
// caller cannot forget to pass a case ID to any method, because the case
// is fixed at construction) rather than merely conventional.
//
// Every method additionally requires an authenticated identity.User on
// its context.Context argument holding identity.PermViewCase; see
// access.go and doc/knowledge-api.md for the full access-control model.
type KnowledgeAPI struct {
	caseID string

	store       *knowledgeisolation.CaseScopedStore
	vectorStore *knowledgeisolation.CaseScopedVectorStore
	indexer     *treeindex.Indexer
	retriever   *hybridretrieval.Retriever

	// citationResolver resolves a node to a formatted citation. Defaults
	// to citation.NoResolver when left nil by NewKnowledgeAPI's caller via
	// WithCitationResolver, so ResolveCitation still returns verified
	// span/text data (with an empty citation string) rather than failing.
	citationResolver citation.Resolver

	// validation configures treevalidation.TreeValidationService. Its
	// zero value is valid: an empty CaseJurisdictionCode simply skips the
	// jurisdiction-consistency check, matching
	// TreeValidationService.Validate's own documented behaviour.
	validation treeValidationConfig
}

// treeValidationConfig mirrors the caller-configurable fields of
// treevalidation.TreeValidationService, kept as knowledgeapi's own type so
// this package's public constructor surface does not leak
// treevalidation's struct directly.
type treeValidationConfig struct {
	caseJurisdictionCode         string
	allowedJurisdictionOverrides []string
	confidenceThreshold          float64
}

// NewKnowledgeAPI constructs a KnowledgeAPI scoped to a single case,
// composing a case-scoped graph store, a case-scoped vector store, a
// treeindex.Indexer built over the same store, and a hybridretrieval.
// Retriever built over the same store/vector-store pair. All four must be
// non-nil and share the same underlying case scope; NewKnowledgeAPI does
// not itself verify that the indexer/retriever were constructed against
// the same store passed here (see doc/knowledge-api.md for the composition
// contract callers must uphold).
//
// Returns ErrEmptyCaseID if caseID is empty, or ErrNilService if any
// dependency is nil.
func NewKnowledgeAPI(
	caseID string,
	store *knowledgeisolation.CaseScopedStore,
	vectorStore *knowledgeisolation.CaseScopedVectorStore,
	indexer *treeindex.Indexer,
	retriever *hybridretrieval.Retriever,
) (*KnowledgeAPI, error) {
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	if store == nil || vectorStore == nil || indexer == nil || retriever == nil {
		return nil, ErrNilService
	}

	return &KnowledgeAPI{
		caseID:           caseID,
		store:            store,
		vectorStore:      vectorStore,
		indexer:          indexer,
		retriever:        retriever,
		citationResolver: citation.NoResolver,
	}, nil
}

// WithCitationResolver returns a shallow copy of api configured to use
// resolver for ResolveCitation instead of the default citation.NoResolver.
// A nil resolver is ignored (the receiver is returned unchanged).
func (api *KnowledgeAPI) WithCitationResolver(resolver citation.Resolver) *KnowledgeAPI {
	if resolver == nil {
		return api
	}
	clone := *api
	clone.citationResolver = resolver
	return &clone
}

// WithValidation returns a shallow copy of api configured to run
// treevalidation with the given case jurisdiction code, allowed
// jurisdiction overrides, and confidence threshold. Passing an empty
// jurisdictionCode disables the jurisdiction-consistency check, matching
// treevalidation.TreeValidationService.Validate's own documented
// behaviour. A zero confidenceThreshold uses
// treevalidation.DefaultConfidenceThreshold.
func (api *KnowledgeAPI) WithValidation(jurisdictionCode string, allowedOverrides []string, confidenceThreshold float64) *KnowledgeAPI {
	clone := *api
	clone.validation = treeValidationConfig{
		caseJurisdictionCode:         jurisdictionCode,
		allowedJurisdictionOverrides: allowedOverrides,
		confidenceThreshold:          confidenceThreshold,
	}
	return &clone
}

// CaseID returns the case this KnowledgeAPI instance is scoped to.
func (api *KnowledgeAPI) CaseID() string {
	return api.caseID
}
