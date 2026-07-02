package pii

import (
	"context"
	"strings"
)

// PIIService orchestrates the full PII pipeline:
//
//	detect -> classify -> apply jurisdiction rules -> redact/pseudonymize
//	  per configured mode -> audit -> return sanitized text + match report
//
// This mirrors packages/segmentation's SegmentationService orchestration
// pattern: a single entry point wires together this package's otherwise
// independent, individually-testable building blocks (Detector,
// ClassifyMatches, JurisdictionPIIRules, Redactor, AuditSink).
type PIIService struct {
	// Detector finds PII in input text. If nil, NewRuleBasedDetector() is
	// used.
	Detector Detector

	// JurisdictionRules optionally supplies per-jurisdiction category
	// overrides (see jurisdiction_rules.go). May be nil, in which case no
	// jurisdiction-specific overrides are applied.
	JurisdictionRules *JurisdictionPIIRules

	// Mode is the default redaction mode. Defaults to ModeRedact if left
	// at the zero value.
	Mode RedactionMode

	// ModeByCategory optionally overrides Mode per category, exactly like
	// Redactor.ModeByCategory.
	ModeByCategory map[PIICategory]RedactionMode

	// Pseudonyms is required when Mode (or any override, including a
	// jurisdiction-rule-required mode) is ModePseudonymize.
	Pseudonyms *PseudonymMap

	// AuditSink receives an audit event for every Process call. If nil,
	// NoOpAuditSink is used.
	AuditSink AuditSink
}

// NewPIIService constructs a PIIService with sensible defaults for every
// pluggable dependency left nil: RuleBasedDetector, ModeRedact, and
// NoOpAuditSink.
func NewPIIService() *PIIService {
	return &PIIService{
		Detector: NewRuleBasedDetector(),
		Mode:     ModeRedact,
	}
}

// ProcessRequest carries the input to PIIService.Process.
type ProcessRequest struct {
	// Text is the source text to scan and sanitize.
	Text string

	// JurisdictionCode is the jurisdiction code to evaluate
	// JurisdictionRules under, when configured. Ignored if
	// JurisdictionRules is nil.
	JurisdictionCode string

	// Actor identifies the caller for audit purposes.
	Actor string
}

// ProcessResult is the output of PIIService.Process.
type ProcessResult struct {
	// SanitizedText is the input text with every detected PII match
	// redacted or pseudonymized per the service's configured mode(s).
	SanitizedText string

	// Matches is every PIIMatch detected in the input, classified (see
	// category.go).
	Matches []PIIMatch

	// Redaction is the full redaction record (see redact.go).
	Redaction RedactionResult
}

// Process runs the full pipeline over req: detect -> classify -> apply
// jurisdiction rules -> redact/pseudonymize -> audit -> return sanitized
// text + match report.
//
// Returns ErrEmptyInput if req.Text is empty or whitespace-only.
func (s *PIIService) Process(ctx context.Context, req ProcessRequest) (ProcessResult, error) {
	if strings.TrimSpace(req.Text) == "" {
		return ProcessResult{}, ErrEmptyInput
	}

	detector := s.Detector
	if detector == nil {
		detector = NewRuleBasedDetector()
	}

	sink := s.AuditSink
	if sink == nil {
		sink = NoOpAuditSink{}
	}

	matches, err := detector.Detect(ctx, req.Text)
	if err != nil {
		return ProcessResult{}, err
	}
	matches = ClassifyMatches(matches)

	_ = sink.Emit(ctx, newAuditEvent(EventDetect, req.Actor, req.JurisdictionCode, len(matches), "", true))

	mode := s.Mode
	if mode == "" {
		mode = ModeRedact
	}

	redactor := &Redactor{Mode: mode, Pseudonyms: s.Pseudonyms}
	if s.ModeByCategory != nil {
		redactor.ModeByCategory = make(map[PIICategory]RedactionMode, len(s.ModeByCategory))
		for k, v := range s.ModeByCategory {
			redactor.ModeByCategory[k] = v
		}
	}
	if s.JurisdictionRules != nil {
		overrides := s.JurisdictionRules.ApplyToMatches(req.JurisdictionCode, matches)
		if redactor.ModeByCategory == nil {
			redactor.ModeByCategory = make(map[PIICategory]RedactionMode, len(overrides))
		}
		for k, v := range overrides {
			redactor.ModeByCategory[k] = v
		}
	}

	result, err := redactor.Redact(req.Text, matches)
	if err != nil {
		return ProcessResult{}, err
	}

	_ = sink.Emit(ctx, newAuditEvent(EventRedact, req.Actor, req.JurisdictionCode, len(matches), "", true))

	return ProcessResult{SanitizedText: result.Text, Matches: matches, Redaction: result}, nil
}

// Reveal reverses a pseudonym token via the service's configured
// PseudonymMap, auditing the attempt regardless of outcome.
//
// Returns ErrInvalidRequest if no Pseudonyms map is configured, or whatever
// error PseudonymMap.Reveal returns (ErrAccessDenied, ErrUnknownToken, or
// ErrAlreadyIrreversible).
func (s *PIIService) Reveal(ctx context.Context, actor, jurisdictionCode, token string) (string, error) {
	sink := s.AuditSink
	if sink == nil {
		sink = NoOpAuditSink{}
	}

	if s.Pseudonyms == nil {
		_ = sink.Emit(ctx, newAuditEvent(EventReveal, actor, jurisdictionCode, 0, token, false))
		return "", ErrInvalidRequest
	}

	original, err := s.Pseudonyms.Reveal(ctx, actor, token)
	_ = sink.Emit(ctx, newAuditEvent(EventReveal, actor, jurisdictionCode, 0, token, err == nil))
	return original, err
}
