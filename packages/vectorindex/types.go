package vectorindex

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// Metric identifies the distance/similarity function a VectorStore uses to
// rank records against a query vector.
type Metric string

const (
	// MetricCosine ranks by cosine similarity (1 - cosine distance),
	// higher is more similar. This is the default and the only metric
	// InMemoryVectorStore implements — see IndexConfig and
	// doc/vector-index.md.
	MetricCosine Metric = "cosine"

	// MetricDotProduct ranks by raw dot product, higher is more similar.
	// Modeled here as a documented extension point for a future ANN
	// backend; InMemoryVectorStore does not implement it.
	MetricDotProduct Metric = "dot_product"

	// MetricEuclidean ranks by negative Euclidean distance, higher (i.e.
	// less negative) is more similar. Modeled here as a documented
	// extension point for a future ANN backend; InMemoryVectorStore does
	// not implement it.
	MetricEuclidean Metric = "euclidean"
)

// allMetrics is the exhaustive set of recognized Metric values.
var allMetrics = map[Metric]struct{}{
	MetricCosine:     {},
	MetricDotProduct: {},
	MetricEuclidean:  {},
}

// IsValid reports whether m is one of the recognized Metric constants.
func (m Metric) IsValid() bool {
	_, ok := allMetrics[m]
	return ok
}

// IndexConfig models the tunable knobs a real Approximate Nearest Neighbor
// (ANN) backend (e.g. pgvector's HNSW/IVFFlat indexes) would consume to
// build and query its index. InMemoryVectorStore performs exact brute-force
// search regardless of these settings — it accepts and stores IndexConfig
// so callers can configure it uniformly with a future ANN-backed
// VectorStore, but every field below is a documented no-op for the
// in-memory backend. See doc/vector-index.md's "ANN extension point"
// section.
type IndexConfig struct {
	// Metric selects the distance/similarity function. Defaults to
	// MetricCosine when left as the zero value.
	Metric Metric

	// DefaultTopK is the top-K used by Query when the caller's
	// QueryRequest.TopK is zero. Defaults to DefaultTopKValue when left as
	// the zero value.
	DefaultTopK int

	// EfSearch models HNSW's "ef" search-time candidate-list size
	// parameter: larger values trade query latency for recall. No-op for
	// InMemoryVectorStore, which always performs an exhaustive scan (i.e.
	// behaves as if ef were unbounded).
	EfSearch int

	// Candidates models an IVF-style index's number of candidate
	// partitions (or lists) to probe per query. No-op for
	// InMemoryVectorStore for the same reason as EfSearch.
	Candidates int
}

// DefaultTopKValue is the top-K used when neither QueryRequest.TopK nor
// IndexConfig.DefaultTopK is set.
const DefaultTopKValue = 10

// WithDefaults returns a copy of cfg with zero-valued fields replaced by
// their documented defaults (Metric -> MetricCosine, DefaultTopK ->
// DefaultTopKValue).
func (cfg IndexConfig) WithDefaults() IndexConfig {
	out := cfg
	if out.Metric == "" {
		out.Metric = MetricCosine
	}
	if out.DefaultTopK <= 0 {
		out.DefaultTopK = DefaultTopKValue
	}
	return out
}

