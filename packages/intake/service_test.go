package intake_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/intake"
)

// pdfPayload is a minimal valid-looking PDF header used in tests that need a
// "real" detectable MIME type.  http.DetectContentType recognises the %PDF-
// magic bytes and returns "application/pdf".
var pdfPayload = []byte("%PDF-1.4 minimal test fixture for intake unit tests")

// newTestService is a helper that constructs an IntakeService with
// NoOpVirusScanHook and the provided QuotaConfig.  The CapturingAuditSink is
// returned so callers can assert on events.
func newTestService(t *testing.T, cfg intake.QuotaConfig) *intake.IntakeService {
	t.Helper()
	svc := intake.NewIntakeService(
		intake.NoOpVirusScanHook{},
		intake.NewInMemoryQuotaChecker(cfg),
		&intake.CapturingAuditSink{},
		5*time.Minute,
	)
	return svc
}

// newTestServiceWithAudit is like newTestService but also returns the sink.
func newTestServiceWithAudit(t *testing.T, cfg intake.QuotaConfig) (*intake.IntakeService, *intake.CapturingAuditSink) {
	t.Helper()
	sink := &intake.CapturingAuditSink{}
	svc := intake.NewIntakeService(
		intake.NoOpVirusScanHook{},
		intake.NewInMemoryQuotaChecker(cfg),
		sink,
		5*time.Minute,
	)
	return svc, sink
}

// newUUID returns a random UUID, failing the test on error.
func newUUID(t *testing.T) uuid.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	if err != nil {
		t.Fatalf("uuid.NewRandom: %v", err)
	}
	return id
}

// TestIngest_MIMEValidationRejectsBadType verifies that uploading a binary
// type not in AllowedMIMETypes is rejected before any bytes reach the buffer.
func TestIngest_MIMEValidationRejectsBadType(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "exploit.exe",
		MIMEType:   "application/x-msdownload",
		SizeBytes:  512,
		TTL:        time.Minute,
	}

	_, err := svc.Ingest(context.Background(), req, bytes.NewReader(make([]byte, 512)))
	if err == nil {
		t.Fatal("expected error for disallowed MIME type, got nil")
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestIngest_SizeLimitEnforced verifies that a payload larger than
// QuotaConfig.MaxFileSizeMB is rejected.
func TestIngest_SizeLimitEnforced(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{MaxFileSizeMB: 1})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "large.txt",
		MIMEType:   "text/plain",
		SizeBytes:  2 * 1024 * 1024, // 2 MB
		TTL:        time.Minute,
	}

	_, err := svc.Ingest(context.Background(), req, bytes.NewReader(make([]byte, 2*1024*1024)))
	if err == nil {
		t.Fatal("expected quota error for oversized file, got nil")
	}
	if !strings.Contains(err.Error(), "quota") && !strings.Contains(err.Error(), "limit") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestIngest_QuotaExceededDailyLimit verifies that the daily upload limit is
// enforced.
func TestIngest_QuotaExceededDailyLimit(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{
		MaxFileSizeMB:            10,
		MaxDailyUploadsPerTenant: 2,
	})

	tenantID := newUUID(t)

	// Two successful uploads should be allowed.
	for i := 0; i < 2; i++ {
		req := intake.IntakeRequest{
			TenantID:   tenantID,
			UploaderID: newUUID(t),
			Filename:   "doc.pdf",
			MIMEType:   "application/pdf",
			SizeBytes:  int64(len(pdfPayload)),
			TTL:        50 * time.Millisecond,
		}
		if _, err := svc.Ingest(context.Background(), req, bytes.NewReader(pdfPayload)); err != nil {
			t.Fatalf("upload %d failed unexpectedly: %v", i+1, err)
		}
	}

	// Third upload should be rejected.
	req := intake.IntakeRequest{
		TenantID:   tenantID,
		UploaderID: newUUID(t),
		Filename:   "doc.pdf",
		MIMEType:   "application/pdf",
		SizeBytes:  int64(len(pdfPayload)),
		TTL:        time.Minute,
	}
	_, err := svc.Ingest(context.Background(), req, bytes.NewReader(pdfPayload))
	if err == nil {
		t.Fatal("expected quota error on third upload, got nil")
	}
	if !strings.Contains(err.Error(), "quota") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

