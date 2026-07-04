package intake

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// defaultDiscardDelay is the TTL applied to TempBuffers when the caller
	// does not specify one via IntakeRequest.TTL.
	defaultDiscardDelay = 5 * time.Minute

	// sniffSize is the number of bytes read upfront to detect MIME type.
	sniffSize = 512
)

// IntakeService orchestrates the validate → buffer → hash → scan → audit →
// discard pipeline for every incoming upload.
type IntakeService struct {
	scanner      VirusScanHook
	quota        QuotaChecker
	audit        AuditSink
	discardDelay time.Duration

	mu      sync.Mutex
	buffers map[uuid.UUID]*TempBuffer
}

// NewIntakeService constructs an IntakeService.
//
//   - scanner: the VirusScanHook to apply after buffering (use NoOpVirusScanHook
//     in environments without AV infrastructure).
//   - quota: the QuotaChecker consulted before any bytes are written.
//   - audit: the AuditSink that receives lifecycle events.
//   - discardDelay: default TTL for TempBuffers; overridden per-request by
//     IntakeRequest.TTL when non-zero.
func NewIntakeService(
	scanner VirusScanHook,
	quota QuotaChecker,
	audit AuditSink,
	discardDelay time.Duration,
) *IntakeService {
	if discardDelay <= 0 {
		discardDelay = defaultDiscardDelay
	}
	return &IntakeService{
		scanner:      scanner,
		quota:        quota,
		audit:        audit,
		discardDelay: discardDelay,
		buffers:      make(map[uuid.UUID]*TempBuffer),
	}
}

// Ingest executes the full intake pipeline for a single upload:
//
//  1. Validate declared MIME type and size against the allowlist.
//  2. Check quota (size, daily, concurrent).
//  3. Read the first sniffSize bytes to detect actual MIME type.
//  4. Create a TempBuffer with the appropriate TTL.
//  5. Stream the payload through StreamingHashReader into the TempBuffer.
//  6. Virus-scan the buffered payload.
//  7. Emit a terminal audit event (ready or failed).
//  8. Schedule automatic discard after the TTL.
//
// The returned *IntakeResult contains the provenance hash and final status.
// The binary is guaranteed to be discarded after the TTL regardless of whether
// the caller retains a reference to the result.
func (s *IntakeService) Ingest(ctx context.Context, req IntakeRequest, body io.Reader) (*IntakeResult, error) {
	intakeID := uuid.New()
	ttl := s.discardDelay
	if req.TTL > 0 {
		ttl = req.TTL
	}

	// Emit a started event immediately so the audit trail begins even when
	// subsequent steps fail.
	_ = s.audit.Emit(ctx, IntakeAuditEvent{
		EventType:  "intake.started",
		IntakeID:   intakeID,
		TenantID:   req.TenantID,
		CaseID:     req.CaseID,
		UploaderID: req.UploaderID,
		Filename:   req.Filename,
		Status:     StatusReceiving,
		Timestamp:  time.Now().UTC(),
	})

	result, err := s.ingest(ctx, intakeID, ttl, req, body)
	if err != nil {
		now := time.Now().UTC()
		_ = s.audit.Emit(ctx, IntakeAuditEvent{
			EventType:  "intake.failed",
			IntakeID:   intakeID,
			TenantID:   req.TenantID,
			CaseID:     req.CaseID,
			UploaderID: req.UploaderID,
			Filename:   req.Filename,
			Status:     StatusFailed,
			Timestamp:  now,
		})
		if s.quota != nil {
			s.quota.RecordComplete(req.TenantID)
		}
		return nil, err
	}
	return result, nil
}

