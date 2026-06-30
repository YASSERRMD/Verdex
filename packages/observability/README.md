# packages/observability

Structured logging, correlation IDs, metrics, tracing, health/readiness
endpoints, an audit log channel, and log redaction for Verdex
services.

Module path: `github.com/YASSERRMD/verdex/packages/observability`

## Why this package exists

Every Verdex service needs the same cross-cutting observability
primitives, applied consistently: a structured logger instead of
scattered `fmt.Println`/`log.Printf` calls, a request correlation ID
that survives a call from inbound header through every log line a
request produces, metrics in a standard exposition format, traced
spans for cross-service work, liveness/readiness contracts a deployer
can rely on, an audit trail kept separate from debug noise, and a
safety net against accidentally logging PII.

Services depend on this package rather than importing `log/slog`,
`github.com/prometheus/client_golang`, or
`go.opentelemetry.io/otel` directly, so call sites stay decoupled from
the specific backend in use.

## Structured logging

`Logger` wraps `log/slog` behind a small type with four level methods
and a `With` for attaching persistent fields:

```go
logger := observability.New(
    observability.WithLevel(observability.LevelInfo),
    observability.WithFormat(observability.FormatJSON), // or FormatConsole
    observability.WithOutput(os.Stdout),                 // default
)

logger.Info(ctx, "case opened", "case_id", caseID, "jurisdiction", "US-CA")

// Attach fields that should appear on every subsequent log line from
// this logger (e.g. once you know the tenant for a request):
scoped := logger.With("tenant_id", tenantID)
scoped.Warn(ctx, "rate limit approaching", "remaining", 3)
```

Supported levels: `LevelDebug`, `LevelInfo`, `LevelWarn`, `LevelError`
(parse from a string with `ParseLevel`). Supported formats:
`FormatJSON` (one JSON object per record; the default) and
`FormatConsole` (human-readable text, for local development; parse
from a string with `ParseFormat`, which also accepts `"text"` as an
alias for console).

## Correlation IDs and the HTTP middleware

A correlation ID identifies a single logical request as it flows
through a service (and, by propagating the header, across services).

```go
mux := http.NewServeMux()
mux.HandleFunc("/widgets", handleWidgets)

handler := observability.CorrelationMiddleware(baseLogger)(mux)
http.ListenAndServe(":8080", handler)
```

`CorrelationMiddleware`:

1. Reads the inbound `X-Correlation-ID` header (`observability.CorrelationIDHeader`).
2. Generates a new one (UUID v4, via `NewCorrelationID`) if it is
   missing or empty.
3. Stores the ID on the request's `context.Context` and sets it on the
   response header before any bytes are written.
4. Binds a per-request child logger (`baseLogger.With("correlation_id", id)`)
   to the context.

Inside a handler, pull the request-scoped logger back out with
`FromContext` so every log line you emit automatically carries the
correlation ID:

```go
func handleWidgets(w http.ResponseWriter, r *http.Request) {
    logger := observability.FromContext(r.Context(), baseLogger)
    logger.Info(r.Context(), "listing widgets")
    // ...
}
```

`FromContext` falls back gracefully: if no logger was explicitly bound
(e.g. outside the middleware) but a correlation ID is present on the
context, it returns `fallback.With("correlation_id", id)`; if neither
is present, it returns `fallback` unchanged.

For non-HTTP code paths (background jobs, queue consumers),
`EnsureCorrelationID(ctx)` returns a context guaranteed to carry an ID,
generating one if absent.

## Metrics

`Registry` is a small interface over counters, gauges, and histograms
with label support, implemented over an isolated
`*prometheus.Registry`:

```go
registry := observability.NewRegistry()

requestsTotal := registry.Counter(
    "verdex_http_requests_total", "Total HTTP requests handled", "method", "status",
)
requestsTotal.Inc("GET", "200")

inFlight := registry.Gauge("verdex_http_inflight_requests", "In-flight HTTP requests")
inFlight.Inc()
defer inFlight.Dec()

latency := registry.Histogram(
    "verdex_http_request_duration_seconds", "Request latency", nil, "route",
)
latency.Observe(0.042, "/widgets")

mux.Handle("/metrics", registry.Handler())
```

`Registry.Handler()` returns an `http.Handler` exposing every metric
registered through that `Registry` in Prometheus exposition format,
ready to mount at `/metrics`. Each `Registry` returned by `NewRegistry`
is backed by its own isolated `prometheus.Registry` — never the global
default — so multiple registries (e.g. one per test) never collide.

## Tracing

`Tracer`/`Span` abstract OpenTelemetry's Go SDK behind a minimal
interface:

```go
provider := observability.NewNoopTracerProvider() // discards spans; safe default
// or, in tests:
provider, recorder := observability.NewInMemoryTracerProvider()
defer provider.Shutdown(ctx)

tracer := provider.Tracer("verdex.widgets")

ctx, span := tracer.Start(ctx, "widgets.list", observability.String("tenant_id", tenantID))
defer span.End()

if err != nil {
    span.RecordError(err)
}
span.SetAttributes(observability.Int("result_count", len(results)))
```

No real OTLP collector is required for this phase. `NewNoopTracerProvider`
creates and ends real spans (so call sites behave identically in every
environment) but discards them. `NewInMemoryTracerProvider` instead
wires an in-memory `SpanRecorder` — call `recorder.Spans()` in a test
to assert on the span names, attributes, status, and parent/child trace
relationships that were recorded, with no live infrastructure needed.

