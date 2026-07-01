package intake

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"
)

// ScanResult encapsulates the outcome of a VirusScanHook invocation.
type ScanResult struct {
	// Clean is true when the scanner found no threats.
	Clean bool

	// Details contains the human-readable verdict string returned by the
	// scanner.  May be empty for a clean result from lightweight scanners.
	Details string

	// ScannedAt is the wall-clock time at which the scan completed.
	ScannedAt time.Time
}

// VirusScanHook is the extension point for antivirus / content-threat scanning.
// Implementations receive a streaming reader so they never need to buffer the
// full payload in memory.
//
// Implementations must not close r.
type VirusScanHook interface {
	// Scan reads the payload from r and returns whether it is clean.  A
	// non-nil error indicates an infrastructure failure (not a threat
	// detection); callers should treat scan errors as a reason to reject the
	// upload.
	Scan(ctx context.Context, r io.Reader, filename string) (clean bool, details string, err error)
}

// NoOpVirusScanHook is a pass-through implementation that always reports the
// payload as clean without inspecting its contents.  Use in development and
// testing environments.
type NoOpVirusScanHook struct{}

// Scan implements VirusScanHook.  It drains r so downstream readers see EOF.
func (NoOpVirusScanHook) Scan(_ context.Context, r io.Reader, _ string) (bool, string, error) {
	// Drain the reader so the caller's position is consistent.
	if _, err := io.Copy(io.Discard, r); err != nil {
		return false, "", fmt.Errorf("intake: noop scanner drain failed: %w", err)
	}
	return true, "no-op: scan skipped", nil
}

// LoggingVirusScanHook wraps another VirusScanHook and logs every scan
// invocation and result using the structured logger.
type LoggingVirusScanHook struct {
	inner  VirusScanHook
	logger *slog.Logger
}

// NewLoggingVirusScanHook creates a LoggingVirusScanHook that delegates to
// inner and logs via logger.  If logger is nil, slog.Default() is used.
func NewLoggingVirusScanHook(inner VirusScanHook, logger *slog.Logger) *LoggingVirusScanHook {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingVirusScanHook{inner: inner, logger: logger}
}

// Scan implements VirusScanHook.
func (l *LoggingVirusScanHook) Scan(ctx context.Context, r io.Reader, filename string) (bool, string, error) {
	l.logger.InfoContext(ctx, "intake: virus scan starting", "filename", filename)
	start := time.Now()

	clean, details, err := l.inner.Scan(ctx, r, filename)

	elapsed := time.Since(start)
	if err != nil {
		l.logger.ErrorContext(ctx, "intake: virus scan error",
			"filename", filename,
			"elapsed_ms", elapsed.Milliseconds(),
			"error", err,
		)
		return false, "", err
	}

	l.logger.InfoContext(ctx, "intake: virus scan complete",
		"filename", filename,
		"clean", clean,
		"details", details,
		"elapsed_ms", elapsed.Milliseconds(),
	)
	return clean, details, nil
}
