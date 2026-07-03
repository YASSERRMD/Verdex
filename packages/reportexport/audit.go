package reportexport

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/observability"
)

// auditAction is the stable verb-phrase recorded on every
// observability.AuditEvent produced from an AuditRecord, mirroring
// caselifecycle.TransitionRecord.ToAuditEvent's "<noun>.<verb>"
// convention.
const auditAction = "report.exported"

// AuditRecord is one immutable record of a single export action: who
// exported what case, in which format, with redaction on or off, and
// when. AuditRecords are append-only — there is no Update method on
// AuditRepository.
type AuditRecord struct {
	// ID uniquely identifies this audit record.
	ID uuid.UUID

	// TenantID is the tenant the exported case belongs to.
	TenantID uuid.UUID

	// CaseID is the case that was exported.
	CaseID uuid.UUID

	// ActorID is the identity.User who performed the export.
	ActorID uuid.UUID

	// Format is the export format requested (FormatPDF, FormatDOCX,
	// FormatMarkdown, or FormatText).
	Format Format

	// Redacted is true if the export was produced with the PII
	// redaction pass applied (see redact.go).
	Redacted bool

	// ExportedAt is when the export occurred.
	ExportedAt time.Time
}

// ToAuditEvent projects a into an observability.AuditEvent, so export
// actions flow through the platform's single audit channel rather
// than a second, parallel logging path — mirroring
// caselifecycle.TransitionRecord.ToAuditEvent exactly.
func (a *AuditRecord) ToAuditEvent() observability.AuditEvent {
	outcome := "success"
	return observability.AuditEvent{
		Time:    a.ExportedAt,
		Actor:   a.ActorID.String(),
		Action:  auditAction,
		Target:  a.CaseID.String(),
		Outcome: outcome,
	}
}

// AuditFilter narrows AuditRepository.List to a subset of a tenant's
// export records.
type AuditFilter struct {
	// CaseID, if non-nil, restricts results to that case.
	CaseID *uuid.UUID

	// ActorID, if non-nil, restricts results to that actor.
	ActorID *uuid.UUID

	// Format, if non-empty, restricts results to that Format.
	Format Format

	// Since, if non-zero, restricts results to records with
	// ExportedAt >= Since.
	Since time.Time
}

// AuditRepository persists AuditRecords, scoped to a tenant on every
// call, mirroring packages/notifications.Repository's convention:
// implementations must refuse (via ErrCrossTenantAccess) to operate on
// a record whose TenantID does not match the tenantID argument.
type AuditRepository interface {
	// Create inserts rec. rec.ID and rec.ExportedAt are generated if
	// zero. Returns ErrNilRecord if rec is nil, and
	// ErrCrossTenantAccess if rec.TenantID does not match tenantID.
	Create(ctx context.Context, tenantID uuid.UUID, rec *AuditRecord) error

	// List returns audit records for tenantID, optionally narrowed by
	// filter, ordered by ExportedAt descending (newest first).
	List(ctx context.Context, tenantID uuid.UUID, filter AuditFilter) ([]*AuditRecord, error)
}
