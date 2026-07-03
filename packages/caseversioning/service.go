package caseversioning

import (
	"context"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// Service is the access-controlled entrypoint for case-versioning
// operations, composing a Repository (snapshot storage) with
// packages/caselifecycle.Repository (the live Case that
// ArtifactCaseMetadata snapshots capture and Restore reverts),
// mirroring how packages/annotations.Service composes
// caselifecycle.Repository rather than reimplementing case lookup.
//
// Service is deliberately not the store of truth for tree revisions,
// evidence, or opinions — see doc.go, "Composition, not
// reimplementation". It only records references (ArtifactTree,
// ArtifactEvidence) or compact copies (ArtifactOpinion) of state that
// lives in packages/irac/packages/treeassembly, packages/annotations/
// packages/evidence, and packages/synthesisagent respectively.
type Service struct {
	repo  Repository
	cases caselifecycle.Repository
	now   func() time.Time
}

// NewService builds a Service. repo and cases must be non-nil.
func NewService(repo Repository, cases caselifecycle.Repository) (*Service, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if cases == nil {
		return nil, ErrNilCaseRepository
	}
	return &Service{repo: repo, cases: cases, now: func() time.Time { return time.Now().UTC() }}, nil
}

// checkCaseAccess confirms caseID is visible to tenantID via the
// composed caselifecycle.Repository, translating any lookup failure
// into ErrForbidden so callers cannot distinguish "case does not exist"
// from "case belongs to another tenant" — the same information-hiding
// posture packages/annotations takes.
func (s *Service) checkCaseAccess(ctx context.Context, tenantID, caseID uuid.UUID) (*caselifecycle.Case, error) {
	c, err := s.cases.Get(ctx, tenantID, caseID)
	if err != nil {
		return nil, ErrForbidden
	}
	return c, nil
}

// caseMetadataPayloadOf builds a CaseMetadataPayload from the live
// Case's current mutable fields.
func caseMetadataPayloadOf(c *caselifecycle.Case) CaseMetadataPayload {
	md := make(map[string]string, len(c.Metadata))
	for k, v := range c.Metadata {
		md[k] = v
	}
	return CaseMetadataPayload{
		Title:      c.Title,
		Reference:  c.Reference,
		CategoryID: c.CategoryID,
		State:      c.State.String(),
		Metadata:   md,
	}
}

// SnapshotCaseMetadata records a new ArtifactCaseMetadata Snapshot
// capturing the case's current mutable fields (title, reference,
// category, state, metadata), attributed to the ctx actor. reason is a
// free-form change-attribution note (e.g. "manual edit", "signoff
// re-review"); label is an optional short name for the snapshot.
// Requires identity.PermEditCase.
func (s *Service) SnapshotCaseMetadata(ctx context.Context, tenantID, caseID uuid.UUID, reason, label string) (*Snapshot, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	c, err := s.checkCaseAccess(ctx, tenantID, caseID)
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		CaseID:              caseID,
		TenantID:            tenantID,
		ArtifactKind:        ArtifactCaseMetadata,
		ArtifactRevisionRef: strconv.Itoa(c.MetadataVersion),
		Payload:             caseMetadataPayloadOf(c),
		CreatedBy:           user.ID,
		Reason:              reason,
		Label:               label,
		CreatedAt:           s.now(),
	}
	if err := s.repo.Create(ctx, tenantID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// SnapshotTree records a new ArtifactTree Snapshot referencing rev by
// its RevisionNumber — a link to the existing packages/irac tree
// revision, not a copy of the tree itself. Requires
// identity.PermEditCase.
func (s *Service) SnapshotTree(ctx context.Context, tenantID, caseID uuid.UUID, rev irac.TreeRevision, reason, label string) (*Snapshot, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, caseID); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		CaseID:              caseID,
		TenantID:            tenantID,
		ArtifactKind:        ArtifactTree,
		ArtifactRevisionRef: strconv.Itoa(rev.RevisionNumber),
		CreatedBy:           user.ID,
		Reason:              reason,
		Label:               label,
		CreatedAt:           s.now(),
	}
	if err := s.repo.Create(ctx, tenantID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// SnapshotEvidence records a new ArtifactEvidence Snapshot referencing
// revisionRef — an upstream annotations.Annotation ID or evidence
// segment ID, when derivable, identifying what evidence-related change
// triggered this snapshot. revisionRef may be empty when no single
// upstream ID applies (e.g. a bulk ingestion). Requires
// identity.PermEditCase.
func (s *Service) SnapshotEvidence(ctx context.Context, tenantID, caseID uuid.UUID, revisionRef, reason, label string) (*Snapshot, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, caseID); err != nil {
		return nil, err
	}

	snap := &Snapshot{
		CaseID:              caseID,
		TenantID:            tenantID,
		ArtifactKind:        ArtifactEvidence,
		ArtifactRevisionRef: revisionRef,
		CreatedBy:           user.ID,
		Reason:              reason,
		Label:               label,
		CreatedAt:           s.now(),
	}
	if err := s.repo.Create(ctx, tenantID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// SnapshotOpinion records a new ArtifactOpinion Snapshot carrying a
// compact copy of op — the first versioning/history mechanism for
// packages/synthesisagent.Opinion, since no upstream package assigns
// Opinion output a revision ID of its own. Requires
// identity.PermEditCase.
func (s *Service) SnapshotOpinion(ctx context.Context, tenantID, caseID uuid.UUID, op synthesisagent.Opinion, reason, label string) (*Snapshot, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, caseID); err != nil {
		return nil, err
	}

	conclusions := make([]synthesisagent.TentativeConclusion, len(op.Conclusions))
	copy(conclusions, op.Conclusions)

	snap := &Snapshot{
		CaseID:       caseID,
		TenantID:     tenantID,
		ArtifactKind: ArtifactOpinion,
		Payload: OpinionPayload{
			CaseID:              op.CaseID,
			ConclusionCount:     len(conclusions),
			Conclusions:         conclusions,
			SkippedIssueNodeIDs: append([]string(nil), op.SkippedIssueNodeIDs...),
			GeneratedAt:         op.GeneratedAt,
		},
		CreatedBy: user.ID,
		Reason:    reason,
		Label:     label,
		CreatedAt: s.now(),
	}
	if err := s.repo.Create(ctx, tenantID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// Get returns the snapshot with the given id, scoped to tenantID.
// Requires identity.PermViewCase.
func (s *Service) Get(ctx context.Context, tenantID, id uuid.UUID) (*Snapshot, error) {
	if err := authorizeView(ctx); err != nil {
		return nil, err
	}
	snap, err := s.repo.Get(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, snap.CaseID); err != nil {
		return nil, err
	}
	return snap, nil
}

// History returns every snapshot for caseID visible to tenantID,
// optionally narrowed by filter, in chronological order — the version
// timeline. Requires identity.PermViewCase.
func (s *Service) History(ctx context.Context, tenantID, caseID uuid.UUID, filter SnapshotFilter) ([]*Snapshot, error) {
	if err := authorizeView(ctx); err != nil {
		return nil, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, caseID); err != nil {
		return nil, err
	}
	return s.repo.ListByCase(ctx, tenantID, caseID, filter)
}

// Diff returns the structured comparison between the snapshots
// identified by snapshotAID and snapshotBID, scoped to tenantID.
// Requires identity.PermViewCase.
func (s *Service) Diff(ctx context.Context, tenantID, snapshotAID, snapshotBID uuid.UUID) (Diff, error) {
	if err := authorizeView(ctx); err != nil {
		return Diff{}, err
	}
	a, err := s.repo.Get(ctx, tenantID, snapshotAID)
	if err != nil {
		return Diff{}, err
	}
	b, err := s.repo.Get(ctx, tenantID, snapshotBID)
	if err != nil {
		return Diff{}, err
	}
	if _, err := s.checkCaseAccess(ctx, tenantID, a.CaseID); err != nil {
		return Diff{}, err
	}
	return ComputeDiff(a, b)
}

// Restore reverts the case's live metadata to the state captured by the
// snapshot identified by snapshotID, then records a new, forward-only
// ArtifactCaseMetadata Snapshot documenting the restore itself
// (RestoredFromID pointing back at snapshotID) — history is never
// rewritten, only extended, per task 3. Only ArtifactCaseMetadata
// snapshots can be restored (ErrNotRestorable for any other
// ArtifactKind); tree/evidence/opinion artifacts are restored, if ever
// needed, by acting on their own upstream package (packages/irac,
// packages/treeassembly, packages/synthesisagent), not by this method.
//
// Requires identity.PermEditCase.
func (s *Service) Restore(ctx context.Context, tenantID, snapshotID uuid.UUID) (*Snapshot, error) {
	user, err := authorizeWrite(ctx)
	if err != nil {
		return nil, err
	}

	target, err := s.repo.Get(ctx, tenantID, snapshotID)
	if err != nil {
		return nil, err
	}
	if target.ArtifactKind != ArtifactCaseMetadata {
		return nil, ErrNotRestorable
	}
	payload, err := AsCaseMetadataPayload(target)
	if err != nil {
		return nil, err
	}

	c, err := s.checkCaseAccess(ctx, tenantID, target.CaseID)
	if err != nil {
		return nil, err
	}

	md := make(map[string]string, len(payload.Metadata))
	for k, v := range payload.Metadata {
		md[k] = v
	}
	c.Title = payload.Title
	c.Reference = payload.Reference
	c.CategoryID = payload.CategoryID
	c.State = caselifecycle.State(payload.State)
	c.Metadata = md
	c.MetadataVersion++
	c.UpdatedAt = s.now()

	if err := s.cases.Update(ctx, tenantID, c); err != nil {
		return nil, err
	}

	restoreID := target.ID
	snap := &Snapshot{
		CaseID:              c.ID,
		TenantID:            tenantID,
		ArtifactKind:        ArtifactCaseMetadata,
		ArtifactRevisionRef: strconv.Itoa(c.MetadataVersion),
		Payload:             caseMetadataPayloadOf(c),
		CreatedBy:           user.ID,
		Reason:              "restore to snapshot " + target.ID.String(),
		Label:               "Restore of " + target.Label,
		RestoredFromID:      &restoreID,
		CreatedAt:           s.now(),
	}
	if err := s.repo.Create(ctx, tenantID, snap); err != nil {
		return nil, err
	}
	return snap, nil
}
