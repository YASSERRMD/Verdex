# Local / Self-hosted Model Deployment Guide

This guide explains how to configure the Verdex `local` adapter to connect to
a locally-hosted LLM server. The adapter speaks the OpenAI-compatible REST API
so it works with any server that exposes `/v1/chat/completions`, `/v1/embeddings`,
and `/v1/models`.

---

## Supported Backends

| Server | Default Port | Notes |
|--------|-------------|-------|
| [Ollama](https://ollama.ai) | 11434 | Simplest setup; manages model downloads |
| [LM Studio](https://lmstudio.ai) | 1234 | GUI-first; good for development |
| [vLLM](https://vllm.ai) | 8000 | High-throughput GPU inference |
| [LocalAI](https://localai.io) | 8080 | CPU-first; broad model format support |
| [llama.cpp server](https://github.com/ggerganov/llama.cpp) | 8080 | Minimal; ideal for embedded / edge |

---

## Ollama

### Installation

```bash
# macOS
brew install ollama

# Linux
curl -fsSL https://ollama.com/install.sh | sh
```

### Start the server

```bash
ollama serve
```

### Pull a model

```bash
# Chat model
ollama pull llama3.2:3b

# Embedding model
ollama pull nomic-embed-text
```

### Verdex adapter config

```go
adapter, err := local.New(local.Config{
    BaseURL:      "http://localhost:11434",
    ModelID:      "llama3.2:3b",
    EmbedModelID: "nomic-embed-text",
    MaxConcurrency: 2,
})
```

---

## LM Studio

1. Download and install from https://lmstudio.ai
2. Open the **Local Server** tab (left sidebar)
3. Load a model from the catalogue
4. Start the server (default port 1234)

```go
adapter, err := local.New(local.Config{
    BaseURL: "http://localhost:1234",
    ModelID: "lmstudio-community/Meta-Llama-3-8B-Instruct-GGUF",
})
```

---

## vLLM

vLLM requires a CUDA-capable GPU and is the recommended backend for production
self-hosted deployments.

### Installation

```bash
pip install vllm
```

### Start the server

```bash
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Meta-Llama-3-8B-Instruct \
  --port 8000
```

### Verdex adapter config

```go
adapter, err := local.New(local.Config{
    BaseURL:        "http://localhost:8000",
    ModelID:        "meta-llama/Meta-Llama-3-8B-Instruct",
    MaxConcurrency: 8, // vLLM handles concurrency internally; adjust to taste
})
```

---

## Quantization Notes

GGUF quantization reduces VRAM/RAM requirements at the cost of some accuracy.
Common quantization levels:

| Level | Size reduction | Accuracy loss | Recommended use |
|-------|---------------|---------------|-----------------|
| Q8_0  | ~13% | Negligible | Best quality, fits in RAM |
| Q6_K  | ~25% | Very low | Recommended for 7–13 B |
| Q5_K_M | ~35% | Low | Good balance |
| Q4_K_M | ~45% | Moderate | Most popular, good balance |
| Q3_K_M | ~55% | Noticeable | Memory-constrained only |
| Q2_K  | ~65% | High | Not recommended for reasoning |

**Model loading time**: GGUF models are memory-mapped at startup. A 70 B Q4_K_M
model (~40 GB) can take 30-60 seconds to load on NVMe storage. The health check
will fail during this window. Use a retry loop:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

for {
    if err := adapter.HealthCheck(ctx); err == nil {
        break
    }
    time.Sleep(5 * time.Second)
}
```

---

## Air-gap / Offline Deployments

Set `OfflineMode: true` to activate the `OfflineModeEnforcer`. In this mode
the adapter validates that every request targets a loopback address
(`localhost`, `127.x.x.x`, `::1`) before sending. Any attempt to call an
external URL returns an error (or panics in test mode).

```go
adapter, err := local.New(local.Config{
    BaseURL:     "http://127.0.0.1:11434",
    ModelID:     "llama3.2:3b",
    OfflineMode: true,
})
```

**Air-gap checklist:**

- [ ] Pre-pull all GGUF files before disconnecting from the internet
- [ ] Disable the OS-level network interface if strict isolation is required
- [ ] Use `DiscoverModels` to verify the model is loaded before serving traffic
- [ ] Set `Timeout` generously (300 s+ for large models on CPU)
- [ ] Consider a health-check startup probe in your container/systemd unit

---

## Concurrency Tuning

The `MaxConcurrency` config option limits simultaneous in-flight requests to
the local server. Unlike cloud APIs, local servers are bound by the underlying
hardware:

| Hardware | Recommended MaxConcurrency |
|----------|---------------------------|
| CPU only | 1 |
| Single GPU | 1–2 |
| Multi-GPU (tensor parallel) | 4–8 |
| vLLM with continuous batching | 8–32 |

Setting `MaxConcurrency` too high on a CPU-only setup will cause all requests
to slow down rather than proceeding in parallel. Start with `1` and increase
only after benchmarking.

---

## Discovery

Use `DiscoverModels` to list loaded models before routing requests:

```go
models, err := local.DiscoverModels(ctx, "http://localhost:11434")
if err != nil {
    log.Fatalf("local server unreachable: %v", err)
}
for _, m := range models {
    fmt.Printf("model: %s\n", m.ID)
}
```

This is useful in startup probes and admin dashboards.
