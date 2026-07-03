package reportexport_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/reportexport"
)

func TestService_Export_RecordsAuditForEveryFormat(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)
	user := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(user)

	svc, repo := newTestService(t)

	for _, format := range []reportexport.Format{reportexport.FormatPDF, reportexport.FormatDOCX, reportexport.FormatMarkdown, reportexport.FormatText} {
		result, err := svc.Export(ctx, reportexport.ExportRequest{Report: report, Format: format})
		if err != nil {
			t.Fatalf("Export(%s): %v", format, err)
		}
		if len(result.Bytes) == 0 {
			t.Errorf("Export(%s) returned empty bytes", format)
		}
		if result.AuditRecord == nil {
			t.Fatalf("Export(%s) returned nil AuditRecord", format)
		}
		if result.AuditRecord.Format != format {
			t.Errorf("AuditRecord.Format = %q, want %q", result.AuditRecord.Format, format)
		}
		if result.AuditRecord.ActorID != user.ID {
			t.Errorf("AuditRecord.ActorID = %v, want %v", result.AuditRecord.ActorID, user.ID)
		}
		if result.AuditRecord.CaseID != c.ID {
			t.Errorf("AuditRecord.CaseID = %v, want %v", result.AuditRecord.CaseID, c.ID)
		}
	}

	records, err := repo.List(ctx, tenantID, reportexport.AuditFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(records) != 4 {
		t.Fatalf("len(records) = %d, want 4 (one per format exported)", len(records))
	}
}

func TestService_Export_AuditRecordsRedactionFlag(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "Contact jane.doe@example.com for details.")
	report := newAssembledReport(t, c, opinion)
	user := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(user)

	svc, _ := newTestService(t)

	unredacted, err := svc.Export(ctx, reportexport.ExportRequest{Report: report, Format: reportexport.FormatMarkdown, Redact: false})
	if err != nil {
		t.Fatalf("Export (unredacted): %v", err)
	}
	if unredacted.AuditRecord.Redacted {
		t.Errorf("unredacted export's AuditRecord.Redacted = true, want false")
	}
	if !bytes.Contains(unredacted.Bytes, []byte("jane.doe@example.com")) {
		t.Errorf("unredacted export should still contain the original PII")
	}

	redacted, err := svc.Export(ctx, reportexport.ExportRequest{Report: report, Format: reportexport.FormatMarkdown, Redact: true})
	if err != nil {
		t.Fatalf("Export (redacted): %v", err)
	}
	if !redacted.AuditRecord.Redacted {
		t.Errorf("redacted export's AuditRecord.Redacted = false, want true")
	}
	if bytes.Contains(redacted.Bytes, []byte("jane.doe@example.com")) {
		t.Errorf("redacted export should not contain the original PII")
	}
}

func TestService_Export_RequiresPermission(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)

	svc, _ := newTestService(t)

	// No user in context at all.
	if _, err := svc.Export(context.Background(), reportexport.ExportRequest{Report: report, Format: reportexport.FormatPDF}); err != reportexport.ErrUnauthenticated {
		t.Errorf("Export with no actor: err = %v, want ErrUnauthenticated", err)
	}

	// An authenticated user with no roles at all (and therefore no
	// identity.PermViewCase grant) must be forbidden, not merely
	// unauthenticated.
	noRoleUser := &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "no-role@example.test",
		Status:   identity.UserStatusActive,
	}
	if _, err := svc.Export(ctxWithUser(noRoleUser), reportexport.ExportRequest{Report: report, Format: reportexport.FormatPDF}); err != reportexport.ErrForbidden {
		t.Errorf("Export with no-permission actor: err = %v, want ErrForbidden", err)
	}
}

func TestService_Export_InvalidFormat(t *testing.T) {
	tenantID := uuid.New()
	c := newTestCase(tenantID)
	opinion := newTestOpinion(c.ID, "Analysis text.")
	report := newAssembledReport(t, c, opinion)
	user := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(user)

	svc, _ := newTestService(t)

	if _, err := svc.Export(ctx, reportexport.ExportRequest{Report: report, Format: "invalid"}); err != reportexport.ErrInvalidFormat {
		t.Errorf("Export(invalid format): err = %v, want ErrInvalidFormat", err)
	}
}

func TestService_Export_NilReport(t *testing.T) {
	tenantID := uuid.New()
	user := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(user)

	svc, _ := newTestService(t)

	if _, err := svc.Export(ctx, reportexport.ExportRequest{Report: nil, Format: reportexport.FormatPDF}); err != reportexport.ErrNilCase {
		t.Errorf("Export(nil report): err = %v, want ErrNilCase", err)
	}
}

func TestService_AuditLog_FiltersByCaseAndFormat(t *testing.T) {
	tenantID := uuid.New()
	c1 := newTestCase(tenantID)
	c2 := newTestCase(tenantID)
	opinion1 := newTestOpinion(c1.ID, "Analysis 1.")
	opinion2 := newTestOpinion(c2.ID, "Analysis 2.")
	report1 := newAssembledReport(t, c1, opinion1)
	report2 := newAssembledReport(t, c2, opinion2)
	user := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(user)

	svc, _ := newTestService(t)

	if _, err := svc.Export(ctx, reportexport.ExportRequest{Report: report1, Format: reportexport.FormatPDF}); err != nil {
		t.Fatalf("Export report1 PDF: %v", err)
	}
	if _, err := svc.Export(ctx, reportexport.ExportRequest{Report: report1, Format: reportexport.FormatDOCX}); err != nil {
		t.Fatalf("Export report1 DOCX: %v", err)
	}
	if _, err := svc.Export(ctx, reportexport.ExportRequest{Report: report2, Format: reportexport.FormatPDF}); err != nil {
		t.Fatalf("Export report2 PDF: %v", err)
	}

	caseID := c1.ID
	records, err := svc.AuditLog(ctx, tenantID, reportexport.AuditFilter{CaseID: &caseID})
	if err != nil {
		t.Fatalf("AuditLog: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("len(records) for case1 = %d, want 2", len(records))
	}
	for _, r := range records {
		if r.CaseID != c1.ID {
			t.Errorf("record CaseID = %v, want %v", r.CaseID, c1.ID)
		}
	}

	pdfRecords, err := svc.AuditLog(ctx, tenantID, reportexport.AuditFilter{Format: reportexport.FormatPDF})
	if err != nil {
		t.Fatalf("AuditLog: %v", err)
	}
	if len(pdfRecords) != 2 {
		t.Fatalf("len(pdfRecords) = %d, want 2", len(pdfRecords))
	}
}

func TestNewService_NilRepository(t *testing.T) {
	if _, err := reportexport.NewService(nil); err != reportexport.ErrNilRepository {
		t.Errorf("NewService(nil) err = %v, want ErrNilRepository", err)
	}
}
