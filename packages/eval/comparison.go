package eval

import (
	"sort"
	"time"
)

// ComparisonResult summarises the head-to-head comparison of two EvalResults
// on the same task.
type ComparisonResult struct {
	// TaskID is the task both results belong to.
	TaskID string

	// WinnerID is the ProviderID of the higher-scoring result.
	// Empty string means the scores are equal.
	WinnerID string

	// ScoreDiff is a.Score - b.Score.  Positive values favour a.
	ScoreDiff float64

	// LatencyDiffMs is a.Latency - b.Latency in milliseconds.
	// Positive values mean a was slower.
	LatencyDiffMs int64

	// Details maps RubricCriteria.Name to (a score - b score) for each
	// criterion that both results share.
	Details map[string]float64
}

// SideBySide returns a ComparisonResult for two EvalResults that must belong
// to the same task (same TaskID).  If their TaskIDs differ the comparison
// proceeds but WinnerID is set to "mismatch".
func SideBySide(a, b EvalResult) ComparisonResult {
	c := ComparisonResult{
		TaskID:        a.TaskID,
		ScoreDiff:     a.Score - b.Score,
		LatencyDiffMs: a.Latency.Milliseconds() - b.Latency.Milliseconds(),
		Details:       make(map[string]float64),
	}

	if a.TaskID != b.TaskID {
		c.WinnerID = "mismatch"
		return c
	}

	switch {
	case a.Score > b.Score:
		c.WinnerID = a.ProviderID
	case b.Score > a.Score:
		c.WinnerID = b.ProviderID
	default:
		c.WinnerID = ""
	}

	// Per-criterion diffs.
	for name, scoreA := range a.Rubric {
		if scoreB, ok := b.Rubric[name]; ok {
			c.Details[name] = scoreA - scoreB
		}
	}

	return c
}

// GenerateReport aggregates a flat slice of EvalResults into an EvalReport.
//
// It computes per-provider average scores and latency percentiles (P50, P95).
// The GoldenVersion field is left empty; callers should set it from the
// GoldenSet.Version used for the run.
func GenerateReport(results []EvalResult) EvalReport {
	report := EvalReport{
		Results: results,
		Summary: make(map[string]ProviderSummary),
		RunAt:   time.Now().UTC(),
	}

	// Group results by provider.
	byProvider := make(map[string][]EvalResult)
	for _, r := range results {
		byProvider[r.ProviderID] = append(byProvider[r.ProviderID], r)
	}

	for pid, pResults := range byProvider {
		var scoreSum float64
		latencies := make([]int64, 0, len(pResults))

		for _, r := range pResults {
			scoreSum += r.Score
			latencies = append(latencies, r.Latency.Milliseconds())
		}

		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

		n := len(latencies)
		p50 := percentile(latencies, 50)
		p95 := percentile(latencies, 95)

		_ = n // used implicitly via len(pResults)

		report.Summary[pid] = ProviderSummary{
			ProviderID: pid,
			AvgScore:   scoreSum / float64(len(pResults)),
			P50Latency: time.Duration(p50) * time.Millisecond,
			P95Latency: time.Duration(p95) * time.Millisecond,
			TotalCost:  0,
		}
	}

	return report
}

// RankProviders returns provider IDs sorted by descending average score.
// Ties are broken alphabetically by ProviderID.
func RankProviders(report EvalReport) []string {
	type entry struct {
		id    string
		score float64
	}
	entries := make([]entry, 0, len(report.Summary))
	for pid, s := range report.Summary {
		entries = append(entries, entry{id: pid, score: s.AvgScore})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].id < entries[j].id
	})
	ranked := make([]string, len(entries))
	for i, e := range entries {
		ranked[i] = e.id
	}
	return ranked
}

// percentile returns the p-th percentile value from a sorted slice of int64.
// Returns 0 if the slice is empty.
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	// Nearest-rank method.
	idx := int(float64(p)/100.0*float64(len(sorted)-1) + 0.5)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