## Health and readiness endpoints

```go
mux.Handle("/healthz", observability.LivenessHandler())

mux.Handle("/readyz", observability.ReadinessHandler(
    observability.NamedChecker{Name: "database", Checker: pingDB},
    observability.NamedChecker{Name: "cache", Checker: pingCache},
))
```

- `/healthz` (liveness): always responds `200 OK` with
  `{"status":"ok"}` as long as the process can serve HTTP at all. It
  never inspects external dependencies — liveness answers "is this
  process alive", not "is this process useful".
- `/readyz` (readiness): runs every registered `Checker` (a
  `func(context.Context) error`) against the incoming request's
  context. If all succeed, responds `200 OK`. If any fail, responds
  `503 Service Unavailable` with a JSON body listing every failure by
  name:

  ```json
  {"status":"unavailable","failures":{"database":"connection refused"}}
  ```

  The checker list is pluggable so later phases can register real DB,
  cache, or downstream-service connectivity checks without changing
  this contract.

## Audit log channel

`AuditLogger` is a **separate** structured log stream from the
application `Logger` — a deliberate design choice so audit records can
be routed, retained, and access-controlled independently of debug/info
application noise:

```go
auditFile, _ := os.OpenFile("/var/log/verdex/audit.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
auditLogger := observability.NewAuditLogger(auditFile)

auditLogger.Log(ctx, observability.AuditEvent{
    Actor:   "user:" + userID,
    Action:  "case.viewed",
    Target:  "case:" + caseID,
    Outcome: "success",
})
```

`AuditEvent` is intentionally minimal: `Time`, `Actor`, `Action`,
`Target`, `Outcome`. This establishes the "audit events live on their
own channel" contract; it is **not** the full audit system. Phase 077
owns the complete audit trail (richer event taxonomy, retention and
tamper-evidence guarantees, query interfaces) and is expected to
extend this type rather than this phase building it out prematurely.

`AuditLogger` always writes JSON regardless of the application
Logger's configured format, since audit records are meant for
downstream machine processing, not console readability. If
`AuditEvent.Time` is left as the zero value, `Log` stamps it with
`time.Now().UTC()`.

## Log redaction (defense in depth, not a substitute for care)

Redaction here is a **best-effort safety net**. It catches obvious
mistakes — a sensitive field landing in a log call, an email address
or credential-shaped string in a free-text message — but it is not a
PII detection system and must never be relied upon as the only
safeguard. The first line of defense is still: don't log sensitive
data.

Three mechanisms, layered:

1. **Explicit field wrapping** — `observability.Redact(value)` wraps a
   single sensitive value passed as a structured log field. It
   implements `slog.LogValuer`, so any `Logger` call automatically
   renders it as `[REDACTED]`, never the underlying value:

   ```go
   logger.Info(ctx, "user updated", "user_id", id, "ssn", observability.Redact(ssn))
   ```

2. **Struct-tag convention** — consistent with `packages/config`'s
   `redact:"true"` tag. `RedactStruct(v)` returns a redaction-safe copy
   of a struct (or pointer to one), blanking every exported string
   field tagged `redact:"true"`, recursing into nested structs, and
   never mutating the original:

   ```go
   type UpdateRequest struct {
       UserID string
       Email  string `redact:"true"`
   }

   logger.Info(ctx, "request received", "body", observability.RedactStruct(req))
   ```

3. **Pattern-based backstop** — `RedactString(s)` scans free-text for
   email-looking and credential-looking (`key=value` or `key: value`
   for names like `password`, `api_key`, `token`, `secret`)
   substrings and replaces them with `[REDACTED]`. Use this as a last
   resort over message strings assembled from less-trusted input, not
   as a primary control.

## Wiring the logger to `packages/config`

```go
cfg, err := config.NewLoader(config.WithFile("/etc/verdex/config.yaml")).Load()
if err != nil {
    log.Fatalf("config: %v", err)
}

logger, err := observability.NewLoggerFromConfig(&cfg)
if err != nil {
    log.Fatalf("observability: %v", err)
}
```

`NewLoggerFromConfig` parses `cfg.Observability.LogLevel` and
`cfg.Observability.LogFormat` (via `ParseLevel`/`ParseFormat`) and
constructs a `Logger` accordingly; pass additional `Option`s (e.g.
`WithOutput` in a test) to override behavior beyond what the config
section controls. Because this couples `packages/observability` to
`packages/config`, this module's `go.mod` carries a `replace
github.com/YASSERRMD/verdex/packages/config => ../config` directive,
since neither module is published — both are resolved locally via the
root `go.work`.

## Testing

```sh
cd packages/observability
go test ./...
```

Tests are table/scenario-driven and cover, among other things: level
and format parsing, JSON/console log output shape, level filtering,
`With`-scoped fields, the full correlation-ID middleware request
lifecycle (missing header generates one, existing header is preserved,
response header is set, a context-derived logger carries the ID),
metrics scrape output, span recording (including parent/child trace
linkage and error status), liveness/readiness handler status codes and
bodies, audit log channel separation, redaction of tagged and
pattern-matched values, and config-driven logger construction.

See `middleware_test.go` for the consolidated correlation-ID
integration tests, and the `*_test.go` file next to each
implementation file for scenario-specific unit coverage.
