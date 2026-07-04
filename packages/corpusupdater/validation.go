package corpusupdater

import (
	"strings"
	"time"
)

// maxPastWindow and maxFutureWindow bound how far an Amendment's
// EffectiveDate may plausibly sit from the moment it is validated.
// These are deliberately generous (a decade each way) -- the point is
// catching an obviously wrong value (a typo'd year, an
// accidentally-zero time.Time), not second-guessing a real long-lead
// legislative effective date.
const (
	maxPastWindow   = 10 * 365 * 24 * time.Hour
	maxFutureWindow = 10 * 365 * 24 * time.Hour
)

// TargetResolver resolves whether a rule/precedent ID actually exists
// in the named corpus, letting Validate check "target id resolvable"
// (task 6) without this package importing packages/statute or
// packages/precedent to do it. A nil TargetResolver skips the
// resolvability check entirely (existence is assumed) -- useful for
// unit tests of the structural checks alone; production callers should
// always supply one.
type TargetResolver func(corpus CorpusTarget, targetID string) bool

// Validate performs the structural checks on a required before
// Engine.StageAmendment accepts it (task 6): a recognized ChangeType,
// a non-empty Citation, a TargetID present when ChangeType requires
// one, a TargetID that resolves via resolve (when resolve is
// non-nil), and an EffectiveDate that isn't absurdly far in the past
// or future relative to now. Returns the first problem found, wrapped
// with the corpusupdater: Validate: prefix, or nil if a is acceptable.
func Validate(a Amendment, resolve TargetResolver, now time.Time) error {
	if !a.ChangeType.IsValid() {
		return wrapf("Validate", ErrInvalidChangeType)
	}
	if strings.TrimSpace(a.Citation) == "" {
		return wrapf("Validate", ErrMissingCitation)
	}
	if !a.TargetCorpus.IsValid() {
		return wrapf("Validate", ErrInvalidCorpusTarget)
	}

	requiresTarget := a.ChangeType == ChangeTypeAmend || a.ChangeType == ChangeTypeRepeal
	if requiresTarget && strings.TrimSpace(a.TargetID) == "" {
		return wrapf("Validate", ErrMissingTargetID)
	}
	if requiresTarget && resolve != nil && !resolve(a.TargetCorpus, a.TargetID) {
		return wrapf("Validate", ErrAmendmentNotFound)
	}
	// ChangeTypeAdd with a caller-supplied TargetID is still checked for
	// resolvability against the *absence* of a collision being
	// unnecessary here: Add legitimately creates a fresh ID, so no
	// resolver check applies to it.

	if a.EffectiveDate.IsZero() {
		return wrapf("Validate", ErrEffectiveDateOutOfRange)
	}
	if a.EffectiveDate.Before(now.Add(-maxPastWindow)) {
		return wrapf("Validate", ErrEffectiveDateOutOfRange)
	}
	if a.EffectiveDate.After(now.Add(maxFutureWindow)) {
		return wrapf("Validate", ErrEffectiveDateOutOfRange)
	}

	return nil
}