// TestIngest_AuditEventsEmitted verifies that at least a started and a ready
// audit event are emitted for a successful intake operation.
func TestIngest_AuditEventsEmitted(t *testing.T) {
	svc, sink := newTestServiceWithAudit(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "doc.pdf",
		MIMEType:   "application/pdf",
		SizeBytes:  int64(len(pdfPayload)),
		TTL:        5 * time.Minute,
	}

	_, err := svc.Ingest(context.Background(), req, bytes.NewReader(pdfPayload))
	if err != nil {
		t.Fatalf("Ingest failed: %v", err)
	}

	eventTypes := make(map[string]bool)
	for _, e := range sink.Events {
		eventTypes[e.EventType] = true
	}

	for _, required := range []string{"intake.started", "intake.ready"} {
		if !eventTypes[required] {
			t.Errorf("missing audit event %q; got events: %v", required, sink.Events)
		}
	}
}

// TestIngest_HashIsDeterministic verifies that ingesting the same bytes twice
// produces identical provenance hashes.
func TestIngest_HashIsDeterministic(t *testing.T) {
	svc := newTestService(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	tenantID := newUUID(t)
	uploaderID := newUUID(t)

	ingest := func() string {
		req := intake.IntakeRequest{
			TenantID:   tenantID,
			UploaderID: uploaderID,
			Filename:   "doc.pdf",
			MIMEType:   "application/pdf",
			SizeBytes:  int64(len(pdfPayload)),
			TTL:        50 * time.Millisecond,
		}
		result, err := svc.Ingest(context.Background(), req, bytes.NewReader(pdfPayload))
		if err != nil {
			t.Fatalf("Ingest failed: %v", err)
		}
		return result.ProvisionHash
	}

	hash1 := ingest()
	hash2 := ingest()

	if hash1 != hash2 {
		t.Fatalf("hash mismatch between two identical ingests: %s vs %s", hash1, hash2)
	}
}

// TestIngest_FailureEmitsAuditEvent verifies that a failed intake still emits
// an audit event.
func TestIngest_FailureEmitsAuditEvent(t *testing.T) {
	svc, sink := newTestServiceWithAudit(t, intake.QuotaConfig{MaxFileSizeMB: 10})

	req := intake.IntakeRequest{
		TenantID:   newUUID(t),
		UploaderID: newUUID(t),
		Filename:   "bad.exe",
		MIMEType:   "application/x-msdownload", // not allowed
		SizeBytes:  128,
		TTL:        time.Minute,
	}

	_, err := svc.Ingest(context.Background(), req, bytes.NewReader(make([]byte, 128)))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	eventTypes := make(map[string]bool)
	for _, e := range sink.Events {
		eventTypes[e.EventType] = true
	}

	if !eventTypes["intake.started"] {
		t.Error("expected intake.started event even for failed intake")
	}
	if !eventTypes["intake.failed"] {
		t.Error("expected intake.failed event")
	}
}

// TestComputeSHA256_Deterministic verifies that ComputeSHA256 produces the
// well-known SHA-256 of the empty string.
func TestComputeSHA256_Deterministic(t *testing.T) {
	// SHA-256 of empty input is the well-known constant below.
	const emptyHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	hash, n, err := intake.ComputeSHA256(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ComputeSHA256 failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 bytes read, got %d", n)
	}
	if hash != emptyHash {
		t.Fatalf("expected %s, got %s", emptyHash, hash)
	}
}

// TestValidateMIME_AllowedTypes verifies each entry in AllowedMIMETypes passes
// validation.
func TestValidateMIME_AllowedTypes(t *testing.T) {
	for _, m := range intake.AllowedMIMETypes {
		if err := intake.ValidateMIME(m); err != nil {
			t.Errorf("ValidateMIME(%q) returned unexpected error: %v", m, err)
		}
	}
}

// TestValidateMIME_DisallowedType verifies that an unknown MIME type is
// rejected.
func TestValidateMIME_DisallowedType(t *testing.T) {
	if err := intake.ValidateMIME("application/x-executable"); err == nil {
		t.Fatal("expected error for disallowed MIME type, got nil")
	}
}

// TestValidateSizeMB_Enforced verifies the size limit helper.
func TestValidateSizeMB_Enforced(t *testing.T) {
	if err := intake.ValidateSizeMB(5*1024*1024, 10); err != nil {
		t.Fatalf("5 MB should pass a 10 MB limit: %v", err)
	}
	if err := intake.ValidateSizeMB(11*1024*1024, 10); err == nil {
		t.Fatal("11 MB should fail a 10 MB limit")
	}
}
