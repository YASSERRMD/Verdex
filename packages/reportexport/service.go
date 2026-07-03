package reportexport

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pii"
)

// Service is the entrypoint for exporting a Report to bytes, composing
// rendering (pdf.go/docx.go/markdown.go), the optional redaction pass
// (redact.go), and export auditing (audit.go) into one actor-gated
// operation — mirroring packages/notifications.Service's composition
// style.
type Service struct {
	auditRepo AuditRepository
}

// NewService builds a Service backed by auditRepo. auditRepo must be
// non-nil.
func NewService(auditRepo AuditRepository) (*Service, error) {
	if auditRepo == nil {
		return nil, ErrNilRepository
	}
	return &Service{auditRepo: auditRepo}, nil
}

// ExportRequest carries the inputs to Service.Export.
type ExportRequest struct {
	// Report is the assembled report to render (see Assemble).
	// Required.
	Report *Report

	// Format selects the output encoding. Required, must be one of
	// the Format constants.
	Format Format

	// Redact, if true, runs RedactionOptions through Redact before
	// rendering.
	Redact bool

	// RedactionOptions configures the redaction pass when Redact is
	// true. Ignored otherwise.
	RedactionOptions RedactionOptions
}

// ExportResult is the output of Service.Export.
type ExportResult struct {
	// Bytes is the rendered document.
	Bytes []byte

	// AuditRecord is the persisted audit record for this export.
	AuditRecord *AuditRecord
}

// Export renders req.Report into req.Format, optionally redacting PII
// first, and records an AuditRecord of the export — who (from ctx),
// when, in which format, and whether redaction was applied.
//
// The ctx actor must hold identity.PermViewCase (ErrForbidden or
// ErrUnauthenticated otherwise): exporting a report is a read of the
// case's analysis, gated the same way viewing the case itself is.
func (s *Service) Export(ctx context.Context, req ExportRequest) (*ExportResult, error) {
	user, err := authorizeExport(ctx)
	if err != nil {
		return nil, err
	}
	if req.Report == nil {
		return nil, ErrNilCase
	}
	if !req.Format.IsValid() {
		return nil, ErrInvalidFormat
	}

	report := req.Report
	if req.Redact {
		opts := req.RedactionOptions
		if opts.Actor == "" {
			opts.Actor = user.ID.String()
		}
		if opts.Service == nil {
			opts.Service = pii.NewPIIService()
		}
		report, err = Redact(ctx, report, opts)
		if err != nil {
			return nil, err
		}
	}

	data, err := render(report, req.Format)
	if err != nil {
		return nil, err
	}

	rec := &AuditRecord{
		ID:       uuid.New(),
		TenantID: report.TenantID,
		CaseID:   report.CaseID,
		ActorID:  user.ID,
		Format:   req.Format,
		Redacted: req.Redact,
	}
	if err := s.auditRepo.Create(ctx, report.TenantID, rec); err != nil {
		return nil, err
	}

	return &ExportResult{Bytes: data, AuditRecord: rec}, nil
}

// AuditLog returns the export history for caseID (or the whole tenant,
// if filter.CaseID is left nil and no explicit caseID scoping is
// desired — callers pass filter.CaseID themselves). The ctx actor must
// hold identity.PermViewCase, same as Export.
func (s *Service) AuditLog(ctx context.Context, tenantID uuid.UUID, filter AuditFilter) ([]*AuditRecord, error) {
	if _, err := authorizeExport(ctx); err != nil {
		return nil, err
	}
	return s.auditRepo.List(ctx, tenantID, filter)
}

// render dispatches to the Format-specific renderer.
func render(r *Report, format Format) ([]byte, error) {
	switch format {
	case FormatPDF:
		return RenderPDF(r)
	case FormatDOCX:
		return RenderDOCX(r)
	case FormatMarkdown:
		text, err := RenderMarkdown(r)
		if err != nil {
			return nil, err
		}
		return []byte(text), nil
	case FormatText:
		text, err := RenderText(r)
		if err != nil {
			return nil, err
		}
		return []byte(text), nil
	default:
		return nil, ErrInvalidFormat
	}
}
