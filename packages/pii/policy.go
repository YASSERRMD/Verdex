package pii

import (
	"context"
	"fmt"
)

// StoragePolicy configures how StorageGuard enforces PII handling at a
// storage boundary: which categories are permitted to reach storage at all,
// and which redaction mode must be applied to categories that are permitted
// only in redacted form.
type StoragePolicy struct {
	// Detector finds PII in text before it is considered "at rest". Must be
	// non-nil.
	Detector Detector

	// Jurisdiction rules, applied (if non-nil) to resolve required
	// redaction modes per category for JurisdictionCode before the
	// package-wide Mode/ModeByCategory settings are consulted.
	JurisdictionRules *JurisdictionPIIRules

	// JurisdictionCode is the jurisdiction code to evaluate JurisdictionRules
	// under. Ignored if JurisdictionRules is nil.
	JurisdictionCode string

	// RejectCategories lists categories that must never reach storage, in
	// any form (redacted or not): if any match of one of these categories
	// is found, WriteGuarded returns ErrPolicyViolation and no write is
	// permitted. Use this for categories a jurisdiction or product
	// requirement says must never be persisted, even redacted (e.g. to
	// keep the fact that a financial identifier was ever present out of
	// storage entirely).
	RejectCategories map[PIICategory]bool

	// Mode is the default redaction mode applied to matches that are
	// permitted to reach storage (i.e. not in RejectCategories).
	Mode RedactionMode

	// ModeByCategory optionally overrides Mode per category, exactly like
	// Redactor.ModeByCategory.
	ModeByCategory map[PIICategory]RedactionMode

	// Pseudonyms is required when Mode (or any override) is
	// ModePseudonymize.
	Pseudonyms *PseudonymMap

	// AuditSink receives an audit event for every guarded write. If nil,
	// NoOpAuditSink is used.
	AuditSink AuditSink

	// Actor identifies the caller for audit purposes.
	Actor string
}

// StorageGuard wraps a write path and enforces a StoragePolicy over text
// before it is considered "at rest": it detects PII, rejects the write
// outright if any match falls in a rejected category, and otherwise redacts
// every remaining match according to policy before returning the sanitized
// text for the caller to actually persist.
//
// StorageGuard performs no actual storage I/O -- it is a boundary
// enforcement function that a caller places immediately before its own
// database/file/queue write.
type StorageGuard struct {
	Policy StoragePolicy
}

// NewStorageGuard constructs a StorageGuard for policy.
func NewStorageGuard(policy StoragePolicy) *StorageGuard {
	return &StorageGuard{Policy: policy}
}

// GuardedWriteResult is the outcome of a WriteGuarded call.
type GuardedWriteResult struct {
	// Text is the sanitized text that is safe to persist.
	Text string

	// Matches is every PIIMatch detected in the input, classified.
	Matches []PIIMatch

	// Redaction is the full redaction record from applying policy's modes.
	Redaction RedactionResult
}

// WriteGuarded evaluates text against the StorageGuard's policy and returns
// sanitized text safe to pass to the caller's actual write path.
//
// It returns ErrPolicyViolation (wrapping the offending category) if any
// detected match falls in policy.RejectCategories -- in that case Text in
// the returned result is empty and must not be used; the caller must not
// write anything. Otherwise, every match is redacted per policy.Mode /
// policy.ModeByCategory (further overridden by policy.JurisdictionRules,
// when configured) before being considered safe to persist.
func (g *StorageGuard) WriteGuarded(ctx context.Context, text string) (GuardedWriteResult, error) {
	if g.Policy.Detector == nil {
		return GuardedWriteResult{}, fmt.Errorf("%w: StoragePolicy.Detector is required", ErrInvalidRequest)
	}

	matches, err := g.Policy.Detector.Detect(ctx, text)
	if err != nil {
		return GuardedWriteResult{}, err
	}
	matches = ClassifyMatches(matches)

	sink := g.Policy.AuditSink
	if sink == nil {
		sink = NoOpAuditSink{}
	}

	for _, m := range matches {
		if g.Policy.RejectCategories[m.Category] {
			_ = sink.Emit(ctx, newAuditEvent(EventRedact, g.Policy.Actor, g.Policy.JurisdictionCode, len(matches), "", false))
			return GuardedWriteResult{}, fmt.Errorf("%w: category %q is not permitted at rest", ErrPolicyViolation, m.Category)
		}
	}

	redactor := &Redactor{Mode: g.Policy.Mode, Pseudonyms: g.Policy.Pseudonyms}
	if g.Policy.ModeByCategory != nil {
		redactor.ModeByCategory = make(map[PIICategory]RedactionMode, len(g.Policy.ModeByCategory))
		for k, v := range g.Policy.ModeByCategory {
			redactor.ModeByCategory[k] = v
		}
	}
	if g.Policy.JurisdictionRules != nil {
		overrides := g.Policy.JurisdictionRules.ApplyToMatches(g.Policy.JurisdictionCode, matches)
		if redactor.ModeByCategory == nil {
			redactor.ModeByCategory = make(map[PIICategory]RedactionMode, len(overrides))
		}
		for k, v := range overrides {
			redactor.ModeByCategory[k] = v
		}
	}

	result, err := redactor.Redact(text, matches)
	if err != nil {
		return GuardedWriteResult{}, err
	}

	_ = sink.Emit(ctx, newAuditEvent(EventRedact, g.Policy.Actor, g.Policy.JurisdictionCode, len(matches), "", true))

	return GuardedWriteResult{Text: result.Text, Matches: matches, Redaction: result}, nil
}
