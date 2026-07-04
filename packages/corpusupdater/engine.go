package corpusupdater

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// CorpusText bundles a rule/precedent's current text and citation, the
// unit CorpusTextStore reads and writes.
type CorpusText struct {
	Text     string
	Citation string
}

// CorpusTextStore is the seam Engine.ApplyAmendment and Engine.Rollback
// use to read a rule/precedent's current text (so a pre-amendment
// snapshot can be captured) and write its new text (so the change
// actually lands), without this package importing
// packages/statute or packages/precedent directly. A production caller
// adapts one small implementation per corpus over the real
// packages/statute/packages/precedent persistence calls (mirroring how
// packages/statute.PersistRules itself wraps packages/graph.GraphStore
// -- this package stays one level of indirection further out, since it
// must operate across both corpora without depending on either).
type CorpusTextStore interface {
	// GetText returns targetID's current text/citation within corpus,
	// or ErrAmendmentNotFound (via the caller's own not-found
	// translated into that sentinel) if targetID does not exist yet
	// (the ChangeTypeAdd case: there is no "previous" text).
	GetText(ctx context.Context, corpus CorpusTarget, targetID string) (CorpusText, error)

	// SetText writes newText/citation as targetID's current text within
	// corpus, creating the entry if it does not already exist
	// (ChangeTypeAdd).
	SetText(ctx context.Context, corpus CorpusTarget, targetID string, text CorpusText) error
}

// Engine is the corpus-update orchestrator: it composes JobRepository,
// AmendmentRepository, and AuditSink into one set of tenant- and
// permission-scoped operations moving a CorpusUpdateJob through its
// JobStatus state machine, mirroring packages/compliance.Engine's and
// packages/privacy.Engine's shape closely: authenticate, check tenant
// match, check permission, mutate, audit regardless of outcome.
type Engine struct {
	jobs       JobRepository
	amendments AmendmentRepository
	audit      *AuditSink
	clock      func() time.Time
}

// NewEngine builds an Engine from its dependencies. jobs and amendments
// must be non-nil (ErrNilStore); audit may be nil (a nil audit sink
// means job/amendment operations simply skip audit recording -- useful
// for lightweight unit tests of the decision logic itself, though
// production callers should always supply one).
func NewEngine(jobs JobRepository, amendments AmendmentRepository, audit *AuditSink) (*Engine, error) {
	if jobs == nil || amendments == nil {
		return nil, ErrNilStore
	}
	return &Engine{jobs: jobs, amendments: amendments, audit: audit}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// CreateJob creates a new CorpusUpdateJob in StatusPending for
// tenantID, requiring managePermission.
func (e *Engine) CreateJob(ctx context.Context, tenantID uuid.UUID, job CorpusUpdateJob) (CorpusUpdateJob, error) {
	user, authErr := authorizeManage(ctx)
	if authErr == nil {
		authErr = requireMatchingUserTenant(user, tenantID)
	}
	if authErr != nil {
		e.recordJobCreate(ctx, tenantID, actorFromCtx(ctx), job, authErr)
		return CorpusUpdateJob{}, authErr
	}

	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	job.TenantID = tenantID
	job.Status = StatusPending
	job.FailureReason = ""
	job.CreatedBy = user.ID
	now := e.now()
	job.CreatedAt = now
	job.UpdatedAt = now

	if err := job.Validate(); err != nil {
		e.recordJobCreate(ctx, tenantID, user.ID, job, err)
		return CorpusUpdateJob{}, err
	}

	if err := e.jobs.Create(ctx, tenantID, &job); err != nil {
		e.recordJobCreate(ctx, tenantID, user.ID, job, err)
		return CorpusUpdateJob{}, wrapf("CreateJob", err)
	}

	e.recordJobCreate(ctx, tenantID, user.ID, job, nil)
	return job, nil
}

func (e *Engine) recordJobCreate(ctx context.Context, tenantID, actorUserID uuid.UUID, job CorpusUpdateJob, createErr error) {
	if e.audit == nil {
		return
	}
	_, _ = e.audit.RecordJobCreate(ctx, tenantID, actorUserID, job, createErr)
}

// GetJob returns the CorpusUpdateJob identified by id for tenantID,
// requiring viewPermission.
func (e *Engine) GetJob(ctx context.Context, tenantID, id uuid.UUID) (CorpusUpdateJob, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return CorpusUpdateJob{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return CorpusUpdateJob{}, err
	}
	job, err := e.jobs.Get(ctx, tenantID, id)
	if err != nil {
		return CorpusUpdateJob{}, wrapf("GetJob", err)
	}
	return *job, nil
}

// ListJobs returns every CorpusUpdateJob for tenantID, optionally
// narrowed to jurisdictionCode (pass "" for all jurisdictions),
// requiring viewPermission.
func (e *Engine) ListJobs(ctx context.Context, tenantID uuid.UUID, jurisdictionCode string) ([]CorpusUpdateJob, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	if jurisdictionCode != "" {
		jobs, err := e.jobs.ListByJurisdiction(ctx, tenantID, jurisdictionCode)
		if err != nil {
			return nil, wrapf("ListJobs", err)
		}
		return jobs, nil
	}
	jobs, err := e.jobs.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListJobs", err)
	}
	return jobs, nil
}

