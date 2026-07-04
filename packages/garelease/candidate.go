package garelease

import (
	"context"

	"github.com/google/uuid"
)

// FreezeReleaseCandidate is task 4's "freeze and tag release candidate"
// -- the software-side half of it. It records a ReleaseCandidate for
// version (a validated semantic version) and commitSHA, snapshotting
// readiness as the ReleaseReadiness the caller most recently computed
// via CheckReadiness. Requires managePermission.
//
// FreezeReleaseCandidate returns ErrReadinessNotReady and refuses to
// persist anything if !readiness.Ready -- freezing an unready candidate
// is blocked outright, not merely logged or warned about (see
// doc/ga-release.md and this package's own tests for the explicit
// proof this blocks). This method does not itself call CheckReadiness;
// the caller is expected to have just computed readiness and passes it
// in, so the exact snapshot that justified freezing is the exact
// snapshot persisted alongside the candidate -- no risk of a
// time-of-check/time-of-use gap between evaluating readiness and
// freezing against it.
//
// This method models the software-side "freeze" record only. It does
// NOT and must not shell out to `git tag` -- seeing doc.go's "What this
// package does NOT do" section for the full boundary explanation. The
// actual git tag is a separate, deliberate step a human or CI job
// performs against the frozen commitSHA, informed by (but not
// triggered by) this record's existence.
func (e *Engine) FreezeReleaseCandidate(ctx context.Context, version, commitSHA string, readiness ReleaseReadiness) (ReleaseCandidate, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordCandidateFreeze(ctx, actorFromCtx(ctx), ReleaseCandidate{Version: version, CommitSHA: commitSHA}, err)
		}
		return ReleaseCandidate{}, err
	}

	if !readiness.Ready {
		wrapped := wrapf("FreezeReleaseCandidate", ErrReadinessNotReady)
		if e.audit != nil {
			_, _ = e.audit.RecordCandidateFreeze(ctx, user.ID, ReleaseCandidate{Version: version, CommitSHA: commitSHA, Readiness: readiness}, wrapped)
		}
		return ReleaseCandidate{}, wrapped
	}

	candidate := ReleaseCandidate{
		ID:        uuid.New(),
		Version:   version,
		CommitSHA: commitSHA,
		Readiness: readiness,
		FrozenBy:  user.ID,
		FrozenAt:  e.now(),
	}

	if err := candidate.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordCandidateFreeze(ctx, user.ID, candidate, err)
		}
		return ReleaseCandidate{}, err
	}

	if err := e.candidates.Create(ctx, &candidate); err != nil {
		wrapped := wrapf("FreezeReleaseCandidate", err)
		if e.audit != nil {
			_, _ = e.audit.RecordCandidateFreeze(ctx, user.ID, candidate, wrapped)
		}
		return ReleaseCandidate{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordCandidateFreeze(ctx, user.ID, candidate, nil)
	}
	return candidate, nil
}

// GetReleaseCandidate returns the ReleaseCandidate identified by id,
// requiring viewPermission.
func (e *Engine) GetReleaseCandidate(ctx context.Context, id uuid.UUID) (ReleaseCandidate, error) {
	if err := authorizeView(ctx); err != nil {
		return ReleaseCandidate{}, err
	}
	c, err := e.candidates.Get(ctx, id)
	if err != nil {
		return ReleaseCandidate{}, wrapf("GetReleaseCandidate", err)
	}
	return *c, nil
}

// ListReleaseCandidates returns every recorded ReleaseCandidate,
// requiring viewPermission.
func (e *Engine) ListReleaseCandidates(ctx context.Context) ([]ReleaseCandidate, error) {
	if err := authorizeView(ctx); err != nil {
		return nil, err
	}
	list, err := e.candidates.List(ctx)
	if err != nil {
		return nil, wrapf("ListReleaseCandidates", err)
	}
	return list, nil
}

