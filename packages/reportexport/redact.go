package reportexport

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/pii"
)

// RedactionOptions configures the optional PII redaction pass Redact
// applies to a Report before rendering.
type RedactionOptions struct {
	// Service performs detection and redaction. If nil,
	// pii.NewPIIService() is used (rule-based detector, ModeRedact
	// default).
	Service *pii.PIIService

	// JurisdictionCode is passed through to pii.PIIService.Process for
	// jurisdiction-specific category overrides, when the Service has
	// JurisdictionRules configured. May be empty.
	JurisdictionCode string

	// Actor identifies the caller for the underlying PII pipeline's
	// own audit trail (separate from this package's export
	// AuditRecord).
	Actor string
}

// Redact returns a copy of r with every free-text field (issue
// analysis, weakest link, and the reasoning trace appendix) passed
// through pii.PIIService.Process, replacing detected PII with
// redaction placeholders per the configured RedactionMode. Structural
// fields (IDs, citation text, party labels, confidence) are left
// untouched, since they are not narrative prose and are not where
// PII originates in a report.
//
// Redact never returns an error for text containing no PII — that is
// pii.ErrEmptyInput, which this function treats as "nothing to
// redact" and passes the original text through unchanged, not a
// failure.
func Redact(ctx context.Context, r *Report, opts RedactionOptions) (*Report, error) {
	if r == nil {
		return nil, ErrNilCase
	}

	svc := opts.Service
	if svc == nil {
		svc = pii.NewPIIService()
	}

	out := *r
	out.Issues = make([]ReportIssue, len(r.Issues))
	for i, issue := range r.Issues {
		redacted := issue
		redacted.Analysis = mustRedactText(ctx, svc, opts, issue.Analysis)
		redacted.WeakestLink = mustRedactText(ctx, svc, opts, issue.WeakestLink)
		out.Issues[i] = redacted
	}
	out.TraceAppendix = mustRedactText(ctx, svc, opts, r.TraceAppendix)

	return &out, nil
}

// mustRedactText runs text through svc.Process, returning text
// unchanged if it is empty/whitespace-only (pii.ErrEmptyInput) or if
// redaction itself fails — a redaction pipeline error must never
// silently corrupt or blank out report content; callers who need to
// observe such failures should call svc.Process directly instead of
// going through Redact.
func mustRedactText(ctx context.Context, svc *pii.PIIService, opts RedactionOptions, text string) string {
	if text == "" {
		return text
	}
	result, err := svc.Process(ctx, pii.ProcessRequest{
		Text:             text,
		JurisdictionCode: opts.JurisdictionCode,
		Actor:            opts.Actor,
	})
	if err != nil {
		return text
	}
	return result.SanitizedText
}