// transitionJob moves job to newStatus, enforcing IsValidTransition,
// persisting the change, and auditing it regardless of outcome.
func (e *Engine) transitionJob(ctx context.Context, tenantID, actorUserID uuid.UUID, job *CorpusUpdateJob, newStatus JobStatus, failureReason string) error {
	from := job.Status
	if !IsValidTransition(from, newStatus) {
		err := ErrInvalidJobTransition
		if e.audit != nil {
			_, _ = e.audit.RecordJobTransition(ctx, tenantID, actorUserID, job.ID, from, newStatus, err)
		}
		return err
	}

	job.Status = newStatus
	job.FailureReason = failureReason
	job.UpdatedAt = e.now()

	err := e.jobs.Update(ctx, tenantID, job)
	if e.audit != nil {
		_, _ = e.audit.RecordJobTransition(ctx, tenantID, actorUserID, job.ID, from, newStatus, err)
	}
	if err != nil {
		return wrapf("transitionJob", err)
	}
	return nil
}

// StageAmendment attaches a validated-later Amendment to jobID,
// requiring managePermission and jobID to be StatusPending or
// StatusValidating (a job that has already moved to StatusApplying or
// beyond no longer accepts new amendments).
func (e *Engine) StageAmendment(ctx context.Context, tenantID uuid.UUID, jobID uuid.UUID, amendment Amendment) (Amendment, error) {
	user, authErr := authorizeManage(ctx)
	if authErr == nil {
		authErr = requireMatchingUserTenant(user, tenantID)
	}
	if authErr != nil {
		e.recordAmendmentStage(ctx, tenantID, actorFromCtx(ctx), amendment, authErr)
		return Amendment{}, authErr
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		wrapped := wrapf("StageAmendment", err)
		e.recordAmendmentStage(ctx, tenantID, user.ID, amendment, wrapped)
		return Amendment{}, wrapped
	}
	if job.Status != StatusPending && job.Status != StatusValidating {
		e.recordAmendmentStage(ctx, tenantID, user.ID, amendment, ErrInvalidJobTransition)
		return Amendment{}, ErrInvalidJobTransition
	}

	if amendment.ID == uuid.Nil {
		amendment.ID = uuid.New()
	}
	amendment.TenantID = tenantID
	amendment.JobID = jobID
	amendment.TargetCorpus = job.TargetCorpus
	amendment.Applied = false
	amendment.RolledBack = false
	amendment.CreatedBy = user.ID
	now := e.now()
	amendment.CreatedAt = now
	amendment.UpdatedAt = now

	if err := e.amendments.Create(ctx, tenantID, &amendment); err != nil {
		wrapped := wrapf("StageAmendment", err)
		e.recordAmendmentStage(ctx, tenantID, user.ID, amendment, wrapped)
		return Amendment{}, wrapped
	}

	e.recordAmendmentStage(ctx, tenantID, user.ID, amendment, nil)
	return amendment, nil
}

func (e *Engine) recordAmendmentStage(ctx context.Context, tenantID, actorUserID uuid.UUID, a Amendment, stageErr error) {
	if e.audit == nil {
		return
	}
	_, _ = e.audit.RecordAmendmentStage(ctx, tenantID, actorUserID, a, stageErr)
}

// ValidateJob transitions jobID from StatusPending to StatusValidating
// and structurally checks every staged Amendment via Validate. If every
// amendment passes, the job transitions to StatusApplying and
// ValidateJob returns nil; if any amendment fails, the job transitions
// to StatusFailed (with FailureReason set to the first problem found)
// and the failure is returned. resolve is passed through to Validate
// for each amendment (may be nil).
func (e *Engine) ValidateJob(ctx context.Context, tenantID, jobID uuid.UUID, resolve TargetResolver) error {
	user, err := authorizeManage(ctx)
	if err == nil {
		err = requireMatchingUserTenant(user, tenantID)
	}
	if err != nil {
		return err
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("ValidateJob", err)
	}

	if err := e.transitionJob(ctx, tenantID, user.ID, job, StatusValidating, ""); err != nil {
		return err
	}

	staged, err := e.amendments.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("ValidateJob", err)
	}

	now := e.now()
	for _, a := range staged {
		if err := Validate(a, resolve, now); err != nil {
			_ = e.transitionJob(ctx, tenantID, user.ID, job, StatusFailed, err.Error())
			return err
		}
	}

	return e.transitionJob(ctx, tenantID, user.ID, job, StatusApplying, "")
}