// VectorRecord is one embedded leaf as stored by a VectorStore: an
// IndexableLeaf's identity and metadata, paired with the embedding vector
// computed from its Text.
type VectorRecord struct {
	// ID is the record's unique identifier. Equal to the source
	// IndexableLeaf.ID (and therefore the underlying irac.Node.ID), so a
	// vector record always traces back to the exact tree node it was
	// derived from.
	ID string

	// NodeType is the underlying node's irac.NodeType.
	NodeType irac.NodeType

	// CaseID identifies the case this record's node belongs to.
	CaseID string

	// JurisdictionCode, CategoryCode, and PartyID are the metadata-filter
	// values carried over from the source IndexableLeaf (see
	// MetadataFilter).
	JurisdictionCode JurisdictionCode
	CategoryCode     CategoryCode
	PartyID          PartyID

	// Text is the original text that was embedded, kept alongside the
	// vector so query results can be rendered without a second lookup.
	Text string

	// Vector is the embedding computed from Text.
	Vector embedding.EmbeddingVector

	// SourceSpans traces this record's text back to the ingested source
	// document(s) it was drawn from, when available (see
	// ProjectLeavesFromNodes).
	SourceSpans []irac.SourceSpan

	// ModelID and ProviderID identify which embedding model/provider
	// produced Vector, copied from the embedding.EmbeddedText that
	// produced it. Used to detect a stale record after an embedding model
	// upgrade (see ModelVersion on embedding.EmbeddingService).
	ModelID    string
	ProviderID string

	// UpdatedAt is the timestamp this record was last upserted.
	UpdatedAt time.Time
}

// MetadataFilter narrows a Query to VectorRecords matching every non-empty
// field. An empty field means "no restriction on this dimension" — e.g. a
// MetadataFilter with only CategoryCode set matches records across every
// jurisdiction and party.
type MetadataFilter struct {
	JurisdictionCode JurisdictionCode
	CategoryCode     CategoryCode
	PartyID          PartyID
}

// Matches reports whether record satisfies every non-empty field of f.
func (f MetadataFilter) Matches(record VectorRecord) bool {
	if f.JurisdictionCode != "" && f.JurisdictionCode != record.JurisdictionCode {
		return false
	}
	if f.CategoryCode != "" && f.CategoryCode != record.CategoryCode {
		return false
	}
	if f.PartyID != "" && f.PartyID != record.PartyID {
		return false
	}
	return true
}

// QueryRequest describes a similarity search against a VectorStore.
type QueryRequest struct {
	// Vector is the query embedding. Required, must be non-empty.
	Vector embedding.EmbeddingVector

	// TopK caps the number of results returned, ranked by descending
	// score. If zero, the store's configured default (IndexConfig.
	// DefaultTopK, or DefaultTopKValue) is used.
	TopK int

	// Filter, if non-zero, restricts results to VectorRecords whose
	// metadata matches every non-empty field (see MetadataFilter.Matches).
	Filter MetadataFilter

	// CaseID, if non-empty, restricts results to records belonging to a
	// single case. Empty means "search across every case" (a cross-case
	// semantic search).
	CaseID string
}

// ScoredResult is one ranked result from a Query call.
//
// This carries two independent scores so a later hybrid-retrieval phase
// (Phase 044) can fuse them without this package needing to know anything
// about graph traversal: VectorScore is fully computed here; GraphScore is
// a placeholder this package never populates (always left at its zero
// value), reserved for whatever composite-score fusion Phase 044
// implements.
type ScoredResult struct {
	// Record is the matched VectorRecord.
	Record VectorRecord

	// VectorScore is the raw similarity score this package computed
	// between the query vector and Record.Vector under the store's
	// configured Metric. Higher is more similar for every Metric this
	// package defines (see Metric's doc comments).
	VectorScore float64

	// GraphScore is a composable placeholder for a graph-traversal-based
	// relevance score (e.g. proximity to a seed node, edge-weighted
	// reachability), to be computed and populated by a future hybrid-
	// retrieval phase (Phase 044). This package never sets it — it is
	// always 0 in results returned by InMemoryVectorStore.Query.
	GraphScore float64

	// CombinedScore is a composable placeholder for whatever fusion of
	// VectorScore and GraphScore a future hybrid-retrieval phase computes
	// (e.g. a weighted sum or reciprocal-rank fusion). This package never
	// sets it — it is always 0 in results returned by
	// InMemoryVectorStore.Query. Callers should not assume it equals
	// VectorScore until a hybrid scorer populates it.
	CombinedScore float64
}
