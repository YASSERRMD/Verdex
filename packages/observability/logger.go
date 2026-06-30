package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Level is a logging severity level, decoupled from slog's own Level
// type so call sites never need to import log/slog directly.
type Level int

// Supported logging levels, ordered from most to least verbose.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// String implements fmt.Stringer.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "info"
	}
}

// ParseLevel parses a case-insensitive level name ("debug", "info",
// "warn"/"warning", "error") into a Level. Unrecognized input returns
// LevelInfo and a non-nil error.
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("observability: unrecognized log level %q", s)
	}
}

func (l Level) slogLevel() slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Format selects the wire encoding used for log output.
type Format int

// Supported log output formats.
const (
	// FormatJSON emits one JSON object per log record (the default for
	// production deployments).
	FormatJSON Format = iota
	// FormatConsole emits a human-readable, slog "text" style line per
	// record, suited to local development terminals.
	FormatConsole
)

// ParseFormat parses a case-insensitive format name ("json" or
// "console"/"text") into a Format. Unrecognized input returns
// FormatJSON and a non-nil error.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json", "":
		return FormatJSON, nil
	case "console", "text":
		return FormatConsole, nil
	default:
		return FormatJSON, fmt.Errorf("observability: unrecognized log format %q", s)
	}
}

// Logger is Verdex's structured logger. It wraps log/slog so call
// sites depend on this small interface-shaped type rather than on
// slog directly, which keeps the door open to swapping the underlying
// implementation later without touching call sites.
//
// Logger is safe for concurrent use, since it only ever wraps an
// *slog.Logger (itself safe for concurrent use).
type Logger struct {
	slog *slog.Logger
}

// Option configures a Logger constructed by New.
type Option func(*loggerConfig)

type loggerConfig struct {
	level  Level
	format Format
	output io.Writer
}

// WithLevel sets the minimum severity the Logger emits. The default is
// LevelInfo.
func WithLevel(level Level) Option {
	return func(c *loggerConfig) { c.level = level }
}

// WithFormat sets the output encoding. The default is FormatJSON.
func WithFormat(format Format) Option {
	return func(c *loggerConfig) { c.format = format }
}

// WithOutput sets the destination io.Writer. The default is os.Stdout.
func WithOutput(w io.Writer) Option {
	return func(c *loggerConfig) { c.output = w }
}

// New constructs a Logger. With no options it logs at info level, in
// JSON format, to os.Stdout.
func New(opts ...Option) *Logger {
	cfg := loggerConfig{
		level:  LevelInfo,
		format: FormatJSON,
		output: os.Stdout,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	handlerOpts := &slog.HandlerOptions{Level: cfg.level.slogLevel()}

	var handler slog.Handler
	switch cfg.format {
	case FormatConsole:
		handler = slog.NewTextHandler(cfg.output, handlerOpts)
	case FormatJSON:
		handler = slog.NewJSONHandler(cfg.output, handlerOpts)
	default:
		handler = slog.NewJSONHandler(cfg.output, handlerOpts)
	}

	return &Logger{slog: slog.New(handler)}
}

// With returns a child Logger that attaches the given key-value field
// pairs to every subsequent log record, in addition to any fields
// already attached to the receiver. Use this to bind request-scoped
// context such as a tenant ID or case ID once and reuse the result.
//
// args must be an even-length list of alternating keys (string) and
// values, matching the convention used by log/slog.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{slog: l.slog.With(args...)}
}

// Debug logs msg at debug level with the given structured fields.
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
	l.slog.DebugContext(ctx, msg, args...)
}

// Info logs msg at info level with the given structured fields.
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
	l.slog.InfoContext(ctx, msg, args...)
}

// Warn logs msg at warn level with the given structured fields.
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
	l.slog.WarnContext(ctx, msg, args...)
}

// Error logs msg at error level with the given structured fields.
func (l *Logger) Error(ctx context.Context, msg string, args ...any) {
	l.slog.ErrorContext(ctx, msg, args...)
}

// Enabled reports whether a record at the given level would be emitted
// by this Logger, given its configured minimum level.
func (l *Logger) Enabled(ctx context.Context, level Level) bool {
	return l.slog.Enabled(ctx, level.slogLevel())
}

// Slog returns the underlying *slog.Logger. Prefer the Logger methods
// above for new code; this escape hatch exists for interoperating with
// third-party libraries that expect a *slog.Logger directly.
func (l *Logger) Slog() *slog.Logger {
	return l.slog
}