// ApplyAmendment applies a single already-staged, already-effective
// Amendment to its target corpus via store: it snapshots the target's
// current text/citation into PreviousText/PreviousCitation (so
// Rollback can restore it later), writes the amendment's new text,
// re-embeds the changed text exactly once via embedder (task 4,
// skipped for ChangeTypeRepeal, which has no surviving text to embed),
// and fires a ChangeNotification via sink naming the cases resolveCases
// returns for targetID (task 5, skipped entirely when sink is nil).
//
// Marks the amendment Applied and persists it via amendments.Update.
// Returns the updated Amendment.
func (e *Engine) ApplyAmendment(ctx context.Context, tenantID uuid.UUID, a Amendment, store CorpusTextStore, embedder Embedder, sink NotificationSink, resolveCases AffectedCaseResolver) (Amendment, error) {
	user, authErr := authorizeManage(ctx)
	if authErr == nil {
		authErr = requireMatchingUserTenant(user, tenantID)
	}
	if authErr != nil {
		return Amendment{}, authErr
	}
	if store == nil {
		return Amendment{}, wrapf("ApplyAmendment", ErrNilStore)
	}

	previous := CorpusText{}
	if a.ChangeType != ChangeTypeAdd {
		var err error
		previous, err = store.GetText(ctx, a.TargetCorpus, a.TargetID)
		if err != nil {
			wrapped := wrapf("ApplyAmendment", err)
			e.recordAmendmentApply(ctx, tenantID, user.ID, a, wrapped)
			return Amendment{}, wrapped
		}
	}
	a.PreviousText = previous.Text
	a.PreviousCitation = previous.Citation

	newText := CorpusText{Text: a.NewText, Citation: a.Citation}
	targetID := a.TargetID
	if targetID == "" {
		targetID = a.ID.String()
		a.TargetID = targetID
	}
	if err := store.SetText(ctx, a.TargetCorpus, targetID, newText); err != nil {
		wrapped := wrapf("ApplyAmendment", err)
		e.recordAmendmentApply(ctx, tenantID, user.ID, a, wrapped)
		return Amendment{}, wrapped
	}

	if embedder != nil && a.ChangeType != ChangeTypeRepeal && a.NewText != "" {
		if _, err := embedder.Embed(ctx, []string{a.NewText}); err != nil {
			wrapped := wrapf("ApplyAmendment", err)
			e.recordAmendmentApply(ctx, tenantID, user.ID, a, wrapped)
			return Amendment{}, wrapped
		}
	}

	a.Applied = true
	a.UpdatedAt = e.now()
	if err := e.amendments.Update(ctx, tenantID, &a); err != nil {
		wrapped := wrapf("ApplyAmendment", err)
		e.recordAmendmentApply(ctx, tenantID, user.ID, a, wrapped)
		return Amendment{}, wrapped
	}
	e.recordAmendmentApply(ctx, tenantID, user.ID, a, nil)

	if sink != nil {
		e.notifyChange(ctx, tenantID, a, resolveCases, sink)
	}

	return a, nil
}

func (e *Engine) recordAmendmentApply(ctx context.Context, tenantID, actorUserID uuid.UUID, a Amendment, applyErr error) {
	if e.audit == nil {
		return
	}
	_, _ = e.audit.RecordAmendmentApply(ctx, tenantID, actorUserID, a, applyErr)
}

// notifyChange resolves the affected case IDs for a and fires a
// ChangeNotification via sink. Delivery/resolution failure is
// deliberately non-fatal to ApplyAmendment itself -- see
// NotificationSink's doc comment.
func (e *Engine) notifyChange(ctx context.Context, tenantID uuid.UUID, a Amendment, resolveCases AffectedCaseResolver, sink NotificationSink) {
	var affected []uuid.UUID
	if resolveCases != nil {
		if ids, err := resolveCases(ctx, a.TargetCorpus, a.TargetID); err == nil {
			affected = ids
		}
	}

	n := ChangeNotification{
		TenantID:        tenantID,
		JobID:           a.JobID,
		AmendmentID:     a.ID,
		TargetCorpus:    a.TargetCorpus,
		TargetID:        a.TargetID,
		ChangeType:      a.ChangeType,
		Citation:        a.Citation,
		AffectedCaseIDs: affected,
		OccurredAt:      e.now(),
	}
	_ = sink.NotifyChange(ctx, n)
}

