package vectorindex

import (
	"context"
	"math"
	"sort"
	"sync"
)

// InMemoryVectorStore is a fully in-memory VectorStore implementation
// backed by a map, performing exact brute-force cosine-similarity search.
// It is the default implementation used by unit tests and by any
// deployment that does not yet need a real ANN backend — mirroring
// packages/graph's InMemoryGraphStore being the reference GraphStore
// implementation.
//
// InMemoryVectorStore is safe for concurrent use: all access to its
// internal map is serialized by mu.
type InMemoryVectorStore struct {
	mu sync.RWMutex

	// config is this store's IndexConfig, normalized via WithDefaults at
	// construction time.
	config IndexConfig

	// records maps record ID -> VectorRecord.
	records map[string]VectorRecord

	// dimensions is the vector dimensionality established by the first
	// Upsert call. Zero until the first record is stored. Subsequent
	// Upsert/Query calls with a different dimensionality are rejected with
	// ErrDimensionMismatch, since brute-force cosine similarity is
	// undefined across mismatched dimensions.
	dimensions int
}

// NewInMemoryVectorStore constructs an empty InMemoryVectorStore configured
// with cfg.WithDefaults(). Only IndexConfig.Metric == MetricCosine is
// supported today; every other Metric value is accepted (this constructor
// never errors) but Query always ranks by cosine similarity regardless, and
// this is documented on IndexConfig as a no-op knob for the in-memory
// backend.
func NewInMemoryVectorStore(cfg IndexConfig) *InMemoryVectorStore {
	return &InMemoryVectorStore{
		config:  cfg.WithDefaults(),
		records: make(map[string]VectorRecord),
	}
}

// Upsert implements VectorStore.
func (s *InMemoryVectorStore) Upsert(_ context.Context, record VectorRecord) error {
	if record.ID == "" {
		return ErrEmptyRecordID
	}
	if len(record.Vector) == 0 {
		return ErrEmptyVector
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.dimensions == 0 {
		s.dimensions = len(record.Vector)
	} else if len(record.Vector) != s.dimensions {
		return ErrDimensionMismatch
	}

	s.records[record.ID] = record
	return nil
}

// Query implements VectorStore using exhaustive cosine-similarity scoring
// over every record matching req.Filter and req.CaseID, regardless of
// IndexConfig.EfSearch/Candidates (both are documented no-ops for this
// backend — see IndexConfig).
func (s *InMemoryVectorStore) Query(_ context.Context, req QueryRequest) ([]ScoredResult, error) {
	if len(req.Vector) == 0 {
		return nil, ErrEmptyVector
	}
	if req.TopK < 0 {
		return nil, ErrInvalidTopK
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.dimensions != 0 && len(req.Vector) != s.dimensions {
		return nil, ErrDimensionMismatch
	}

	topK := req.TopK
	if topK == 0 {
		topK = s.config.DefaultTopK
	}

	results := make([]ScoredResult, 0, len(s.records))
	for _, record := range s.records {
		if req.CaseID != "" && record.CaseID != req.CaseID {
			continue
		}
		if !req.Filter.Matches(record) {
			continue
		}
		results = append(results, ScoredResult{
			Record:      record,
			VectorScore: cosineSimilarity(req.Vector, record.Vector),
		})
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].VectorScore > results[j].VectorScore
	})

	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// Delete implements VectorStore.
func (s *InMemoryVectorStore) Delete(_ context.Context, id string) error {
	if id == "" {
		return ErrEmptyRecordID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.records, id)
	return nil
}

// DeleteCase implements VectorStore.
func (s *InMemoryVectorStore) DeleteCase(_ context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, record := range s.records {
		if record.CaseID == caseID {
			delete(s.records, id)
		}
	}
	return nil
}

// Health implements VectorStore. InMemoryVectorStore has no external
// dependency to fail against: it is always healthy once constructed,
// mirroring packages/graph's InMemoryGraphStore HealthCheck behavior.
func (s *InMemoryVectorStore) Health(_ context.Context) error {
	return nil
}

// Len returns the number of records currently held by the store. Useful
// for assertions in tests.
func (s *InMemoryVectorStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// cosineSimilarity computes the cosine similarity between a and b: their
// dot product divided by the product of their magnitudes. Returns 0 if
// either vector has zero magnitude or the vectors differ in length (the
// caller is expected to have already validated matching dimensionality;
// this is a defensive fallback, not the primary validation path).
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, magA, magB float64
	for i := range a {
		dot += a[i] * b[i]
		magA += a[i] * a[i]
		magB += b[i] * b[i]
	}

	if magA == 0 || magB == 0 {
		return 0
	}

	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}
