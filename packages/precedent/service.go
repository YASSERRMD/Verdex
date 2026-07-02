package precedent

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// PrecedentIngestionService orchestrates the full precedent-ingestion
// pipeline:
//
//	load -> extract holding/ratio -> build rule nodes with citations ->
//	  tag -> weight by court hierarchy -> embed -> score authority ->
//	  persist -> return []PrecedentRule
//
// This mirrors packages/statute's StatuteIngestionService orchestration
// pattern: a single entry point wires together this package's otherwise
// independent, individually testable building blocks (Loader,
// BuildPrecedentRules, TagPrecedents, ApplyCourtHierarchy,
// EmbedPrecedents, ScorePrecedents, PersistPrecedents).
type PrecedentIngestionService struct {
	// Loader reads the raw precedent corpus. If nil, NewDefaultLoader()
	// is used.
	Loader Loader

	// HoldingExtractor extracts holding/ratio from each precedent's
	// FullText. If nil, ExtractHoldingAndRatio is used. This is the
	// pluggable extension point documented in holding.go.
	HoldingExtractor ExtractorFunc

	// Embedding computes embeddings for precedent holding+ratio text via
	// EmbedChunked. If nil, rules are still built, scored, and persisted
	// but Embeddings is left empty on every result (embedding is
	// skipped, not an error) — a service used purely for
	// extraction/citation/persistence testing need not supply a live
	// embedding backend.
	Embedding embedding.EmbeddingService

	// Store persists the constructed irac.RuleNodes. If nil, a fresh
	// graph.InMemoryGraphStore is used.
	Store graph.GraphStore
}

// NewPrecedentIngestionService constructs a PrecedentIngestionService with
// a DefaultLoader and a fresh in-memory graph.GraphStore. No
// embedding.EmbeddingService is set — callers that want embeddings must
// set Embedding explicitly, since this package never reimplements
// embedding or defaults to a live provider.
func NewPrecedentIngestionService() *PrecedentIngestionService {
	return &PrecedentIngestionService{
		Loader: NewDefaultLoader(),
		Store:  graph.NewInMemoryGraphStore(),
	}
}

// IngestRequest carries the input to PrecedentIngestionService.Ingest.
type IngestRequest struct {
	// Source supplies the raw precedent corpus bytes to s.Loader.Load.
	// Required.
	Source io.Reader

	// JurisdictionCode identifies the jurisdiction every produced
	// irac.RuleNode is tagged and persisted under. Required.
	JurisdictionCode string

	// LegalFamily tags every produced irac.RuleNode's LegalFamily field.
	LegalFamily string

	// CategoryCode tags every produced rule's category (see tagging.go).
	CategoryCode CategoryCode

	// CourtLevelOverride, if non-empty, is applied uniformly to every
	// rule instead of classifying each rule's court independently (see
	// ApplyCourtHierarchy).
	CourtLevelOverride CourtLevel

	// ChunkConfig controls how each precedent's holding+ratio text is
	// split before embedding (see EmbedOptions.ChunkConfig).
	ChunkConfig embedding.ChunkConfig

	// IDPrefix prefixes every generated irac.RuleNode ID. If empty,
	// "precedent" is used.
	IDPrefix string

	// CreatedAt stamps every produced rule's CreatedAt and provenance
	// timestamps. If zero, time.Now() is used.
	CreatedAt time.Time

	// ScoreAsOf is the reference time AuthorityScoreAsOf uses to compute
	// recency. If zero, time.Now() is used.
	ScoreAsOf time.Time
}

// IngestResult bundles the full per-rule detail the pipeline produced
// (tags, court level, embeddings, authority score) alongside the plain
// []PrecedentRule Ingest returns, for callers that need the full picture
// (mirroring packages/statute's IngestResult convention).
type IngestResult struct {
	Rules            []ScoredPrecedent
	FailedHoldingIDs []string
	PersistedRuleIDs []string
}

// Ingest runs the full pipeline over req and returns the resulting
// []PrecedentRule, persisted via s.Store. It is a thin convenience
// wrapper over IngestDetailed for callers that only need the persisted
// rules.
//
// Returns ErrEmptyInput if req.Source is nil or req.JurisdictionCode is
// blank, ErrMalformedCorpus if the corpus cannot be parsed, or
// ErrPersistFailed (wrapping the underlying store error) if persistence
// fails partway through.
func (s *PrecedentIngestionService) Ingest(ctx context.Context, req IngestRequest) ([]PrecedentRule, error) {
	result, err := s.IngestDetailed(ctx, req)
	rules := make([]PrecedentRule, len(result.Rules))
	for i, r := range result.Rules {
		rules[i] = r.PrecedentRule
	}
	if err != nil {
		return rules, err
	}
	return rules, nil
}

// IngestDetailed runs the full precedent-ingestion pipeline over req:
//
//	load -> extract holding/ratio -> build rule nodes with citations ->
//	  tag -> weight by court hierarchy -> embed -> score authority ->
//	  persist
//
// returning an IngestResult with one ScoredPrecedent per loaded precedent.
func (s *PrecedentIngestionService) IngestDetailed(ctx context.Context, req IngestRequest) (IngestResult, error) {
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
		idPrefix = "precedent"
	}
	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	caseID := fmt.Sprintf("precedent:%s", req.JurisdictionCode)

	// 1. load
	raws, err := loader.Load(ctx, req.Source)
	if err != nil {
		return IngestResult{}, err
	}

	// 2. extract holding/ratio, 3. build rule nodes with citations
	built, failedIDs, err := BuildPrecedentRules(raws, RuleBuildOptions{
		CaseID:           caseID,
		JurisdictionCode: req.JurisdictionCode,
		LegalFamily:      req.LegalFamily,
		IDPrefix:         idPrefix,
		CreatedAt:        createdAt,
		Extractor:        s.HoldingExtractor,
	})
	if err != nil {
		return IngestResult{}, err
	}
	if len(built) == 0 {
		return IngestResult{}, ErrMalformedCorpus
	}

	// 4. tag
	tagged := TagPrecedents(built, TagOptions{
		CategoryCode:     req.CategoryCode,
		JurisdictionCode: req.JurisdictionCode,
		LegalFamily:      req.LegalFamily,
	})

	// 5. weight by court hierarchy
	hierarchy := ApplyCourtHierarchy(tagged, req.CourtLevelOverride)

	// 6. embed
	embedded, embedErr := EmbedPrecedents(ctx, s.Embedding, hierarchy, EmbedOptions{ChunkConfig: req.ChunkConfig})

	// 7. score authority
	scored := ScorePrecedents(embedded, req.ScoreAsOf)

	// 8. persist
	persistedNodes, persistErr := PersistPrecedents(ctx, store, req.JurisdictionCode, scored)
	persistedIDs := make([]string, len(persistedNodes))
	for i, n := range persistedNodes {
		persistedIDs[i] = n.ID
	}

	result := IngestResult{
		Rules:            scored,
		FailedHoldingIDs: failedIDs,
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

// ensure PrecedentRule satisfies irac.NodeLike via its embedded RuleNode,
// documenting the intended interop with heterogeneous IRAC tree handling.
var _ irac.NodeLike = PrecedentRule{}