// ApplyJob applies every effective (Amendment.IsEffective(now)),
// not-yet-applied Amendment staged on jobID via ApplyAmendment,
// transitioning jobID StatusApplying -> StatusApplied on full success
// or StatusApplying -> StatusFailed (with FailureReason set) on the
// first ApplyAmendment error, leaving already-applied amendments in
// their Applied state (a partial application a caller can inspect via
// ListAmendments and later Rollback).
func (e *Engine) ApplyJob(ctx context.Context, tenantID, jobID uuid.UUID, store CorpusTextStore, embedder Embedder, sink NotificationSink, resolveCases AffectedCaseResolver) error {
	user, err := authorizeManage(ctx)
	if err == nil {
		err = requireMatchingUserTenant(user, tenantID)
	}
	if err != nil {
		return err
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("ApplyJob", err)
	}
	if job.Status != StatusApplying {
		return ErrInvalidJobTransition
	}

	staged, err := e.amendments.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("ApplyJob", err)
	}

	now := e.now()
	for _, a := range staged {
		if a.Applied || !a.IsEffective(now) {
			continue
		}
		if _, err := e.ApplyAmendment(ctx, tenantID, a, store, embedder, sink, resolveCases); err != nil {
			_ = e.transitionJob(ctx, tenantID, user.ID, job, StatusFailed, err.Error())
			return err
		}
	}

	return e.transitionJob(ctx, tenantID, user.ID, job, StatusApplied, "")
}

// EffectiveAmendments returns the subset of jobID's staged Amendments
// that are currently effective (Amendment.IsEffective(now)) --
// task 3's query path for "only effective amendments are visible".
func (e *Engine) EffectiveAmendments(ctx context.Context, tenantID, jobID uuid.UUID, now time.Time) ([]Amendment, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}

	staged, err := e.amendments.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, wrapf("EffectiveAmendments", err)
	}

	out := make([]Amendment, 0, len(staged))
	for _, a := range staged {
		if a.IsEffective(now) {
			out = append(out, a)
		}
	}
	return out, nil
}

// ListAmendments returns every Amendment staged on jobID, effective or
// not, requiring viewPermission.
func (e *Engine) ListAmendments(ctx context.Context, tenantID, jobID uuid.UUID) ([]Amendment, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	amendments, err := e.amendments.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return nil, wrapf("ListAmendments", err)
	}
	return amendments, nil
}

// Rollback reverts every Applied, not-yet-RolledBack Amendment in
// jobID back to its PreviousText/PreviousCitation snapshot via store,
// then transitions jobID StatusApplied -> StatusRolledBack (task 7).
// Requires jobID to be in StatusApplied (ErrJobNotApplied otherwise).
// A ChangeTypeAdd amendment's "rollback" restores the target to its
// empty pre-existence state (store.SetText with a blank CorpusText),
// since it had no PreviousText to begin with.
func (e *Engine) Rollback(ctx context.Context, tenantID, jobID uuid.UUID, store CorpusTextStore) error {
	user, err := authorizeManage(ctx)
	if err == nil {
		err = requireMatchingUserTenant(user, tenantID)
	}
	if err != nil {
		return err
	}
	if store == nil {
		return wrapf("Rollback", ErrNilStore)
	}

	job, err := e.jobs.Get(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("Rollback", err)
	}
	if job.Status != StatusApplied {
		if e.audit != nil {
			_, _ = e.audit.RecordJobRollback(ctx, tenantID, user.ID, jobID, 0, ErrJobNotApplied)
		}
		return ErrJobNotApplied
	}

	staged, err := e.amendments.ListForJob(ctx, tenantID, jobID)
	if err != nil {
		return wrapf("Rollback", err)
	}

	reverted := 0
	for _, a := range staged {
		if !a.Applied || a.RolledBack {
			continue
		}
		restore := CorpusText{Text: a.PreviousText, Citation: a.PreviousCitation}
		if err := store.SetText(ctx, a.TargetCorpus, a.TargetID, restore); err != nil {
			wrapped := wrapf("Rollback", err)
			if e.audit != nil {
				_, _ = e.audit.RecordJobRollback(ctx, tenantID, user.ID, jobID, reverted, wrapped)
			}
			return wrapped
		}
		a.RolledBack = true
		a.UpdatedAt = e.now()
		if err := e.amendments.Update(ctx, tenantID, &a); err != nil {
			wrapped := wrapf("Rollback", err)
			if e.audit != nil {
				_, _ = e.audit.RecordJobRollback(ctx, tenantID, user.ID, jobID, reverted, wrapped)
			}
			return wrapped
		}
		reverted++
	}

	if err := e.transitionJob(ctx, tenantID, user.ID, job, StatusRolledBack, ""); err != nil {
		return err
	}
	if e.audit != nil {
		_, _ = e.audit.RecordJobRollback(ctx, tenantID, user.ID, jobID, reverted, nil)
	}
	return nil
}
