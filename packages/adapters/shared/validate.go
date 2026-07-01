package shared

import (
	"fmt"
	"time"
)

// ValidateAPIKey returns a formatted error when key is empty.
func ValidateAPIKey(providerID, key string) error {
	if key == "" {
		return fmt.Errorf("%s: APIKey must not be empty", providerID)
	}
	return nil
}

// ValidateTimeout returns the provided timeout, or the fallback if it is
// non-positive. This allows callers to record a canonical validated value.
func ValidateTimeout(t, fallback time.Duration) time.Duration {
	if t <= 0 {
		return fallback
	}
	return t
}

// ValidateModel returns model if non-empty, otherwise fallback. Useful for
// ensuring a default model is always set after configuration.
func ValidateModel(model, fallback string) string {
	if model == "" {
		return fallback
	}
	return model
}
