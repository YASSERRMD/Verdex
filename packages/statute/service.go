package statute

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

// StatuteIngestionService orchestrates the full statute-ingestion
// pipeline:
//
//	load -> parse hierarchy -> build rule nodes with citations -> tag
//	  -> track amendments -> resolve cross-refs -> embed -> persist
//	  -> return []irac.RuleNode
//
// This mirrors packages/fact's FactConstructionService orchestration
// pattern: a single entry point wires together this package's otherwise
// independent, individually testable building blocks (Loader,
// ParseHierarchy, BuildRuleNodes, TagRules, ApplyAmendments,
// DetectAndResolveAll, EmbedRules, PersistRules).
type StatuteIngestionService struct {
	// Loader reads the raw statute corpus. If nil, NewDefaultLoader() is
	// used.
	Loader Loader

	// Embedding computes embeddings for rule text via EmbedChunked. If
	// nil, rules are still built and persisted but Embeddings is left
	// empty on every result (embedding is skipped, not an error) — a
	// service used purely for hierarchy/citation/persistence testing
	// need not supply a live embedding backend.
	Embedding embedding.EmbeddingService

	// Store persists the constructed irac.RuleNodes. If nil, a fresh
	// graph.InMemoryGraphStore is used.
	Store graph.GraphStore
}

// NewStatuteIngestionService constructs a StatuteIngestionService with a
// DefaultLoader and a fresh in-memory graph.GraphStore. No
// embedding.EmbeddingService is set — callers that want embeddings must
// set Embedding explicitly, since this package never reimplements
// embedding or defaults to a live provider.
func NewStatuteIngestionService() *StatuteIngestionService {
	return &StatuteIngestionService{
		Loader: NewDefaultLoader(),
		Store:  graph.NewInMemoryGraphStore(),
	}
}

// IngestRequest carries the input to StatuteIngestionService.Ingest.
type IngestRequest struct {
	// Source supplies the raw statute corpus bytes to s.Loader.Load.
	// Required.
	Source io.Reader

	// JurisdictionCode identifies the jurisdiction every produced
	// irac.RuleNode is tagged and persisted under. Required.
	JurisdictionCode string

	// LegalFamily tags every produced irac.RuleNode's LegalFamily
	// field.
	LegalFamily jurisdiction.LegalFamily

	// CategoryCode tags every produced rule's category (see tagging.go).
	CategoryCode CategoryCode

	// Granularity selects which StatuteNode tier becomes one
	// irac.RuleNode (see RuleBuildOptions.Granularity). Defaults to
	// GranularityClause.
	Granularity RuleGranularity

	// EffectiveDates optionally supplies a per-act effective date,
	// keyed by RawStatute.ActNumber, applied to every rule built from
	// that act's hierarchy (see amendment.go). May be nil/empty.
	EffectiveDates map[string]time.Time

	// Amendments optionally supplies pre-existing amendment history,
	// keyed by irac.RuleNode.ID as produced by this Ingest call's own
	// ID scheme (see RuleBuildOptions.IDPrefix — combine with a known
	// act/index convention to predict IDs, or post-process the returned
	// []irac.RuleNode and call ApplyAmendments/PersistRules directly for
	// finer control). May be nil/empty.
	Amendments map[string]AmendmentRecord

	// ChunkConfig controls how each rule's text is split before
	// embedding (see EmbedOptions.ChunkConfig).
	ChunkConfig embedding.ChunkConfig

	// IDPrefix prefixes every generated irac.RuleNode ID. If empty,
	// "rule" is used.
	IDPrefix string

	// CreatedAt stamps every produced rule's CreatedAt and provenance
	// timestamps. If zero, time.Now() is used.
	CreatedAt time.Time
}

// IngestResult bundles the full per-rule detail the pipeline produced —
// citation, tags, amendment record, embeddings — alongside the plain
// []irac.RuleNode Ingest returns, for callers that need the full
// picture (mirroring packages/fact's FactDetail/
// ConstructFactsDetailed split).
type IngestResult struct {
	Rules            []EmbeddedRule
	CrossReferences  []CrossReference
	UnresolvedXRefs  []CrossReference
	PersistedRuleIDs []string
}

