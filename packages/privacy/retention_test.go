package privacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/privacy"
)

func TestRetentionPolicy_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  privacy.RetentionPolicy
		wantErr error
	}{
		{
			name:   "valid hard delete",
			policy: privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: time.Hour, OnAction: privacy.ActionHardDelete},
		},
		{
			name:   "valid anonymize",
			policy: privacy.RetentionPolicy{Category: privacy.CategoryBehavioral, Window: time.Hour, OnAction: privacy.ActionAnonymize},
		},
		{
			name:    "invalid category",
			policy:  privacy.RetentionPolicy{Category: "bogus", Window: time.Hour, OnAction: privacy.ActionHardDelete},
			wantErr: privacy.ErrInvalidDataCategory,
		},
		{
			name:    "zero window",
			policy:  privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 0, OnAction: privacy.ActionHardDelete},
			wantErr: privacy.ErrInvalidRetentionPolicy,
		},
		{
			name:    "negative window",
			policy:  privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: -time.Hour, OnAction: privacy.ActionHardDelete},
			wantErr: privacy.ErrInvalidRetentionPolicy,
		},
		{
			name:    "retain is not a settable action",
			policy:  privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: time.Hour, OnAction: privacy.ActionRetain},
			wantErr: privacy.ErrInvalidRetentionPolicy,
		},
		{
			name:    "unknown action",
			policy:  privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: time.Hour, OnAction: "bogus"},
			wantErr: privacy.ErrInvalidRetentionPolicy,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.policy.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestEnforceRetention_WithinWindow proves a record younger than the
// policy's window is reported ActionRetain -- no premature deletion.
func TestEnforceRetention_WithinWindow(t *testing.T) {
	t.Parallel()

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 30 * 24 * time.Hour, OnAction: privacy.ActionHardDelete}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordCreatedAt := now.Add(-10 * 24 * time.Hour) // 10 days old, well within 30-day window

	action, err := privacy.EnforceRetention(policy, recordCreatedAt, now)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if action != privacy.ActionRetain {
		t.Fatalf("EnforceRetention() = %q, want %q", action, privacy.ActionRetain)
	}
}

// TestEnforceRetention_PastWindow proves a record older than the
// policy's window is reported as the policy's prescribed action.
func TestEnforceRetention_PastWindow(t *testing.T) {
	t.Parallel()

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 30 * 24 * time.Hour, OnAction: privacy.ActionAnonymize}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordCreatedAt := now.Add(-45 * 24 * time.Hour) // 45 days old, past the 30-day window

	action, err := privacy.EnforceRetention(policy, recordCreatedAt, now)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if action != privacy.ActionAnonymize {
		t.Fatalf("EnforceRetention() = %q, want %q", action, privacy.ActionAnonymize)
	}
}

// TestEnforceRetention_ExactBoundary proves the boundary is exclusive:
// a record exactly at the cutoff is treated as past the window (>=
// window means action required), matching CutoffBefore's semantics.
func TestEnforceRetention_ExactBoundary(t *testing.T) {
	t.Parallel()

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 30 * 24 * time.Hour, OnAction: privacy.ActionHardDelete}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recordCreatedAt := policy.CutoffBefore(now) // exactly at the cutoff

	action, err := privacy.EnforceRetention(policy, recordCreatedAt, now)
	if err != nil {
		t.Fatalf("EnforceRetention: %v", err)
	}
	if action != privacy.ActionHardDelete {
		t.Fatalf("EnforceRetention() at exact boundary = %q, want %q", action, privacy.ActionHardDelete)
	}
}

func TestEnforceRetention_InvalidPolicy(t *testing.T) {
	t.Parallel()

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 0, OnAction: privacy.ActionHardDelete}
	_, err := privacy.EnforceRetention(policy, time.Now(), time.Now())
	if !errors.Is(err, privacy.ErrInvalidRetentionPolicy) {
		t.Fatalf("EnforceRetention() error = %v, want ErrInvalidRetentionPolicy", err)
	}
}

func TestEngine_SetRetentionPolicy_And_EvaluateRetention(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 24 * time.Hour, OnAction: privacy.ActionHardDelete}
	if err := engine.SetRetentionPolicy(ctxWithUser(admin), policy); err != nil {
		t.Fatalf("SetRetentionPolicy: %v", err)
	}

	old := time.Now().Add(-48 * time.Hour)
	action, err := engine.EvaluateRetention(ctxWithUser(admin), privacy.CategoryTranscript, old)
	if err != nil {
		t.Fatalf("EvaluateRetention: %v", err)
	}
	if action != privacy.ActionHardDelete {
		t.Fatalf("EvaluateRetention() = %q, want %q", action, privacy.ActionHardDelete)
	}
}

func TestEngine_EvaluateRetention_NoPolicyRegistered(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.EvaluateRetention(ctxWithUser(admin), privacy.CategoryFinancial, time.Now())
	if !errors.Is(err, privacy.ErrNoRetentionPolicy) {
		t.Fatalf("EvaluateRetention() error = %v, want ErrNoRetentionPolicy", err)
	}
}

func TestEngine_SetRetentionPolicy_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	policy := privacy.RetentionPolicy{Category: privacy.CategoryTranscript, Window: 24 * time.Hour, OnAction: privacy.ActionHardDelete}
	err := engine.SetRetentionPolicy(ctxWithUser(auditor), policy)
	if !errors.Is(err, privacy.ErrForbidden) {
		t.Fatalf("SetRetentionPolicy() error = %v, want ErrForbidden", err)
	}
}