// ingest is the inner implementation; errors here cause Ingest to emit a
// failure event.
func (s *IntakeService) ingest(
	ctx context.Context,
	intakeID uuid.UUID,
	ttl time.Duration,
	req IntakeRequest,
	body io.Reader,
) (*IntakeResult, error) {

	// Step 1: validate declared MIME type.
	if err := ValidateMIME(req.MIMEType); err != nil {
		return nil, err
	}

	// Step 2: quota check (also validates size via QuotaConfig.MaxFileSizeMB).
	if s.quota != nil {
		if err := s.quota.Check(ctx, req.TenantID, req); err != nil {
			return nil, err
		}
		// RecordComplete is deferred in the Ingest wrapper.
		defer s.quota.RecordComplete(req.TenantID)
	}

	// Step 3: sniff actual MIME type from the first bytes.
	sniff := make([]byte, sniffSize)
	n, err := io.ReadAtLeast(body, sniff, 1)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("intake: failed to read sniff bytes: %w", err)
	}
	sniff = sniff[:n]
	detectedMIME := DetectMIME(sniff)
	// Use the detected MIME type for further validation.
	if err2 := ValidateMIME(detectedMIME); err2 != nil {
		// Detected type not allowed.
		return nil, fmt.Errorf("intake: detected MIME type mismatch: %w", err2)
	}

	// Reconstruct the full stream: sniffed bytes + remainder.
	fullBody := io.MultiReader(bytes.NewReader(sniff), body)

	// Step 4: create TempBuffer.
	tb, err := Create(ttl)
	if err != nil {
		return nil, fmt.Errorf("intake: failed to create temp buffer: %w", err)
	}
	s.registerBuffer(intakeID, tb)

	// Step 5: stream through StreamingHashReader into TempBuffer.
	_ = s.audit.Emit(ctx, IntakeAuditEvent{
		EventType:  "intake.hashing",
		IntakeID:   intakeID,
		TenantID:   req.TenantID,
		CaseID:     req.CaseID,
		UploaderID: req.UploaderID,
		Filename:   req.Filename,
		Status:     StatusHashing,
		Timestamp:  time.Now().UTC(),
	})

	hashReader := NewStreamingHashReader(fullBody)
	written, err := io.Copy(tb, hashReader)
	if err != nil {
		_ = tb.Discard()
		s.unregisterBuffer(intakeID)
		return nil, fmt.Errorf("intake: streaming copy failed after %d bytes: %w", written, err)
	}
	receivedAt := time.Now().UTC()
	hash := hashReader.Hash()

	// Step 6: virus scan.
	_ = s.audit.Emit(ctx, IntakeAuditEvent{
		EventType:  "intake.scanning",
		IntakeID:   intakeID,
		TenantID:   req.TenantID,
		CaseID:     req.CaseID,
		UploaderID: req.UploaderID,
		Filename:   req.Filename,
		Hash:       hash,
		Status:     StatusScanning,
		Timestamp:  time.Now().UTC(),
	})

	scanReader, err := tb.Reader()
	if err != nil {
		_ = tb.Discard()
		s.unregisterBuffer(intakeID)
		return nil, fmt.Errorf("intake: failed to open buffer for scanning: %w", err)
	}

	clean, details, err := s.scanner.Scan(ctx, scanReader, req.Filename)
	if err != nil {
		_ = tb.Discard()
		s.unregisterBuffer(intakeID)
		return nil, fmt.Errorf("intake: virus scan error: %w", err)
	}
	if !clean {
		_ = tb.Discard()
		s.unregisterBuffer(intakeID)
		return nil, fmt.Errorf("intake: payload failed virus scan: %s", details)
	}

	// Step 7: emit ready event.
	_ = s.audit.Emit(ctx, IntakeAuditEvent{
		EventType:  "intake.ready",
		IntakeID:   intakeID,
		TenantID:   req.TenantID,
		CaseID:     req.CaseID,
		UploaderID: req.UploaderID,
		Filename:   req.Filename,
		Hash:       hash,
		Status:     StatusReady,
		Timestamp:  receivedAt,
	})

	result := &IntakeResult{
		IntakeID:      intakeID,
		ProvisionHash: hash,
		MIMEType:      detectedMIME,
		SizeBytes:     hashReader.BytesRead(),
		ReceivedAt:    receivedAt,
		status:        StatusReady,
	}

	// Step 8: schedule discard.  The TempBuffer's internal timer already fires
	// at ExpiresAt; we additionally schedule an explicit goroutine that emits
	// the audit event and updates the result.
	go func() {
		time.Sleep(time.Until(tb.ExpiresAt))
		discardTime := time.Now().UTC()
		_ = tb.Discard()
		s.unregisterBuffer(intakeID)
		result.markDiscarded(discardTime)
		_ = s.audit.Emit(ctx, IntakeAuditEvent{
			EventType:  "intake.discarded",
			IntakeID:   intakeID,
			TenantID:   req.TenantID,
			CaseID:     req.CaseID,
			UploaderID: req.UploaderID,
			Filename:   req.Filename,
			Hash:       hash,
			Status:     StatusDiscarded,
			Timestamp:  discardTime,
		})
	}()

	return result, nil
}

// DiscardAll immediately discards the TempBuffer associated with intakeID, if
// any.  Returns nil when the buffer is already discarded or was never
// registered.
func (s *IntakeService) DiscardAll(ctx context.Context, intakeID uuid.UUID) error {
	s.mu.Lock()
	tb, ok := s.buffers[intakeID]
	s.mu.Unlock()

	if !ok || tb == nil {
		return nil
	}

	if err := tb.Discard(); err != nil {
		return fmt.Errorf("intake: DiscardAll(%s): %w", intakeID, err)
	}
	s.unregisterBuffer(intakeID)
	return nil
}

func (s *IntakeService) registerBuffer(id uuid.UUID, tb *TempBuffer) {
	s.mu.Lock()
	s.buffers[id] = tb
	s.mu.Unlock()
}

func (s *IntakeService) unregisterBuffer(id uuid.UUID) {
	s.mu.Lock()
	delete(s.buffers, id)
	s.mu.Unlock()
}