// Ingest runs the full pipeline over req and returns the resulting
// irac.RuleNodes, persisted via s.Store. It is a thin convenience
// wrapper over IngestDetailed for callers that only need the persisted
// nodes.
//
// Returns ErrEmptyInput if req.Source is nil or req.JurisdictionCode is
// blank, ErrMalformedCorpus if the corpus cannot be parsed, or
// ErrPersistFailed (wrapping the underlying store error) if persistence
// fails partway through.
func (s *StatuteIngestionService) Ingest(ctx context.Context, req IngestRequest) ([]irac.RuleNode, error) {
	result, err := s.IngestDetailed(ctx, req)
	nodes := make([]irac.RuleNode, len(result.Rules))
	for i, r := range result.Rules {
		nodes[i] = r.Node
	}
	if err != nil {
		return nodes, err
	}
	return nodes, nil
}

// IngestDetailed runs the full statute-ingestion pipeline over req:
//
//	load -> parse hierarchy -> build rule nodes with citations -> tag
//	  -> track amendments -> resolve cross-refs -> embed -> persist
//
// returning an IngestResult with one EmbeddedRule per successfully
// built rule (across every act in the corpus) plus the corpus-wide
// cross-reference detection results.
func (s *StatuteIngestionService) IngestDetailed(ctx context.Context, req IngestRequest) (IngestResult, error) {
	if req.Source == nil || strings.TrimSpace(req.JurisdictionCode) == "" {
		return IngestResult{}, ErrEmptyInput
	}

	loader := s.Loader
	if loader == nil {
		loader = NewDefaultLoader()
	}
	store := s.Store
	if store == nil {
		store = graph.NewInMemoryGraphStore()
	}
	idPrefix := req.IDPrefix
	if idPrefix == "" {
		idPrefix = "rule"
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	caseID := fmt.Sprintf("statute:%s", req.JurisdictionCode)

	// 1. load
	raws, err := loader.Load(ctx, req.Source)
	if err != nil {
		return IngestResult{}, err
	}

	// 2. parse hierarchy, 3. build rule nodes with citations
	var allBuilt []BuiltRule
	for actIdx, raw := range raws {
		act, err := ParseHierarchy(raw)
		if err != nil {
			continue
		}
		built, err := BuildRuleNodes(act, RuleBuildOptions{
			Granularity:      req.Granularity,
			CaseID:           caseID,
			JurisdictionCode: req.JurisdictionCode,
			LegalFamily:      string(req.LegalFamily),
			IDPrefix:         fmt.Sprintf("%s-%d", idPrefix, actIdx),
			CreatedAt:        createdAt,
		})
		if err != nil {
			continue
		}
		allBuilt = append(allBuilt, built...)
	}
	if len(allBuilt) == 0 {
		return IngestResult{}, ErrMalformedCorpus
	}

	// 4. tag
	tagged := TagRules(allBuilt, TagOptions{
		CategoryCode:     req.CategoryCode,
		JurisdictionCode: req.JurisdictionCode,
		LegalFamily:      req.LegalFamily,
	})

	// 5. track amendments
	amended := ApplyAmendments(tagged, req.Amendments)
	if len(req.EffectiveDates) > 0 {
		for i, a := range amended {
			date, ok := req.EffectiveDates[a.Citation.Act]
			if !ok {
				continue
			}
			amended[i].Amendment = a.Amendment.WithEffectiveDate(date)
		}
	}

	// 6. resolve cross-refs
	xrefs := DetectAndResolveAll(allBuilt)
	unresolved := UnresolvedCrossReferences(xrefs)

	// 7. embed
	embedded, embedErr := EmbedRules(ctx, s.Embedding, amended, EmbedOptions{ChunkConfig: req.ChunkConfig})

	// 8. persist
	persistedNodes, persistErr := PersistRules(ctx, store, req.JurisdictionCode, embedded)
	persistedIDs := make([]string, len(persistedNodes))
	for i, n := range persistedNodes {
		persistedIDs[i] = n.ID
	}

	result := IngestResult{
		Rules:            embedded,
		CrossReferences:  xrefs,
		UnresolvedXRefs:  unresolved,
		PersistedRuleIDs: persistedIDs,
	}

	if embedErr != nil {
		return result, embedErr
	}
	if persistErr != nil {
		return result, persistErr
	}
	return result, nil
}
