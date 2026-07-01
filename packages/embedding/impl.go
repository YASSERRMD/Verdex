package embedding

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// Option is a functional option for [NewEmbeddingService].
type Option func(*embeddingServiceImpl)

// WithBatchSize sets the maximum number of texts sent to the provider in a
// single Embed call.  Defaults to 32.
func WithBatchSize(n int) Option {
	return func(s *embeddingServiceImpl) {
		if n > 0 {
			s.batchSize = n
		}
	}
}

// WithVersionRegistry attaches a [VersionRegistry] so the service can record
// and compare embedding schema versions.
func WithVersionRegistry(r VersionRegistry) Option {
	return func(s *embeddingServiceImpl) {
		s.versionReg = r
	}
}

// WithMetricsSink attaches a [MetricsSink] to receive telemetry on each
// operation.
func WithMetricsSink(m MetricsSink) Option {
	return func(s *embeddingServiceImpl) {
		s.sink = m
	}
}

// embeddingServiceImpl is the canonical implementation of [EmbeddingService].
type embeddingServiceImpl struct {
	provider     provider.LLMProvider
	cache        Cache
	batchSize    int
	modelVersion string
	versionReg   VersionRegistry
	sink         MetricsSink

	// running counters flushed via sink after each Embed call.
	metrics EmbeddingMetrics
}

// NewEmbeddingService constructs a production [EmbeddingService] backed by
// the given [provider.LLMProvider] and [Cache].  Functional options customise
// batching, versioning, and telemetry.
func NewEmbeddingService(p provider.LLMProvider, cache Cache, opts ...Option) EmbeddingService {
	cap := p.Capabilities()
	version := fmt.Sprintf("%s/%s/v1", cap.ProviderID, cap.ModelID)

	s := &embeddingServiceImpl{
		provider:     p,
		cache:        cache,
		batchSize:    32,
		modelVersion: version,
		sink:         NoOpMetricsSink{},
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Embed implements [EmbeddingService].
func (s *embeddingServiceImpl) Embed(ctx context.Context, texts []string) ([]EmbeddedText, error) {
	if len(texts) == 0 {
		return nil, ErrEmptyInput
	}

	var m EmbeddingMetrics

	cap := s.provider.Capabilities()
	modelID := cap.ModelID
	providerID := cap.ProviderID

	// Determine current schema version for stamping.
	version := 1
	if s.versionReg != nil {
		if v, err := s.versionReg.CurrentVersion(ctx); err == nil && v != nil {
			version = v.Version
		}
	}

	// Build result slice and identify cache misses.
	results := make([]EmbeddedText, len(texts))
	missIndices := make([]int, 0, len(texts))
	missTexts := make([]string, 0, len(texts))

	for i, text := range texts {
		hash := CacheKey(text, modelID)
		cached, err := s.cache.Get(ctx, hash)
		if err == nil && cached != nil {
			results[i] = *cached
			atomic.AddInt64(&m.CacheHits, 1)
			continue
		}
		atomic.AddInt64(&m.CacheMisses, 1)
		missIndices = append(missIndices, i)
		missTexts = append(missTexts, text)
	}

	// Batch-call provider for misses.
	if len(missTexts) > 0 {
		embedded, err := s.batchEmbed(ctx, missTexts, modelID, providerID, version, &m)
		if err != nil {
			atomic.AddInt64(&m.Errors, 1)
			s.sink.Record(m)
			return nil, err
		}
		for j, idx := range missIndices {
			et := embedded[j]
			results[idx] = et
			// Store in cache; ignore cache write errors.
			_ = s.cache.Set(ctx, et.ContentHash, et)
		}
	}

	atomic.AddInt64(&m.TotalEmbedded, int64(len(texts)))
	s.sink.Record(m)
	return results, nil
}

// batchEmbed splits texts into groups of batchSize and calls provider.Embed
// for each group.
func (s *embeddingServiceImpl) batchEmbed(
	ctx context.Context,
	texts []string,
	modelID, providerID string,
	version int,
	m *EmbeddingMetrics,
) ([]EmbeddedText, error) {
	results := make([]EmbeddedText, 0, len(texts))

	for start := 0; start < len(texts); start += s.batchSize {
		end := start + s.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[start:end]

		resp, err := s.provider.Embed(ctx, provider.EmbedRequest{
			Texts: batch,
			Model: modelID,
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEmbeddingFailed, err)
		}
		atomic.AddInt64(&m.BatchCalls, 1)

		for i, vec := range resp.Embeddings {
			text := batch[i]
			hash := CacheKey(text, modelID)
			et := EmbeddedText{
				ContentHash: hash,
				Text:        text,
				Vector:      EmbeddingVector(vec),
				Dimensions:  resp.Dimensions,
				ModelID:     modelID,
				ProviderID:  providerID,
				Version:     version,
				CreatedAt:   time.Now().UTC(),
			}
			results = append(results, et)
		}
	}
	return results, nil
}

// EmbedChunked implements [EmbeddingService].
func (s *embeddingServiceImpl) EmbedChunked(ctx context.Context, text string, cfg ChunkConfig) ([]EmbeddedText, error) {
	if text == "" {
		return nil, ErrEmptyInput
	}
	c := &Chunker{}
	chunks := c.Split(text, cfg)
	if len(chunks) == 0 {
		return nil, ErrEmptyInput
	}
	return s.Embed(ctx, chunks)
}

// Invalidate implements [EmbeddingService].
func (s *embeddingServiceImpl) Invalidate(ctx context.Context, contentHash string) error {
	return s.cache.Delete(ctx, contentHash)
}

// ModelVersion implements [EmbeddingService].
func (s *embeddingServiceImpl) ModelVersion() string {
	return s.modelVersion
}
