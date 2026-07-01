package local

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ModelInfo describes a single model registered in the local server's model
// catalogue, as returned by GET /v1/models.
type ModelInfo struct {
	// ID is the model identifier, e.g. "llama3:8b" (Ollama) or
	// "lmstudio-community/Meta-Llama-3-8B-Instruct-GGUF" (LM Studio).
	ID string `json:"id"`
	// Object is always "model" per the OpenAI spec.
	Object string `json:"object"`
	// Created is the Unix timestamp (as a string) at which the model entry
	// was registered. Some servers return "" when not applicable.
	Created json.Number `json:"created"`
}

// modelsListResponse mirrors the OpenAI GET /v1/models response envelope.
type modelsListResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// DiscoverModels calls GET /v1/models on the local server identified by
// baseURL and returns the list of available models.
//
// It returns [ErrOfflineEndpointUnreachable] when the server cannot be reached
// (connection refused, timeout, DNS failure, etc.) so callers can distinguish
// a misconfigured server from an API-level error.
func DiscoverModels(ctx context.Context, baseURL string) ([]ModelInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("local: building /v1/models request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfflineEndpointUnreachable, err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: /v1/models returned HTTP %d", ErrOfflineEndpointUnreachable, resp.StatusCode)
	}

	var list modelsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("local: decoding /v1/models response: %w", err)
	}
	return list.Data, nil
}
