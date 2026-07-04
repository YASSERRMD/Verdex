package garelease

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
)

// Stable "<noun>.<verb>" action verb-phrases recorded for every Event
// this package appends, following the convention documented on
// observability.AuditEvent.Action and already used by
// packages/compliance/audit.go and packages/vulnmanagement/audit.go.
const (
	auditActionReadinessCheck   = "garelease.readiness_check"
	auditActionCandidateFreeze  = "garelease.candidate_freeze"
	auditActionReleaseCut       = "garelease.release_cut"
	auditActionPostReleaseCheck = "garelease.post_release_check"
)

// systemActor is the Actor label recorded for events this package
// appends on behalf of the requesting caller with no resolvable ctx
// actor, mirroring packages/compliance and packages/vulnmanagement's
// systemActor idiom.
const systemActor = "system:garelease"

// platformScope is the TenantID recorded on every Event this package's
// AuditSink appends. ReleaseCandidate/Release/readiness snapshots are
// platform-global, not tenant-scoped (see types.go), so there is no
// real per-tenant scope to record. packages/auditlog.Store.Append hard
// -rejects a uuid.Nil TenantID (ErrEmptyTenantID), so this package uses
// a fixed, well-known, non-nil UUID (deterministically derived via
// uuid.NewSHA1 over a stable namespace+name, so it is identical across
// every process and restart, never randomly regenerated) as the
// platform-wide scope every event in this package's trail is recorded
// under -- distinguishable from any real tenant ID (which
// packages/tenancy always mints via uuid.New(), never
// uuid.NewSHA1(fixed namespace, ...)) by construction, and stable
// enough that ReleaseActivity's own Query(ctx, platformScope, ...) call
// always finds this package's own history.
var platformScope = uuid.NewSHA1(uuid.NameSpaceOID, []byte("verdex:garelease:platform"))

// AuditSink records every readiness check, candidate freeze, release
// cut, and post-release checklist run via packages/auditlog.Store,
// reusing the existing hash-chained, queryable audit trail rather than
// a second table -- exactly the composition pattern
// packages/compliance's and packages/vulnmanagement's own AuditSink
// established.
type AuditSink struct {
	store *auditlog.Store
}

// NewAuditSink builds an AuditSink backed by store. Returns
// ErrNilAuditSink if store is nil.
func NewAuditSink(store *auditlog.Store) (*AuditSink, error) {
	if store == nil {
		return nil, ErrNilAuditSink
	}
	return &AuditSink{store: store}, nil
}

// actorFor returns actorUserID.String() if non-nil, else systemActor.
func actorFor(actorUserID uuid.UUID) string {
	if actorUserID == uuid.Nil {
		return systemActor
	}
	return actorUserID.String()
}

// actorFromCtx resolves the actor's user ID from ctx if present,
// falling back to uuid.Nil (which actorFor renders as systemActor) when
// ctx carries no authenticated user -- used by the audit-on-failure
// paths, which must still record an event even when authorizeManage
// itself failed (e.g. ErrUnauthenticated).
func actorFromCtx(ctx context.Context) uuid.UUID {
	user, err := authorizeActor(ctx)
	if err != nil {
		return uuid.Nil
	}
	return user.ID
}

// RecordReadinessCheck appends an Event describing a CheckReadiness
// evaluation.
func (s *AuditSink) RecordReadinessCheck(ctx context.Context, actorUserID uuid.UUID, readiness ReleaseReadiness, checkErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "evaluated"
	detail := fmt.Sprintf("ready=%v checks=%d failed=%d", readiness.Ready, len(readiness.Checks), len(readiness.FailedChecks()))
	if checkErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, checkErr.Error())
	}

	ev := auditlog.Event{
		TenantID: platformScope,
		Kind:     auditlog.KindSystem,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionReadinessCheck
	ev.Target = "release_readiness"
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordReadinessCheck", err)
	}
	return appended, nil
}

// RecordCandidateFreeze appends an Event describing a
// FreezeReleaseCandidate call.
func (s *AuditSink) RecordCandidateFreeze(ctx context.Context, actorUserID uuid.UUID, candidate ReleaseCandidate, freezeErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "frozen"
	detail := fmt.Sprintf("version=%s commit=%s ready=%v", candidate.Version, candidate.CommitSHA, candidate.Readiness.Ready)
	if freezeErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, freezeErr.Error())
	}

	ev := auditlog.Event{
		TenantID: platformScope,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionCandidateFreeze
	ev.Target = candidate.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordCandidateFreeze", err)
	}
	return appended, nil
}

// RecordReleaseCut appends an Event describing a CutRelease call.
func (s *AuditSink) RecordReleaseCut(ctx context.Context, actorUserID uuid.UUID, release Release, cutErr error) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "cut"
	detail := fmt.Sprintf("candidate=%s version=%s commit=%s", release.CandidateID, release.Version, release.CommitSHA)
	if cutErr != nil {
		outcome = "denied"
		detail = fmt.Sprintf("%s error=%s", detail, cutErr.Error())
	}

	ev := auditlog.Event{
		TenantID: platformScope,
		Kind:     auditlog.KindAdmin,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionReleaseCut
	ev.Target = release.ID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordReleaseCut", err)
	}
	return appended, nil
}

// RecordPostReleaseCheck appends an Event describing a
// RunPostReleaseChecklist call.
func (s *AuditSink) RecordPostReleaseCheck(ctx context.Context, actorUserID uuid.UUID, releaseID uuid.UUID, report PostReleaseReport) (auditlog.Event, error) {
	if s == nil || s.store == nil {
		return auditlog.Event{}, ErrNilAuditSink
	}
	outcome := "passed"
	if !report.Passed() {
		outcome = "failed"
	}
	detail := fmt.Sprintf("checks=%d failed=%d", len(report.Results), len(report.Failures()))

	ev := auditlog.Event{
		TenantID: platformScope,
		Kind:     auditlog.KindSystem,
		Detail:   detail,
	}
	ev.Actor = actorFor(actorUserID)
	ev.Action = auditActionPostReleaseCheck
	ev.Target = releaseID.String()
	ev.Outcome = outcome

	appended, err := s.store.Append(ctx, ev)
	if err != nil {
		return auditlog.Event{}, wrapf("RecordPostReleaseCheck", err)
	}
	return appended, nil
}

// Querying this package's own recorded activity back
//
// This package deliberately does not expose a ReleaseActivity-style
// read wrapper the way packages/compliance and packages/privacy do:
// packages/auditlog.Store.Query requires the authenticated caller's
// identity.User.TenantID to equal exactly the tenantID being queried
// (requireMatchingUserTenant), and platformScope is not a real tenant
// any identity.User is ever provisioned under in the ordinary case. A
// deployment that wants to review this package's platform-wide trail
// queries packages/auditlog.Store.Query directly
// (auditlog.Filter{Action: "garelease.readiness_check"} etc.) using a
// platform-operator identity.User explicitly provisioned with
// TenantID == platformScope -- a deliberate, narrow escape hatch a
// deployment operator sets up once, not a gap this package's own API
// papers over silently.
