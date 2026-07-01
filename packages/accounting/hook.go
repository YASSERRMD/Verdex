package accounting

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// AccountingHook implements provider.TokenAccountingHook and forwards every
// recorded usage event to an AccountingService.
//
// It is safe for concurrent use by multiple goroutines.
type AccountingHook struct {
	mu       sync.Mutex
	svc      *AccountingService
	tenantID uuid.UUID // default tenant used when context carries no tenant
}

// NewAccountingHook constructs an AccountingHook that routes usage events to
// svc under defaultTenantID when no tenant override is present in the context.
func NewAccountingHook(svc *AccountingService, defaultTenantID uuid.UUID) *AccountingHook {
	return &AccountingHook{svc: svc, tenantID: defaultTenantID}
}

// RecordUsage implements provider.TokenAccountingHook.
//
// It constructs a UsageRecord from the provider call metadata and delegates to
// AccountingService.RecordUsage.  Errors are swallowed because the hook MUST
// NOT interfere with the calling LLM request path.
func (h *AccountingHook) RecordUsage(ctx context.Context, providerID string, usage provider.TokenUsage, task provider.TaskType) {
	// Extract tenant and case IDs from the context if available.
	tenantID := h.resolveTenantID(ctx)
	caseID := caseIDFromContext(ctx)

	record := UsageRecord{
		ID:           uuid.New(),
		TenantID:     tenantID,
		CaseID:       caseID,
		ProviderID:   providerID,
		TaskType:     string(task),
		InputTokens:  usage.InputTokens,
		OutputTokens: usage.OutputTokens,
		TotalTokens:  usage.TotalTokens,
		CreatedAt:    time.Now().UTC(),
	}

	// Fire-and-forget: swallow errors to not impact the LLM request path.
	h.mu.Lock()
	defer h.mu.Unlock()
	_ = h.svc.RecordUsage(ctx, record)
}

// resolveTenantID returns the tenant ID from the context if set, otherwise the
// default tenant ID configured at hook construction time.
func (h *AccountingHook) resolveTenantID(ctx context.Context) uuid.UUID {
	if id := tenantIDFromContext(ctx); id != uuid.Nil {
		return id
	}
	return h.tenantID
}

// --- context helpers ---------------------------------------------------------

// accountingTenantKey is the context key for tenant ID injection.
type accountingTenantKey struct{}

// accountingCaseKey is the context key for case ID injection.
type accountingCaseKey struct{}

// WithTenantID returns a new context carrying the given tenant ID so that
// AccountingHook can associate the usage record with the correct tenant.
func WithTenantID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, accountingTenantKey{}, id)
}

// WithCaseID returns a new context carrying the given case ID.
func WithCaseID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, accountingCaseKey{}, id)
}

func tenantIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(accountingTenantKey{}).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

func caseIDFromContext(ctx context.Context) *uuid.UUID {
	if v, ok := ctx.Value(accountingCaseKey{}).(uuid.UUID); ok && v != uuid.Nil {
		id := v
		return &id
	}
	return nil
}
