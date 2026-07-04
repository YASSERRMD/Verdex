package compliance

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EvidenceKind classifies what kind of artifact a ControlEvidence
// record points at.
type EvidenceKind string

const (
	// EvidenceKindAuditQuery references a specific
	// packages/auditlog.Filter/query proving a control's operations
	// are actually being recorded (e.g. "every SAR transition for this
	// tenant over the last 90 days").
	EvidenceKindAuditQuery EvidenceKind = "audit_query"

	// EvidenceKindTestName references an automated test (by fully
	// qualified Go test name) that exercises the control's behavior,
	// e.g. "packages/privacy.TestExecuteErasure_ProvenanceHashSurvives".
	EvidenceKindTestName EvidenceKind = "test_name"

	// EvidenceKindDocument references a document link (a doc/*.md
	// section, a runbook, a policy document) describing how the
	// control is satisfied procedurally.
	EvidenceKindDocument EvidenceKind = "document"

	// EvidenceKindConfiguration references a specific configuration
	// artifact (e.g. a RetentionPolicy, a Profile, an RLS
	// migration) proving the control is enforced structurally.
	EvidenceKindConfiguration EvidenceKind = "configuration"
)

// IsValid reports whether k is one of the named EvidenceKind
// constants.
func (k EvidenceKind) IsValid() bool {
	switch k {
	case EvidenceKindAuditQuery, EvidenceKindTestName, EvidenceKindDocument, EvidenceKindConfiguration:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k EvidenceKind) String() string { return string(k) }

// ControlEvidence is a tenant-scoped record of what proves a Control
// is satisfied for a given tenant/deployment (task 5): a reference to
// an auditlog query, a test name, a doc link, or a configuration
// artifact. GapAnalysis (gap.go) consumes a tenant's full
// ControlEvidence set to decide each Control's Status.
type ControlEvidence struct {
	// ID uniquely identifies this evidence record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this evidence was collected for.
	TenantID uuid.UUID `json:"tenant_id"`

	// ControlID references the Control this evidence supports.
	ControlID uuid.UUID `json:"control_id"`

	// Kind classifies what this evidence points at.
	Kind EvidenceKind `json:"kind"`

	// Reference is the actual pointer: an auditlog.Filter description,
	// a Go test name, a doc URL/path, or a configuration identifier,
	// depending on Kind. Reference only -- this package does not
	// import or dereference whatever Reference names.
	Reference string `json:"reference"`

	// Description is a short human-readable note about what this
	// evidence demonstrates.
	Description string `json:"description,omitempty"`

	// CollectedBy is the identity.User who recorded this evidence.
	CollectedBy uuid.UUID `json:"collected_by"`

	// CollectedAt is when this evidence was recorded.
	CollectedAt time.Time `json:"collected_at"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks e for structural well-formedness.
func (e *ControlEvidence) Validate() error {
	if e == nil {
		return ErrInvalidEvidence
	}
	if e.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if e.ControlID == uuid.Nil {
		return wrapf("ControlEvidence.Validate", ErrInvalidEvidence)
	}
	if !e.Kind.IsValid() {
		return wrapf("ControlEvidence.Validate", ErrInvalidEvidence)
	}
	if strings.TrimSpace(e.Reference) == "" {
		return wrapf("ControlEvidence.Validate", ErrInvalidEvidence)
	}
	if e.CollectedAt.IsZero() {
		return wrapf("ControlEvidence.Validate", ErrInvalidEvidence)
	}
	return nil
}

// EvidenceRepository persists ControlEvidence records, scoped to a
// tenant on every call, mirroring
// packages/privacy.InventoryRepository's conventions.
type EvidenceRepository interface {
	Create(ctx context.Context, tenantID uuid.UUID, e *ControlEvidence) error
	Get(ctx context.Context, tenantID, id uuid.UUID) (*ControlEvidence, error)
	ListForControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]ControlEvidence, error)
	ListAll(ctx context.Context, tenantID uuid.UUID) ([]ControlEvidence, error)
}

// RegisterControl adds control to the catalogue (task 1), requiring
// managePermission. Unlike tenant-scoped operations elsewhere in this
// package, a Control is shared reference data (see
// ControlRepository's doc comment), so RegisterControl does not take
// or check a tenantID. Returns ErrDuplicateControl if control.Code is
// already catalogued.
func (e *Engine) RegisterControl(ctx context.Context, control Control) (Control, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordControlRegister(ctx, uuid.Nil, actorFromCtx(ctx), control, err)
		}
		return Control{}, err
	}

	if control.ID == uuid.Nil {
		control.ID = uuid.New()
	}
	if control.CreatedBy == uuid.Nil {
		control.CreatedBy = user.ID
	}
	now := e.now()
	if control.CreatedAt.IsZero() {
		control.CreatedAt = now
	}
	control.UpdatedAt = now

	if err := control.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordControlRegister(ctx, user.TenantID, user.ID, control, err)
		}
		return Control{}, err
	}
	if err := e.controls.Create(ctx, &control); err != nil {
		wrapped := wrapf("RegisterControl", err)
		if e.audit != nil {
			_, _ = e.audit.RecordControlRegister(ctx, user.TenantID, user.ID, control, wrapped)
		}
		return Control{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordControlRegister(ctx, user.TenantID, user.ID, control, nil)
	}
	return control, nil
}

// ListControls returns every catalogued Control, requiring
// viewPermission.
func (e *Engine) ListControls(ctx context.Context) ([]Control, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	list, err := e.controls.List(ctx)
	if err != nil {
		return nil, wrapf("ListControls", err)
	}
	return list, nil
}

// ListControlsByFramework returns every catalogued Control mapped to
// framework, requiring viewPermission.
func (e *Engine) ListControlsByFramework(ctx context.Context, framework Framework) ([]Control, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	list, err := e.controls.ListByFramework(ctx, framework)
	if err != nil {
		return nil, wrapf("ListControlsByFramework", err)
	}
	return list, nil
}

// RecordEvidence creates a ControlEvidence record (task 5), requiring
// managePermission and tenant match. controlID must resolve to a
// catalogued Control. Every call is recorded via AuditSink regardless
// of outcome.
func (e *Engine) RecordEvidence(ctx context.Context, tenantID uuid.UUID, ev ControlEvidence) (ControlEvidence, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, actorFromCtx(ctx), ev, err)
		}
		return ControlEvidence{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, user.ID, ev, err)
		}
		return ControlEvidence{}, err
	}

	if _, err := e.controls.Get(ctx, ev.ControlID); err != nil {
		wrapped := wrapf("RecordEvidence", err)
		if e.audit != nil {
			_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, user.ID, ev, wrapped)
		}
		return ControlEvidence{}, wrapped
	}

	ev.TenantID = tenantID
	if ev.ID == uuid.Nil {
		ev.ID = uuid.New()
	}
	if ev.CollectedBy == uuid.Nil {
		ev.CollectedBy = user.ID
	}
	now := e.now()
	if ev.CollectedAt.IsZero() {
		ev.CollectedAt = now
	}
	ev.CreatedAt = now
	ev.UpdatedAt = now

	if err := ev.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, user.ID, ev, err)
		}
		return ControlEvidence{}, err
	}
	if err := e.evidence.Create(ctx, tenantID, &ev); err != nil {
		wrapped := wrapf("RecordEvidence", err)
		if e.audit != nil {
			_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, user.ID, ev, wrapped)
		}
		return ControlEvidence{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordEvidenceAdd(ctx, tenantID, user.ID, ev, nil)
	}
	return ev, nil
}

// ListEvidenceForControl returns every ControlEvidence recorded for
// tenantID against controlID, requiring viewPermission and tenant
// match.
func (e *Engine) ListEvidenceForControl(ctx context.Context, tenantID, controlID uuid.UUID) ([]ControlEvidence, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.evidence.ListForControl(ctx, tenantID, controlID)
	if err != nil {
		return nil, wrapf("ListEvidenceForControl", err)
	}
	return list, nil
}

// ListAllEvidence returns every ControlEvidence recorded for tenantID
// across every control, requiring viewPermission and tenant match.
func (e *Engine) ListAllEvidence(ctx context.Context, tenantID uuid.UUID) ([]ControlEvidence, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.evidence.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListAllEvidence", err)
	}
	return list, nil
}
