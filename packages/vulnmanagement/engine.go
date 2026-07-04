package vulnmanagement

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Engine is the vulnerability-management orchestrator: it composes a
// tenant's Finding set, TriageDecision history, and SLA/reporting logic
// into one set of tenant- and permission-scoped operations, recording
// every finding recorded and triage decision via AuditSink. Engine
// mirrors packages/compliance.Engine's and packages/threatmodel.Engine's
// shape closely: authenticate, check tenant match, check permission,
// mutate, audit regardless of outcome.
type Engine struct {
	findings FindingRepository
	triage   TriageRepository
	audit    *AuditSink
	clock    func() time.Time
}

// NewEngine builds an Engine from its dependencies. findings and
// triage must be non-nil (ErrNilStore); audit may be nil (a nil audit
// sink means finding/triage operations simply skip audit recording --
// useful for lightweight unit tests of the decision logic itself,
// though production callers should always supply one).
func NewEngine(findings FindingRepository, triage TriageRepository, audit *AuditSink) (*Engine, error) {
	if findings == nil || triage == nil {
		return nil, ErrNilStore
	}
	return &Engine{
		findings: findings,
		triage:   triage,
		audit:    audit,
		clock:    time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// RecordFinding creates a new Finding for tenantID (the ingestion side
// of tasks 1-3: SCA/SAST/container scanners all funnel their output
// through this one call), requiring managePermission and tenant match.
// A fresh Finding always starts at StatusOpen regardless of what the
// caller supplies, since a newly-scanned finding has not yet been
// triaged by definition. Every call is recorded via AuditSink
// regardless of outcome.
func (e *Engine) RecordFinding(ctx context.Context, tenantID uuid.UUID, f Finding) (Finding, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingRecord(ctx, tenantID, actorFromCtx(ctx), f, err)
		}
		return Finding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingRecord(ctx, tenantID, user.ID, f, err)
		}
		return Finding{}, err
	}

	f.TenantID = tenantID
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	f.Status = StatusOpen
	now := e.now()
	if f.DiscoveredAt.IsZero() {
		f.DiscoveredAt = now
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = now
	}
	f.UpdatedAt = now

	if err := f.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordFindingRecord(ctx, tenantID, user.ID, f, err)
		}
		return Finding{}, err
	}
	if err := e.findings.Create(ctx, tenantID, &f); err != nil {
		wrapped := wrapf("RecordFinding", err)
		if e.audit != nil {
			_, _ = e.audit.RecordFindingRecord(ctx, tenantID, user.ID, f, wrapped)
		}
		return Finding{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordFindingRecord(ctx, tenantID, user.ID, f, nil)
	}
	return f, nil
}

// GetFinding returns the Finding identified by id for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) GetFinding(ctx context.Context, tenantID, id uuid.UUID) (Finding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return Finding{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Finding{}, err
	}
	f, err := e.findings.Get(ctx, tenantID, id)
	if err != nil {
		return Finding{}, wrapf("GetFinding", err)
	}
	return *f, nil
}

// ListFindings returns every Finding recorded for tenantID, requiring
// viewPermission and tenant match.
func (e *Engine) ListFindings(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.findings.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListFindings", err)
	}
	return list, nil
}

// ListFindingsBySource returns every Finding recorded for tenantID
// produced by source, requiring viewPermission and tenant match.
func (e *Engine) ListFindingsBySource(ctx context.Context, tenantID uuid.UUID, source ScannerSource) ([]Finding, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.findings.ListBySource(ctx, tenantID, source)
	if err != nil {
		return nil, wrapf("ListFindingsBySource", err)
	}
	return list, nil
}
