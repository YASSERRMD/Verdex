# Embedding Service

The `embedding` package provides a provider-agnostic service for computing,
caching, versioning, and chunking text embeddings inside Verdex.

## Overview

```
┌─────────────────────────────────────┐
│         EmbeddingService            │
│  ┌──────────┐  ┌─────────────────┐  │
│  │  Chunker │  │ embeddingService│  │
│  │  (Split) │  │  Impl (Embed)   │  │
│  └────┬─────┘  └────┬────────────┘  │
│       │             │               │
│       └─────────────┤               │
│                     ▼               │
│              ┌─────────┐            │
│              │  Cache  │            │
│              └────┬────┘            │
│                   │ miss            │
│                   ▼                 │
│          ┌──────────────┐           │
│          │ LLMProvider  │           │
│          │  .Embed()    │           │
│          └──────────────┘           │
└─────────────────────────────────────┘
```

## Usage

```go
package main

import (
    "context"
    "fmt"
    "github.com/YASSERRMD/verdex/packages/embedding"
    "github.com/YASSERRMD/verdex/packages/provider"
)

func main() {
    ctx := context.Background()

    // Use any LLMProvider that supports Embed.
    p := provider.DefaultNoOpProvider()

    // In-memory cache (swap for Redis, etc. in production).
    cache := embedding.NewInMemoryCache()

    // Optional: track schema versions.
    reg := embedding.NewInMemoryVersionRegistry()

    // Optional: collect metrics.
    sink := embedding.NewAccumulatingMetricsSink()

    svc := embedding.NewEmbeddingService(p, cache,
        embedding.WithBatchSize(64),
        embedding.WithVersionRegistry(reg),
        embedding.WithMetricsSink(sink),
    )

    texts := []string{
        "Article 34(1) of the Constitution guarantees equal protection.",
        "Precedent R v Brown [1994] 1 AC 212 limits consent in criminal law.",
    }

    results, err := svc.Embed(ctx, texts)
    if err != nil {
        panic(err)
    }

    for _, r := range results {
        fmt.Printf("[%s] dims=%d version=%d\n", r.ContentHash[:8], r.Dimensions, r.Version)
    }
}
```

## Chunking Long Documents

```go
cfg := embedding.ChunkConfig{
    MaxTokens: 256,  // max whitespace-delimited words per chunk
    Overlap:   32,   // repeat last 32 words in the next chunk
    SplitOn:   "\n\n", // prefer paragraph boundaries
}

chunks, err := svc.EmbedChunked(ctx, longJudgmentText, cfg)
```

## Version-Based Re-Embedding

```go
// Record the current model.
_ = reg.RecordVersion(ctx, embedding.EmbeddingVersion{
    ModelID: "text-embedding-3-large", ProviderID: "openai", Dimensions: 3072,
})

// Later, after upgrading the model:
_ = reg.RecordVersion(ctx, embedding.EmbeddingVersion{
    ModelID: "text-embedding-4", ProviderID: "openai", Dimensions: 4096,
})

// Check whether a stored embedding is stale.
needs, _ := embedding.NeedsReEmbed(ctx, reg, storedEmbedding)
if needs {
    // Re-embed and update your index.
}
```

## Caching

The `Cache` interface abstracts the backing store.  `InMemoryCache` is
provided for tests and single-binary deployments; production services should
implement `Cache` over Redis, Memcached, or a relational store.

Cache keys are SHA-256 hashes of `"<text>|<modelID>"` (see `CacheKey`),
making them content-addressable and safe to share across instances.

## Metrics

Attach an `AccumulatingMetricsSink` (or any `MetricsSink` implementation) to
collect per-operation throughput data:

| Counter        | Description                                      |
|----------------|--------------------------------------------------|
| TotalEmbedded  | Number of texts processed in this operation      |
| CacheHits      | Texts resolved from cache without provider call  |
| CacheMisses    | Texts that required a provider call              |
| BatchCalls     | Number of individual `provider.Embed` invocations|
| Errors         | Operations that returned a non-nil error         |

## Error Handling

| Error                   | When raised                                       |
|-------------------------|---------------------------------------------------|
| `ErrEmptyInput`         | Empty text slice or empty string passed to Embed  |
| `ErrCacheMiss`          | `Cache.Get` for a hash not in the cache           |
| `ErrEmbeddingFailed`    | Upstream provider returns an error                |
| `ErrTextTooLong`        | A segment exceeds the provider token limit        |
| `ErrProviderUnsupported`| Provider does not support embeddings              |
