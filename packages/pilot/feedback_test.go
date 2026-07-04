package pilot_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_SubmitFeedback_RequiresExistingCase(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	admin := adminUser(te.tenantID)

	_, err := te.engine.SubmitFeedback(ctxWithUser(admin), te.tenantID, pilot.FeedbackEntry{
		PilotCaseID: uuid.New(),
		Ratings: []pilot.DimensionRating{
			{Dimension: pilot.DimensionGrounding, Score: 0.5},
		},
		Trust: pilot.TrustModerate,
	})
	if !errors.Is(err, pilot.ErrCaseNotFound) {
		t.Fatalf("error = %v, want ErrCaseNotFound", err)
	}
}

func TestEngine_SubmitFeedback_ComputesOverallScore(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)

	entry := submitTestFeedback(t, te, pc.ID)
	const want = 0.85
	if got := entry.OverallScore(); !floatsClose(got, want) {
		t.Fatalf("OverallScore() = %v, want %v", got, want)
	}

	score, ok := entry.ScoreFor(pilot.DimensionGrounding)
	if !ok || score != 0.8 {
		t.Fatalf("ScoreFor(grounding) = (%v, %v), want (0.8, true)", score, ok)
	}
	if _, ok := entry.ScoreFor(pilot.DimensionCoherence); ok {
		t.Fatal("ScoreFor(coherence) should report false: no such rating was submitted")
	}
}

func TestEngine_SubmitFeedback_RejectsInvalidDimensionScore(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	_, err := te.engine.SubmitFeedback(ctxWithUser(admin), te.tenantID, pilot.FeedbackEntry{
		PilotCaseID: pc.ID,
		Ratings: []pilot.DimensionRating{
			{Dimension: pilot.DimensionGrounding, Score: 1.5}, // out of [0,1]
		},
		Trust: pilot.TrustModerate,
	})
	if !errors.Is(err, pilot.ErrInvalidFeedback) {
		t.Fatalf("error = %v, want ErrInvalidFeedback", err)
	}
}

func TestEngine_SubmitFeedback_RejectsInvalidTrust(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	_, err := te.engine.SubmitFeedback(ctxWithUser(admin), te.tenantID, pilot.FeedbackEntry{
		PilotCaseID: pc.ID,
		Ratings: []pilot.DimensionRating{
			{Dimension: pilot.DimensionGrounding, Score: 0.5},
		},
		Trust: pilot.TrustRating(99),
	})
	if !errors.Is(err, pilot.ErrInvalidFeedback) {
		t.Fatalf("error = %v, want ErrInvalidFeedback", err)
	}
}

func TestEngine_ListFeedbackForDeployment_JoinsThroughCases(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	pc1 := assignTestCase(t, te, d.ID)
	pc2 := assignTestCase(t, te, d.ID)
	submitTestFeedback(t, te, pc1.ID)
	submitTestFeedback(t, te, pc2.ID)
	submitTestFeedback(t, te, pc2.ID)

	list, err := te.engine.ListFeedbackForDeployment(ctxWithUser(admin), te.tenantID, d.ID)
	if err != nil {
		t.Fatalf("ListFeedbackForDeployment: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len(list) = %d, want 3", len(list))
	}
}
