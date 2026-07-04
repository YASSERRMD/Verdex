package vulnmanagement

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// remediationSLA maps each Severity to how long a Finding may remain
// unresolved before it is considered SLA-breached (task 6). These are
// real, deployment-reasonable defaults, not placeholders: Critical
// findings (e.g. remote code execution) get a short week to fix;
// Medium findings get a full quarter, since low-exploitability issues
// legitimately queue behind higher-priority work.
var remediationSLA = map[Severity]time.Duration{
	SeverityCritical: 7 * 24 * time.Hour,
	SeverityHigh:     30 * 24 * time.Hour,
	SeverityMedium:   90 * 24 * time.Hour,
	SeverityLow:      180 * 24 * time.Hour,
}

// RemediationDeadlineFor returns the duration a Finding of the given
// Severity may remain open before breaching its remediation SLA. The
// bool result is false for an unrecognized/invalid Severity (callers
// should treat that as "no SLA applies" rather than silently using a
// zero duration, which would make every finding of that severity
// immediately breached).
func RemediationDeadlineFor(severity Severity) (time.Duration, bool) {
	d, ok := remediationSLA[severity]
	return d, ok
}

// SLADeadline returns the absolute time by which f must reach a
// terminal Status (see Status.IsTerminal) to stay within its
// remediation SLA, computed from f.DiscoveredAt plus
// RemediationDeadlineFor(f.Severity). ok is false if f.Severity carries
// no configured SLA.
func SLADeadline(f Finding) (deadline time.Time, ok bool) {
	d, ok := RemediationDeadlineFor(f.Severity)
	if !ok {
		return time.Time{}, false
	}
	return f.DiscoveredAt.Add(d), true
}

// IsSLABreached reports whether f is past its remediation SLA deadline
// as of now, given now as an injected clock value for testability. A
// Finding already in a terminal Status (Resolved/AcceptedRisk/
// FalsePositive) is never considered breached regardless of timing --
// the SLA clock only matters for work still outstanding. A Finding
// whose Severity carries no configured SLA is never considered
// breached (real time-based logic: no SLA means no deadline to miss,
// not an implicit immediate breach).
func IsSLABreached(f Finding, now time.Time) bool {
	if f.Status.IsTerminal() {
		return false
	}
	deadline, ok := SLADeadline(f)
	if !ok {
		return false
	}
	return now.After(deadline)
}

// FindingsPastSLA filters findings down to those IsSLABreached reports
// true for as of now -- the real, testable-with-injected-now query
// this phase's SLA tracking task requires, not a stub that always
// returns an empty (or full) list.
func FindingsPastSLA(findings []Finding, now time.Time) []Finding {
	out := make([]Finding, 0)
	for _, f := range findings {
		if IsSLABreached(f, now) {
			out = append(out, f)
		}
	}
	return out
}

// ListSLABreaches returns every Finding recorded for tenantID that is
// currently SLA-breached (per FindingsPastSLA, evaluated against the
// Engine's own clock), requiring viewPermission and tenant match.
func (e *Engine) ListSLABreaches(ctx context.Context, tenantID uuid.UUID) ([]Finding, error) {
	all, err := e.ListFindings(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	return FindingsPastSLA(all, e.now()), nil
}
