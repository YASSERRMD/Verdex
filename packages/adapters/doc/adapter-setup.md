# Cloud Adapter Setup

This document describes how to configure and use each of the three cloud
provider adapters shipped in `packages/adapters`.

---

## Anthropic

### Configuration

| Field          | Type            | Default                          | Required |
|----------------|-----------------|----------------------------------|----------|
| `APIKey`       | `string`        | —                                | Yes      |
| `BaseURL`      | `string`        | `https://api.anthropic.com`      | No       |
| `DefaultModel` | `string`        | `claude-3-5-sonnet-20241022`     | No       |
| `Timeout`      | `time.Duration` | `120s`                           | No       |

### Example

```go
import "github.com/YASSERRMD/verdex/packages/adapters/anthropic"

adapter, err := anthropic.New(anthropic.Config{
    APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
    DefaultModel: "claude-3-5-sonnet-20241022",
})
```

### Notes

- Embedding is **not** supported; `Embed()` returns `ErrProviderUnavailable`.
- System messages are automatically extracted from the message list and sent
  via Anthropic's top-level `system` field.
- The adapter sets the `anthropic-version: 2023-06-01` header on every request.

---

## OpenAI

### Configuration

| Field        | Type            | Default                        | Required |
|--------------|-----------------|--------------------------------|----------|
| `APIKey`     | `string`        | —                              | Yes      |
| `BaseURL`    | `string`        | `https://api.openai.com`       | No       |
| `ChatModel`  | `string`        | `gpt-4o`                       | No       |
| `EmbedModel` | `string`        | `text-embedding-3-small`       | No       |
| `Timeout`    | `time.Duration` | `120s`                         | No       |

### Example

```go
import "github.com/YASSERRMD/verdex/packages/adapters/openai"

adapter, err := openai.New(openai.Config{
    APIKey:     os.Getenv("OPENAI_API_KEY"),
    ChatModel:  "gpt-4o",
    EmbedModel: "text-embedding-3-small",
})
```

### Azure OpenAI

Override `BaseURL` with your Azure endpoint and set `APIKey` to your Azure key.

```go
adapter, err := openai.New(openai.Config{
    APIKey:  os.Getenv("AZURE_OPENAI_KEY"),
    BaseURL: "https://<resource>.openai.azure.com/openai/deployments/<deployment>",
})
```

### Notes

- The adapter sends `Authorization: Bearer <key>` on every request.
- OpenAI streaming uses `data: [DONE]` to signal end-of-stream; the adapter
  handles this transparently.

---

## Gemini

### Configuration

| Field     | Type            | Default                                        | Required |
|-----------|-----------------|------------------------------------------------|----------|
| `APIKey`  | `string`        | —                                              | Yes      |
| `BaseURL` | `string`        | `https://generativelanguage.googleapis.com`    | No       |
| `ModelID` | `string`        | `gemini-1.5-pro`                               | No       |
| `Timeout` | `time.Duration` | `120s`                                         | No       |

### Example

```go
import "github.com/YASSERRMD/verdex/packages/adapters/gemini"

adapter, err := gemini.New(gemini.Config{
    APIKey:  os.Getenv("GEMINI_API_KEY"),
    ModelID: "gemini-1.5-pro",
})
```

### Notes

- The API key is appended as a `?key=<key>` query parameter (Gemini does not
  use an `Authorization` header for API-key authentication).
- Embedding uses `batchEmbedContents` with the `text-embedding-004` model by
  default; pass `EmbedRequest.Model` to override.
- Gemini uses `"model"` for the assistant role; the adapter maps
  `provider.Message{Role: "assistant"}` to `"model"` automatically.
- System messages are sent via Gemini's top-level `systemInstruction` field.

---

## Shared Behaviour

All three adapters share the following behaviour:

- **Exponential-backoff retry**: transient HTTP errors (429, 503, 5xx) are
  retried up to three times with back-offs of 200 ms, 400 ms, and 800 ms.
- **SSE streaming**: `ChatStream` returns a `<-chan provider.StreamChunk`
  channel that must be fully drained by the caller.
- **Context cancellation**: all blocking calls respect `context.Context`.
  Cancelling the context closes the stream channel promptly.
- **Thread safety**: all adapters are safe for concurrent use.
