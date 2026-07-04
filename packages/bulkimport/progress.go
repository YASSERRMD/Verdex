package bulkimport

import "time"

// Progress is a point-in-time snapshot of an ImportJob's completion
// state (task 6): processed/total/failed/skipped counts, a computed
// percent, and the timestamps needed to estimate how much longer the
// job will take.
type Progress struct {
	// JobID identifies the job this snapshot describes.
	JobID string `json:"job_id"`

	// Total is the expected total record count (ImportJob.TotalRecords).
	// Zero means the total is not known ahead of time.
	Total int `json:"total"`

	// Processed is the count of records processed so far (imported +
	// skipped + rejected).
	Processed int `json:"processed"`

	// Imported, Skipped, and Failed break Processed down by outcome.
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
	Failed   int `json:"failed"`

	// PercentComplete is Processed/Total*100, clamped to [0, 100].
	// Zero when Total is zero (indeterminate) rather than
	// misleadingly reporting 0% or 100%.
	PercentComplete float64 `json:"percent_complete"`

	// StartedAt is when the job first began processing. Zero value if
	// it has not started yet.
	StartedAt time.Time `json:"started_at,omitempty"`

	// LastUpdatedAt is when this snapshot's underlying counts were
	// last updated -- the timestamp an ETA estimate should be computed
	// relative to.
	LastUpdatedAt time.Time `json:"last_updated_at"`

	// Status is the job's current Status at the time of this snapshot.
	Status Status `json:"status"`
}

// EstimatedTimeRemaining returns a rough ETA for the job to finish,
// linearly extrapolating from the processing rate observed between
// StartedAt and LastUpdatedAt. Returns false if there is not enough
// data to estimate (no StartedAt, zero Total, or zero Processed).
func (p Progress) EstimatedTimeRemaining() (time.Duration, bool) {
	if p.StartedAt.IsZero() || p.Total <= 0 || p.Processed <= 0 {
		return 0, false
	}
	elapsed := p.LastUpdatedAt.Sub(p.StartedAt)
	if elapsed <= 0 {
		return 0, false
	}
	remaining := p.Total - p.Processed
	if remaining <= 0 {
		return 0, true
	}
	perRecord := elapsed / time.Duration(p.Processed)
	return perRecord * time.Duration(remaining), true
}

// progressFromJob builds a Progress snapshot from j at instant now.
func progressFromJob(j ImportJob, now time.Time) Progress {
	p := Progress{
		JobID:         j.ID.String(),
		Total:         j.TotalRecords,
		Processed:     j.ProcessedRecords,
		Imported:      j.ImportedRecords,
		Skipped:       j.SkippedRecords,
		Failed:        j.FailedRecords,
		LastUpdatedAt: now,
		Status:        j.Status,
	}
	if j.StartedAt != nil {
		p.StartedAt = *j.StartedAt
	}
	if j.TotalRecords > 0 {
		pct := float64(j.ProcessedRecords) / float64(j.TotalRecords) * 100
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		p.PercentComplete = pct
	}
	return p
}
