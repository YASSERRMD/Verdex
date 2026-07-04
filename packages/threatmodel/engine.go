package threatmodel

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Engine is the threat-model orchestrator: it holds an in-process
// catalogue of ThreatModels (see Catalogue) and exposes
// permission-gated, audited operations over the Mitigations within
// it -- principally TransitionMitigation, recording every attempted
// transition via AuditSink regardless of outcome, mirroring
// packages/privacy.Engine and packages/compliance.Engine's shape
// closely: authenticate, check tenant match, check permission, mutate,
// audit regardless of outcome. Unlike those Engines, Engine here holds
// no repository for the catalogue itself -- see doc.go's persistence
// discussion for why the ThreatModel catalogue is versioned-in-code
// rather than stored.
type Engine struct {
	catalogue *Catalogue
	audit     *AuditSink
	clock     func() time.Time
}

// NewEngine builds an Engine from catalogue and audit. catalogue must
// be non-nil (ErrNilRepository); audit may be nil (a nil audit sink
// means TransitionMitigation simply skips audit recording -- useful
// for lightweight unit tests of the decision logic itself, though
// production callers should always supply one).
func NewEngine(catalogue *Catalogue, audit *AuditSink) (*Engine, error) {
	if catalogue == nil {
		return nil, ErrNilRepository
	}
	return &Engine{catalogue: catalogue, audit: audit, clock: time.Now}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// Catalogue is an in-process, read-mostly index over a set of
// ThreatModels, letting a Mitigation be looked up by ID (needed before
// TransitionMitigation can validate and apply a status change) without
// requiring a database. Build one via NewCatalogue(SeedThreatModels())
// for the platform's starter catalogue, or with any caller-assembled
// []ThreatModel.
//
// Catalogue is intentionally not a Repository in this package's
// tenant-scoped sense: a ThreatModel/Threat/Mitigation is shared
// reference data (see doc.go), so there is no tenant argument anywhere
// on Catalogue's methods, exactly mirroring how
// packages/compliance.ControlRepository carries no tenant argument for
// the same reason.
type Catalogue struct {
	models      []ThreatModel
	mitigations map[uuid.UUID]mitigationLocation
}

// mitigationLocation records where within c.models a given Mitigation
// lives, so TransitionMitigation can locate and update it in place
// without a linear scan per call.
type mitigationLocation struct {
	modelIndex  int
	threatIndex int
	mitIndex    int
}

// NewCatalogue builds a Catalogue indexing every Mitigation across
// models by ID. Models are defensively copied so a caller's own slice
// mutations after construction cannot silently desync the Catalogue's
// internal index from its own copy of the data.
func NewCatalogue(models []ThreatModel) *Catalogue {
	c := &Catalogue{
		models:      make([]ThreatModel, len(models)),
		mitigations: make(map[uuid.UUID]mitigationLocation),
	}
	copy(c.models, models)
	for mi, tm := range c.models {
		for ti, th := range tm.Threats {
			for gi, m := range th.Mitigations {
				if m.ID == uuid.Nil {
					continue
				}
				c.mitigations[m.ID] = mitigationLocation{modelIndex: mi, threatIndex: ti, mitIndex: gi}
			}
		}
	}
	return c
}

// ThreatModels returns a copy of every ThreatModel in the catalogue.
func (c *Catalogue) ThreatModels() []ThreatModel {
	out := make([]ThreatModel, len(c.models))
	copy(out, c.models)
	return out
}

// FindMitigation returns a copy of the Mitigation with the given ID,
// and false if no such Mitigation is indexed.
func (c *Catalogue) FindMitigation(id uuid.UUID) (Mitigation, bool) {
	loc, ok := c.mitigations[id]
	if !ok {
		return Mitigation{}, false
	}
	return c.models[loc.modelIndex].Threats[loc.threatIndex].Mitigations[loc.mitIndex], true
}

// setMitigationStatus updates the indexed Mitigation's Status and
// LastTransitionAt in place. Caller must have already validated id is
// present via FindMitigation/c.mitigations.
func (c *Catalogue) setMitigationStatus(id uuid.UUID, status MitigationStatus, at time.Time) {
	loc := c.mitigations[id]
	m := &c.models[loc.modelIndex].Threats[loc.threatIndex].Mitigations[loc.mitIndex]
	m.Status = status
	m.LastTransitionAt = at
}

// TransitionMitigation moves the Mitigation identified by mitigationID
// from its current status to newStatus (task 1's mitigation-status
// tracking, composed with AuditSink for durable history), requiring
// managePermission and tenant match. tenantID identifies which
// tenant's deployment this transition is being recorded on behalf of
// (see AuditSink.RecordMitigationTransition's doc comment for why a
// tenant scope applies here even though the catalogue itself does
// not). The transition must be legal per CanTransitionMitigation;
// an illegal transition (e.g. skipping directly from Planned to
// Verified) is rejected with ErrIllegalStatusTransition before any
// state changes, and is still recorded via AuditSink as a denied
// attempt.
func (e *Engine) TransitionMitigation(ctx context.Context, tenantID uuid.UUID, mitigationID uuid.UUID, newStatus MitigationStatus) (Mitigation, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, actorFromCtx(ctx), mitigationID, "", newStatus, err)
		}
		return Mitigation{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, "", newStatus, err)
		}
		return Mitigation{}, err
	}

	current, ok := e.catalogue.FindMitigation(mitigationID)
	if !ok {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, "", newStatus, ErrMitigationNotFound)
		}
		return Mitigation{}, ErrMitigationNotFound
	}

	if !newStatus.IsValid() {
		wrapped := wrapf("TransitionMitigation", ErrInvalidMitigation)
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, current.Status, newStatus, wrapped)
		}
		return Mitigation{}, wrapped
	}

	if !CanTransitionMitigation(current.Status, newStatus) {
		wrapped := wrapf("TransitionMitigation", ErrIllegalStatusTransition)
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, current.Status, newStatus, wrapped)
		}
		return Mitigation{}, wrapped
	}

	e.catalogue.setMitigationStatus(mitigationID, newStatus, e.now())
	updated, _ := e.catalogue.FindMitigation(mitigationID)

	if e.audit != nil {
		_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, current.Status, newStatus, nil)
	}
	return updated, nil
}

