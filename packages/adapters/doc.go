// Package adapters provides concrete implementations of the
// [github.com/YASSERRMD/verdex/packages/provider.LLMProvider] interface for
// major cloud LLM providers.
//
// # Available adapters
//
//   - [github.com/YASSERRMD/verdex/packages/adapters/anthropic] — Anthropic Claude models
//   - [github.com/YASSERRMD/verdex/packages/adapters/openai] — OpenAI GPT models and embeddings
//   - [github.com/YASSERRMD/verdex/packages/adapters/gemini] — Google Gemini models
//
// # Shared utilities
//
// The [github.com/YASSERRMD/verdex/packages/adapters/shared] sub-package
// provides an HTTP client builder with exponential-backoff retry, an SSE
// stream reader for server-sent events, and a provider-neutral HTTP status
// code mapper that converts API error responses into typed
// [github.com/YASSERRMD/verdex/packages/provider.ProviderError] values.
//
// # Usage
//
// Instantiate an adapter, register it with a
// [github.com/YASSERRMD/verdex/packages/provider.Registry], then let the
// router select the right provider at runtime:
//
//	adapter := anthropic.New(anthropic.Config{
//	    APIKey:       os.Getenv("ANTHROPIC_API_KEY"),
//	    DefaultModel: "claude-3-5-sonnet-20241022",
//	})
//	registry.Register(adapter)
package adapters
