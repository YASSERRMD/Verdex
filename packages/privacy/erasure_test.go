package privacy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/privacy"
)

// TestExecuteErasure_ProvenanceHashSurvives is the non-negotiable
// constraint this phase's brief calls out explicitly: erasure must
// scrub personal content while the packages/provenance chain-of-
// custody hash for that same content remains queryable, completely
// untouched. This test files an ErasureRequest referencing a
// (simulated) ProvenanceRecordID/ProvenanceHash, executes erasure with
// a ScrubFunc that actually "deletes" content from a fake downstream
// store, and asserts the returned ErasureResult still carries the
// exact original hash and record ID -- proving ExecuteErasure never
// drops, zeroes, or mutates them, regardless of what the scrub itself
// did to the personal content.
func TestExecuteErasure_ProvenanceHashSurvives(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	provenanceRecordID := uuid.New()
	const provenanceHash = "3b2e9a4f6c1d8e7f0a5b9c2d4e6f8a1b3c5d7e9f0a2b4c6d8e0f2a4c6e8f0a2b"

	submitted, err := engine.SubmitErasureRequest(ctxWithUser(admin), tenantID, privacy.ErasureRequest{
		SubjectID:          "subject-1",
		Category:           privacy.CategoryCaseParty,
		SourceTag:          "case.parties",
		RecordRef:          "party-record-42",
		ProvenanceRecordID: provenanceRecordID,
		ProvenanceHash:     provenanceHash,
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}

	// downstreamStore simulates the personal-content store ScrubFunc
	// reaches into (e.g. packages/caselifecycle). This is deliberately
	// separate from any provenance store: privacy has no dependency on
	// packages/provenance's store, so this test proves the hash
	// preservation guarantee holds purely from ExecuteErasure's own
	// control flow, not from coincidentally never touching a real
	// provenance record.
	downstreamStore := map[string]string{"party-record-42": "Jane Doe, 123 Main St"}
	scrub := func(_ context.Context, req privacy.ErasureRequest) error {
		delete(downstreamStore, req.RecordRef)
		return nil
	}

	result, err := engine.ExecuteErasure(ctxWithUser(admin), tenantID, submitted.ID, scrub)
	if err != nil {
		t.Fatalf("ExecuteErasure: %v", err)
	}

	// The personal content itself is gone.
	if !result.ContentScrubbed {
		t.Fatal("result.ContentScrubbed = false, want true")
	}
	if _, stillPresent := downstreamStore["party-record-42"]; stillPresent {
		t.Fatal("downstream content was not actually scrubbed")
	}

	// The provenance hash and record ID survive completely untouched.
	if result.ProvenanceRecordID != provenanceRecordID {
		t.Fatalf("result.ProvenanceRecordID = %v, want %v (unchanged)", result.ProvenanceRecordID, provenanceRecordID)
	}
	if result.ProvenanceHash != provenanceHash {
		t.Fatalf("result.ProvenanceHash = %q, want %q (unchanged)", result.ProvenanceHash, provenanceHash)
	}
	if !result.ProvenancePreserved {
		t.Fatal("result.ProvenancePreserved = false, want true")
	}

	// The persisted ErasureRequest itself must also still carry the
	// original hash -- it is never overwritten by ExecuteErasure.
	list, err := engine.ListErasuresForSubject(ctxWithUser(admin), tenantID, "subject-1")
	if err != nil {
		t.Fatalf("ListErasuresForSubject: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListErasuresForSubject = %d entries, want 1", len(list))
	}
	if list[0].ProvenanceHash != provenanceHash {
		t.Fatalf("persisted ErasureRequest.ProvenanceHash = %q, want %q", list[0].ProvenanceHash, provenanceHash)
	}
	if list[0].ProvenanceRecordID != provenanceRecordID {
		t.Fatalf("persisted ErasureRequest.ProvenanceRecordID = %v, want %v", list[0].ProvenanceRecordID, provenanceRecordID)
	}
	if list[0].Status != privacy.ErasureStatusCompleted {
		t.Fatalf("persisted ErasureRequest.Status = %q, want %q", list[0].Status, privacy.ErasureStatusCompleted)
	}
}

// TestExecuteErasure_NoProvenanceRecord proves erasure of content that
// never had a provenance record still succeeds, with
// ProvenancePreserved vacuously true (nothing to have broken) and both
// provenance fields at their zero value.
func TestExecuteErasure_NoProvenanceRecord(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	submitted, err := engine.SubmitErasureRequest(ctxWithUser(admin), tenantID, privacy.ErasureRequest{
		SubjectID: "subject-2",
		Category:  privacy.CategoryBehavioral,
		SourceTag: "analytics.usage",
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}

	scrub := func(context.Context, privacy.ErasureRequest) error { return nil }
	result, err := engine.ExecuteErasure(ctxWithUser(admin), tenantID, submitted.ID, scrub)
	if err != nil {
		t.Fatalf("ExecuteErasure: %v", err)
	}
	if !result.ProvenancePreserved {
		t.Fatal("result.ProvenancePreserved = false, want true (vacuously, no provenance record existed)")
	}
	if result.ProvenanceRecordID != uuid.Nil {
		t.Fatalf("result.ProvenanceRecordID = %v, want uuid.Nil", result.ProvenanceRecordID)
	}
	if result.ProvenanceHash != "" {
		t.Fatalf("result.ProvenanceHash = %q, want empty", result.ProvenanceHash)
	}
}

// TestExecuteErasure_ScrubFailure_PreservesRequestState proves a
// failing ScrubFunc leaves the ErasureRequest in ErasureStatusReceived
// (not silently marked completed), so a retry can still find and
// re-attempt it.
func TestExecuteErasure_ScrubFailure_PreservesRequestState(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	submitted, err := engine.SubmitErasureRequest(ctxWithUser(admin), tenantID, privacy.ErasureRequest{
		SubjectID: "subject-3",
		Category:  privacy.CategoryTranscript,
		SourceTag: "ingestion.transcript",
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}

	scrubErr := errors.New("downstream store unavailable")
	scrub := func(context.Context, privacy.ErasureRequest) error { return scrubErr }

	_, err = engine.ExecuteErasure(ctxWithUser(admin), tenantID, submitted.ID, scrub)
	if err == nil {
		t.Fatal("ExecuteErasure() error = nil, want the wrapped scrub error")
	}

	list, err := engine.ListErasuresForSubject(ctxWithUser(admin), tenantID, "subject-3")
	if err != nil {
		t.Fatalf("ListErasuresForSubject: %v", err)
	}
	if len(list) != 1 || list[0].Status != privacy.ErasureStatusReceived {
		t.Fatalf("ErasureRequest.Status after failed scrub = %v, want still %q", list, privacy.ErasureStatusReceived)
	}
}

// TestExecuteErasure_AlreadyErasedRejected proves a second
// ExecuteErasure call against the same request is rejected rather
// than silently re-running (or double-counting) the scrub.
func TestExecuteErasure_AlreadyErasedRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	submitted, err := engine.SubmitErasureRequest(ctxWithUser(admin), tenantID, privacy.ErasureRequest{
		SubjectID: "subject-4",
		Category:  privacy.CategoryContact,
		SourceTag: "case.parties",
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}

	callCount := 0
	scrub := func(context.Context, privacy.ErasureRequest) error {
		callCount++
		return nil
	}

	if _, err := engine.ExecuteErasure(ctxWithUser(admin), tenantID, submitted.ID, scrub); err != nil {
		t.Fatalf("first ExecuteErasure: %v", err)
	}
	_, err = engine.ExecuteErasure(ctxWithUser(admin), tenantID, submitted.ID, scrub)
	if !errors.Is(err, privacy.ErrAlreadyErased) {
		t.Fatalf("second ExecuteErasure() error = %v, want ErrAlreadyErased", err)
	}
	if callCount != 1 {
		t.Fatalf("scrub was called %d times, want exactly 1", callCount)
	}
}

// TestErasureRequest_Validate_ProvenanceHashRequired proves the
// application-level guard mirrors the database CHECK constraint added
// in migration 000024: a request cannot reference a provenance record
// without also carrying its hash.
func TestErasureRequest_Validate_ProvenanceHashRequired(t *testing.T) {
	t.Parallel()

	req := &privacy.ErasureRequest{
		TenantID:           uuid.New(),
		SubjectID:          "subject-1",
		Category:           privacy.CategoryCaseParty,
		SourceTag:          "case.parties",
		Status:             privacy.ErasureStatusReceived,
		ProvenanceRecordID: uuid.New(),
		// ProvenanceHash deliberately left blank.
	}
	req.RequestedAt = time.Now()

	err := req.Validate()
	if !errors.Is(err, privacy.ErrProvenanceHashRequired) {
		t.Fatalf("Validate() error = %v, want ErrProvenanceHashRequired", err)
	}
}

func TestEngine_ExecuteErasure_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	auditor := auditorUser(tenantID)

	submitted, err := engine.SubmitErasureRequest(ctxWithUser(admin), tenantID, privacy.ErasureRequest{
		SubjectID: "subject-5",
		Category:  privacy.CategoryCaseParty,
		SourceTag: "case.parties",
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}

	scrub := func(context.Context, privacy.ErasureRequest) error { return nil }
	_, err = engine.ExecuteErasure(ctxWithUser(auditor), tenantID, submitted.ID, scrub)
	if !errors.Is(err, privacy.ErrForbidden) {
		t.Fatalf("ExecuteErasure() error = %v, want ErrForbidden", err)
	}
}