// ResetMitigation forcibly moves the Mitigation identified by
// mitigationID back to MitigationPlanned regardless of its current
// status -- the one sanctioned way to regress a MitigationVerified
// mitigation (see CanTransitionMitigation's doc comment), used when a
// previously verified control is found to have regressed and must be
// re-verified from scratch. reason is required (ErrInputInvalidStructure
// if blank) and is recorded in the audit Detail, since an unexplained
// regression of a verified control is exactly the kind of event this
// package's audit trail must make reviewable.
func (e *Engine) ResetMitigation(ctx context.Context, tenantID uuid.UUID, mitigationID uuid.UUID, reason string) (Mitigation, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, actorFromCtx(ctx), mitigationID, "", MitigationPlanned, err)
		}
		return Mitigation{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, "", MitigationPlanned, err)
		}
		return Mitigation{}, err
	}
	if err := ValidateNonBlank(reason); err != nil {
		wrapped := wrapf("ResetMitigation", err)
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, "", MitigationPlanned, wrapped)
		}
		return Mitigation{}, wrapped
	}

	current, ok := e.catalogue.FindMitigation(mitigationID)
	if !ok {
		if e.audit != nil {
			_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, "", MitigationPlanned, ErrMitigationNotFound)
		}
		return Mitigation{}, ErrMitigationNotFound
	}

	e.catalogue.setMitigationStatus(mitigationID, MitigationPlanned, e.now())
	updated, _ := e.catalogue.FindMitigation(mitigationID)

	if e.audit != nil {
		_, _ = e.audit.RecordMitigationTransition(ctx, tenantID, user.ID, mitigationID, current.Status, MitigationPlanned, nil)
	}
	return updated, nil
}

// ListThreatModels returns every ThreatModel in the catalogue,
// requiring viewPermission.
func (e *Engine) ListThreatModels(ctx context.Context) ([]ThreatModel, error) {
	if _, err := authorizeView(ctx); err != nil {
		return nil, err
	}
	return e.catalogue.ThreatModels(), nil
}

// GetMitigation returns the Mitigation identified by mitigationID,
// requiring viewPermission.
func (e *Engine) GetMitigation(ctx context.Context, mitigationID uuid.UUID) (Mitigation, error) {
	if _, err := authorizeView(ctx); err != nil {
		return Mitigation{}, err
	}
	m, ok := e.catalogue.FindMitigation(mitigationID)
	if !ok {
		return Mitigation{}, ErrMitigationNotFound
	}
	return m, nil
}

// MitigationHistory returns every recorded transition for tenantID
// matching filter, requiring viewPermission and tenant match.
func (e *Engine) MitigationHistory(ctx context.Context, tenantID uuid.UUID, filter auditlog.Filter) ([]auditlog.Event, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	if e.audit == nil {
		return nil, ErrNilAuditSink
	}
	return e.audit.MitigationHistory(ctx, tenantID, filter)
}