// CutRelease is task 8's "cut GA release tag" -- the software-side
// half of it. It promotes the ReleaseCandidate identified by
// candidateID to a Release, requiring managePermission and that:
//
//  1. candidateID resolves to an existing, previously-frozen
//     ReleaseCandidate (ErrCandidateNotFound otherwise -- since every
//     ReleaseCandidate this package's own repository holds was, by
//     construction via FreezeReleaseCandidate, already frozen with a
//     Ready snapshot, "not found" is the only realistic "not frozen"
//     case through this package's own API; ErrCandidateNotFrozen is
//     reserved as defense-in-depth against a hand-built repository
//     implementation that stored an unfrozen record).
//  2. the candidate's stored Readiness snapshot is STILL Ready
//     (ErrCandidateReadinessStale otherwise) -- re-checked at cut time,
//     not merely trusted from freeze time, in case the caller wants to
//     re-verify before the irreversible GA cut.
//  3. the candidate has not already been cut to a Release
//     (ErrAlreadyReleased otherwise) -- CutRelease is not
//     idempotent-by-retry.
//
// This method models the software-side "cut" record only. It does NOT
// and must not shell out to `git tag` -- see doc.go's "What this
// package does NOT do" section. The actual git tag against the
// candidate's CommitSHA is a separate, deliberate step the orchestrator
// performs after this record (and the PR introducing it) has merged.
func (e *Engine) CutRelease(ctx context.Context, candidateID uuid.UUID) (Release, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return Release{}, err
	}

	candidate, err := e.candidates.Get(ctx, candidateID)
	if err != nil {
		wrapped := wrapf("CutRelease", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReleaseCut(ctx, user.ID, Release{CandidateID: candidateID}, wrapped)
		}
		return Release{}, wrapped
	}

	if !candidate.Readiness.Ready {
		wrapped := wrapf("CutRelease", ErrCandidateReadinessStale)
		if e.audit != nil {
			_, _ = e.audit.RecordReleaseCut(ctx, user.ID, Release{CandidateID: candidateID, Version: candidate.Version}, wrapped)
		}
		return Release{}, wrapped
	}

	if existing, err := e.releases.GetByCandidateID(ctx, candidateID); err == nil && existing != nil {
		wrapped := wrapf("CutRelease", ErrAlreadyReleased)
		if e.audit != nil {
			_, _ = e.audit.RecordReleaseCut(ctx, user.ID, Release{CandidateID: candidateID, Version: candidate.Version}, wrapped)
		}
		return Release{}, wrapped
	}

	release := Release{
		ID:          uuid.New(),
		CandidateID: candidate.ID,
		Version:     candidate.Version,
		CommitSHA:   candidate.CommitSHA,
		CutBy:       user.ID,
		CutAt:       e.now(),
	}

	if err := release.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordReleaseCut(ctx, user.ID, release, err)
		}
		return Release{}, err
	}

	if err := e.releases.Create(ctx, &release); err != nil {
		wrapped := wrapf("CutRelease", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReleaseCut(ctx, user.ID, release, wrapped)
		}
		return Release{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordReleaseCut(ctx, user.ID, release, nil)
	}
	return release, nil
}

// GetRelease returns the Release identified by id, requiring
// viewPermission.
func (e *Engine) GetRelease(ctx context.Context, id uuid.UUID) (Release, error) {
	if err := authorizeView(ctx); err != nil {
		return Release{}, err
	}
	r, err := e.releases.Get(ctx, id)
	if err != nil {
		return Release{}, wrapf("GetRelease", err)
	}
	return *r, nil
}

// ListReleases returns every recorded Release, requiring viewPermission.
func (e *Engine) ListReleases(ctx context.Context) ([]Release, error) {
	if err := authorizeView(ctx); err != nil {
		return nil, err
	}
	list, err := e.releases.List(ctx)
	if err != nil {
		return nil, wrapf("ListReleases", err)
	}
	return list, nil
}
