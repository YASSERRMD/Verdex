package accounting

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PricingConfig holds per-million-token prices for a specific provider/model.
type PricingConfig struct {
	// InputPricePer1M is the cost in USD for one million input tokens.
	InputPricePer1M float64
	// OutputPricePer1M is the cost in USD for one million output tokens.
	OutputPricePer1M float64
}

// UsageRecord is an immutable record of the token consumption produced by a
// single LLM call, scoped to a tenant and optionally to a case.
type UsageRecord struct {
	// ID is a unique identifier for this record.
	ID uuid.UUID
	// TenantID identifies the tenant that made the request.
	TenantID uuid.UUID
	// CaseID optionally scopes the record to a specific judicial case.
	CaseID *uuid.UUID
	// ProviderID is the LLMProvider.ID() value (e.g. "anthropic").
	ProviderID string
	// TaskType classifies the call (e.g. "chat", "embed", "reason").
	TaskType string
	// InputTokens is the number of tokens in the prompt.
	InputTokens int
	// OutputTokens is the number of tokens in the completion.
	OutputTokens int
	// TotalTokens is InputTokens + OutputTokens (as reported by the provider).
	TotalTokens int
	// CostUSD is the estimated USD cost; nil if pricing is unavailable.
	CostUSD *float64
	// RequestID is the provider-assigned completion ID for correlation.
	RequestID string
	// CreatedAt is when the record was created (UTC).
	CreatedAt time.Time
}

// Validate checks that all token counts are non-negative and that the record
// has a non-zero TenantID.
func (r UsageRecord) Validate() error {
	if r.TenantID == uuid.Nil {
		return fmt.Errorf("accounting: UsageRecord.TenantID must not be nil")
	}
	if r.InputTokens < 0 || r.OutputTokens < 0 || r.TotalTokens < 0 {
		return ErrNegativeTokens
	}
	return nil
}

// CostEstimate computes the estimated USD cost of a single LLM call given the
// token counts and a PricingConfig.  It returns 0 when both prices are zero.
func CostEstimate(inputTokens, outputTokens int, pricing PricingConfig) float64 {
	inputCost := float64(inputTokens) / 1_000_000.0 * pricing.InputPricePer1M
	outputCost := float64(outputTokens) / 1_000_000.0 * pricing.OutputPricePer1M
	return inputCost + outputCost
}
