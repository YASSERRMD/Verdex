package intake

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ErrQuotaExceeded is returned when an intake request violates a quota limit.
var ErrQuotaExceeded = errors.New("intake: quota exceeded")

// QuotaConfig specifies the limits enforced by a QuotaChecker.
type QuotaConfig struct {
	// MaxFileSizeMB is the maximum permitted file size in megabytes.  A value
	// of 0 disables the size check.
	MaxFileSizeMB int

	// MaxDailyUploadsPerTenant is the maximum number of successful intake
	// operations a single tenant may perform in a rolling 24-hour window.  A
	// value of 0 disables the check.
	MaxDailyUploadsPerTenant int

	// MaxConcurrentPerTenant is the maximum number of in-flight intake
	// operations a single tenant may have at the same time.  A value of 0
	// disables the check.
	MaxConcurrentPerTenant int
}

// QuotaChecker is the extension point for quota enforcement.
type QuotaChecker interface {
	// Check validates that req does not violate any quota for tenantID.  It
	// returns ErrQuotaExceeded (or a wrapped variant) when a limit is hit, or
	// another non-nil error for infrastructure failures.
	Check(ctx context.Context, tenantID uuid.UUID, req IntakeRequest) error

	// RecordComplete must be called after an intake operation finishes
	// (successfully or not) so that concurrent-upload counters are decremented.
	RecordComplete(tenantID uuid.UUID)
}

// dailyBucket tracks per-tenant daily upload counts.
type dailyBucket struct {
	count int
	date  string // "YYYY-MM-DD" in UTC
}

// InMemoryQuotaChecker is a non-persistent QuotaChecker suitable for single-
// instance deployments and testing.  It holds counters in memory; counters are
// lost on process restart.
type InMemoryQuotaChecker struct {
	cfg    QuotaConfig
	mu     sync.Mutex
	daily  map[uuid.UUID]*dailyBucket
	active map[uuid.UUID]int // concurrent uploads in-flight
}

// NewInMemoryQuotaChecker creates an InMemoryQuotaChecker using cfg.
func NewInMemoryQuotaChecker(cfg QuotaConfig) *InMemoryQuotaChecker {
	return &InMemoryQuotaChecker{
		cfg:    cfg,
		daily:  make(map[uuid.UUID]*dailyBucket),
		active: make(map[uuid.UUID]int),
	}
}

// Check implements QuotaChecker.
func (q *InMemoryQuotaChecker) Check(_ context.Context, tenantID uuid.UUID, req IntakeRequest) error {
	// 1. File-size check.
	if q.cfg.MaxFileSizeMB > 0 {
		if err := ValidateSizeMB(req.SizeBytes, q.cfg.MaxFileSizeMB); err != nil {
			return fmt.Errorf("%w: %s", ErrQuotaExceeded, err.Error())
		}
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")

	// 2. Concurrent-upload check.
	if q.cfg.MaxConcurrentPerTenant > 0 {
		if q.active[tenantID] >= q.cfg.MaxConcurrentPerTenant {
			return fmt.Errorf("%w: tenant %s has %d concurrent uploads (limit %d)",
				ErrQuotaExceeded, tenantID, q.active[tenantID], q.cfg.MaxConcurrentPerTenant)
		}
	}

	// 3. Daily-upload check.
	if q.cfg.MaxDailyUploadsPerTenant > 0 {
		bucket := q.daily[tenantID]
		if bucket == nil || bucket.date != today {
			bucket = &dailyBucket{date: today}
			q.daily[tenantID] = bucket
		}
		if bucket.count >= q.cfg.MaxDailyUploadsPerTenant {
			return fmt.Errorf("%w: tenant %s has reached the daily upload limit of %d",
				ErrQuotaExceeded, tenantID, q.cfg.MaxDailyUploadsPerTenant)
		}
		bucket.count++
	}

	// Increment concurrent counter (decremented by RecordComplete).
	q.active[tenantID]++

	return nil
}

// RecordComplete implements QuotaChecker.
func (q *InMemoryQuotaChecker) RecordComplete(tenantID uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.active[tenantID] > 0 {
		q.active[tenantID]--
	}
}
