package garelease_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/garelease"
)

func TestFreezeReleaseCandidate_RequiresPermission(t *testing.T) {
	engine := newTestEngine(t)

	_, err := engine.FreezeReleaseCandidate(context.Background(), "1.100.0", strings.Repeat("a", 40), readyReadiness())
	if !errors.Is(err, garelease.ErrUnauthenticated) {
		t.Fatalf("FreezeReleaseCandidate with no actor = %v, want ErrUnauthenticated", err)
	}

	ctx := ctxWithUser(auditorUser())
	_, err = engine.FreezeReleaseCandidate(ctx, "1.100.0", strings.Repeat("a", 40), readyReadiness())
	if !errors.Is(err, garelease.ErrForbidden) {
		t.Fatalf("FreezeReleaseCandidate as viewer-only = %v, want ErrForbidden", err)
	}
}

// TestFreezeReleaseCandidate_RefusesUnreadyCandidate is the explicit
// test this phase's brief calls for: FreezeReleaseCandidate must
// return an error and persist nothing when readiness.Ready is false.
func TestFreezeReleaseCandidate_RefusesUnreadyCandidate(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	_, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", strings.Repeat("a", 40), notReadyReadiness())
	if !errors.Is(err, garelease.ErrReadinessNotReady) {
		t.Fatalf("FreezeReleaseCandidate with a not-Ready snapshot = %v, want ErrReadinessNotReady", err)
	}

	// Confirm nothing was persisted: listing candidates afterward must
	// be empty.
	candidates, err := engine.ListReleaseCandidates(ctx)
	if err != nil {
		t.Fatalf("ListReleaseCandidates: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("ListReleaseCandidates after a refused freeze = %d entries, want 0 (nothing should have been persisted)", len(candidates))
	}
}

func TestFreezeReleaseCandidate_SucceedsWhenReady(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	commitSHA := strings.Repeat("a", 40)
	candidate, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", commitSHA, readyReadiness())
	if err != nil {
		t.Fatalf("FreezeReleaseCandidate: %v", err)
	}
	if candidate.Version != "1.100.0" {
		t.Errorf("candidate.Version = %q, want 1.100.0", candidate.Version)
	}
	if candidate.CommitSHA != commitSHA {
		t.Errorf("candidate.CommitSHA = %q, want %q", candidate.CommitSHA, commitSHA)
	}
	if !candidate.Readiness.Ready {
		t.Errorf("candidate.Readiness.Ready = false, want true")
	}
	if candidate.FrozenAt.IsZero() {
		t.Errorf("candidate.FrozenAt is zero")
	}

	fetched, err := engine.GetReleaseCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("GetReleaseCandidate: %v", err)
	}
	if fetched.ID != candidate.ID {
		t.Errorf("fetched.ID = %v, want %v", fetched.ID, candidate.ID)
	}
}

func TestFreezeReleaseCandidate_InvalidVersionRejected(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	_, err := engine.FreezeReleaseCandidate(ctx, "not-a-version", strings.Repeat("a", 40), readyReadiness())
	if !errors.Is(err, garelease.ErrInvalidVersion) {
		t.Fatalf("FreezeReleaseCandidate with an invalid version = %v, want ErrInvalidVersion", err)
	}
}

func TestFreezeReleaseCandidate_InvalidCommitSHARejected(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	_, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", "not-a-sha", readyReadiness())
	if !errors.Is(err, garelease.ErrInvalidCommitSHA) {
		t.Fatalf("FreezeReleaseCandidate with an invalid commit sha = %v, want ErrInvalidCommitSHA", err)
	}
}

func TestCutRelease_RequiresPermission(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	candidate, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", strings.Repeat("a", 40), readyReadiness())
	if err != nil {
		t.Fatalf("FreezeReleaseCandidate: %v", err)
	}

	_, err = engine.CutRelease(context.Background(), candidate.ID)
	if !errors.Is(err, garelease.ErrUnauthenticated) {
		t.Fatalf("CutRelease with no actor = %v, want ErrUnauthenticated", err)
	}

	viewerCtx := ctxWithUser(auditorUser())
	_, err = engine.CutRelease(viewerCtx, candidate.ID)
	if !errors.Is(err, garelease.ErrForbidden) {
		t.Fatalf("CutRelease as viewer-only = %v, want ErrForbidden", err)
	}
}

func TestCutRelease_RequiresFrozenReadyCandidate(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	_, err := engine.CutRelease(ctx, mustUUID())
	if !errors.Is(err, garelease.ErrCandidateNotFound) {
		t.Fatalf("CutRelease against a nonexistent candidate = %v, want ErrCandidateNotFound", err)
	}
}

func TestCutRelease_SucceedsForFrozenReadyCandidate(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	commitSHA := strings.Repeat("c", 40)
	candidate, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", commitSHA, readyReadiness())
	if err != nil {
		t.Fatalf("FreezeReleaseCandidate: %v", err)
	}

	release, err := engine.CutRelease(ctx, candidate.ID)
	if err != nil {
		t.Fatalf("CutRelease: %v", err)
	}
	if release.CandidateID != candidate.ID {
		t.Errorf("release.CandidateID = %v, want %v", release.CandidateID, candidate.ID)
	}
	if release.Version != candidate.Version {
		t.Errorf("release.Version = %q, want %q", release.Version, candidate.Version)
	}
	if release.CommitSHA != commitSHA {
		t.Errorf("release.CommitSHA = %q, want %q", release.CommitSHA, commitSHA)
	}

	fetched, err := engine.GetRelease(ctx, release.ID)
	if err != nil {
		t.Fatalf("GetRelease: %v", err)
	}
	if fetched.ID != release.ID {
		t.Errorf("fetched.ID = %v, want %v", fetched.ID, release.ID)
	}
}

func TestCutRelease_RefusesDoubleCut(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	candidate, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", strings.Repeat("d", 40), readyReadiness())
	if err != nil {
		t.Fatalf("FreezeReleaseCandidate: %v", err)
	}
	if _, err := engine.CutRelease(ctx, candidate.ID); err != nil {
		t.Fatalf("first CutRelease: %v", err)
	}

	_, err = engine.CutRelease(ctx, candidate.ID)
	if !errors.Is(err, garelease.ErrAlreadyReleased) {
		t.Fatalf("second CutRelease on the same candidate = %v, want ErrAlreadyReleased", err)
	}
}

func TestListReleaseCandidatesAndReleases(t *testing.T) {
	engine := newTestEngine(t)
	ctx := ctxWithUser(adminUser())

	c1, err := engine.FreezeReleaseCandidate(ctx, "1.100.0", strings.Repeat("1", 40), readyReadiness())
	if err != nil {
		t.Fatalf("FreezeReleaseCandidate 1: %v", err)
	}
	if _, err := engine.FreezeReleaseCandidate(ctx, "1.100.1", strings.Repeat("2", 40), readyReadiness()); err != nil {
		t.Fatalf("FreezeReleaseCandidate 2: %v", err)
	}

	candidates, err := engine.ListReleaseCandidates(ctx)
	if err != nil {
		t.Fatalf("ListReleaseCandidates: %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(ListReleaseCandidates()) = %d, want 2", len(candidates))
	}

	if _, err := engine.CutRelease(ctx, c1.ID); err != nil {
		t.Fatalf("CutRelease: %v", err)
	}
	releases, err := engine.ListReleases(ctx)
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("len(ListReleases()) = %d, want 1", len(releases))
	}
}
