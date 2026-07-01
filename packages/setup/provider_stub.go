package setup

import "time"

// ProviderConfigStub is a lightweight placeholder that records the AI inference
// provider details chosen during setup.  It intentionally does not contain real
// credentials or client logic; actual provider integration is handled by a
// separate package once setup is complete.
type ProviderConfigStub struct {
	// ProviderType identifies the inference provider (e.g. "openai", "azure",
	// "anthropic", "local").
	ProviderType string `json:"provider_type"`

	// Endpoint is the base URL for the provider's API.
	Endpoint string `json:"endpoint"`

	// ModelID is the specific model to use with the provider.
	ModelID string `json:"model_id"`

	// ConfiguredAt records when this stub was last written.
	ConfiguredAt time.Time `json:"configured_at"`
}
