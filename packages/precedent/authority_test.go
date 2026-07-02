package precedent

import (
	"context"
	"testing"
	"time"
)

func TestRecencyScore_MoreRecentScoresHigher(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	old := time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)

	recentScore := RecencyScore(recent, asOf)
	oldScore := RecencyScore(old, asOf)

	if recentScore <= oldScore {
		t.Errorf("RecencyScore(recent) = %v, want > RecencyScore(old) = %v", recentScore, oldScore)
	}
	if recentScore <= 0 || recentScore > 1 {
		t.Errorf("RecencyScore(recent) = %v, want in (0, 1]", recentScore)
	}
}

func TestRecencyScore_ZeroDateIsNeutral(t *testing.T) {
	score := RecencyScore(time.Time{}, time.Now())
	if score != 0.5 {
		t.Errorf("RecencyScore(zero date) = %v, want 0.5", score)
	}
}

func TestRecencyScore_HalfLifeDecay(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	decided := asOf.AddDate(-int(recencyHalfLifeYears), 0, 0)
	score := RecencyScore(decided, asOf)
	if score < 0.45 || score > 0.55 {
		t.Errorf("RecencyScore at one half-life = %v, want ~0.5", score)
	}
}

func TestAuthorityScoreAsOf_CourtHierarchyOrdering(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	decided := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	base := func(level CourtLevel) HierarchyRule {
		rule := syntheticPrecedentRule(t)
		rule.Source.DecidedDate = decided
		tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})
		hierarchy := ApplyCourtHierarchy(tagged, level)
		return hierarchy[0]
	}

	supreme := AuthorityScoreAsOf(base(CourtSupreme), asOf)
	appellate := AuthorityScoreAsOf(base(CourtAppellate), asOf)
	trial := AuthorityScoreAsOf(base(CourtTrial), asOf)

	if supreme <= appellate {
		t.Errorf("supreme AuthorityScore = %v, want > appellate = %v", supreme, appellate)
	}
	if appellate <= trial {
		t.Errorf("appellate AuthorityScore = %v, want > trial = %v", appellate, trial)
	}
}

func TestAuthorityScoreAsOf_RecencyRespondsToDate(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	newer := func(decided time.Time) HierarchyRule {
		rule := syntheticPrecedentRule(t)
		rule.Source.DecidedDate = decided
		tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})
		hierarchy := ApplyCourtHierarchy(tagged, CourtAppellate)
		return hierarchy[0]
	}

	recentScore := AuthorityScoreAsOf(newer(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)), asOf)
	oldScore := AuthorityScoreAsOf(newer(time.Date(1960, 1, 1, 0, 0, 0, 0, time.UTC)), asOf)

	if recentScore <= oldScore {
		t.Errorf("recentScore = %v, want > oldScore = %v", recentScore, oldScore)
	}
}

func TestAuthorityScoreAsOf_Bounds(t *testing.T) {
	asOf := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rule := syntheticPrecedentRule(t)
	rule.Source.DecidedDate = asOf
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})
	hierarchy := ApplyCourtHierarchy(tagged, CourtSupreme)

	score := AuthorityScoreAsOf(hierarchy[0], asOf)
	if score < 0 || score > 1 {
		t.Errorf("AuthorityScoreAsOf() = %v, want in [0, 1]", score)
	}
}

func TestScorePrecedents(t *testing.T) {
	rule := syntheticPrecedentRule(t)
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})
	hierarchy := ApplyCourtHierarchy(tagged, "")
	embedded, err := EmbedPrecedents(context.Background(), &fakeEmbeddingService{}, hierarchy, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedPrecedents() error = %v", err)
	}

	scored := ScorePrecedents(embedded, time.Now())
	if len(scored) != 1 {
		t.Fatalf("len(scored) = %d, want 1", len(scored))
	}
	if scored[0].Authority <= 0 {
		t.Errorf("Authority = %v, want > 0", scored[0].Authority)
	}
}
