# packages/config

Typed, layered configuration for Verdex services, with no secrets in
code.

Module path: `github.com/YASSERRMD/verdex/packages/config`

## Why this package exists

Every Verdex service needs the same things from configuration:

- A single typed `Config` struct instead of scattered `os.Getenv` calls.
- Predictable precedence when the same setting is set in more than one
  place (a file, an environment variable, a per-deployment profile).
- A way to reference a secret (a database password, an API key) without
  ever writing its plaintext value into a YAML file or a git-tracked
  fixture.
- Validation that fails fast and loud at startup instead of producing a
  confusing runtime error five minutes later.
- A safe way to log the resolved configuration without leaking
  credentials.

This package addresses all five. It does **not** define any
model/LLM-provider configuration — `Config.Provider` is an intentionally
empty placeholder. Per `CONTRIBUTING.md`, provider selection routes
through the `LLMProvider` interface in `packages/provider`, owned by a
later phase.

## Config schema

```go
type Config struct {
    Deployment    DeploymentConfig
    Server        ServerConfig
    Database      DatabaseConfig
    Observability ObservabilityConfig
    Provider      ProviderConfig // placeholder, no fields yet
}
```

| Section         | Field              | Type            | YAML key            | Meaning                                            |
|-----------------|--------------------|-----------------|----------------------|-----------------------------------------------------|
| `Deployment`    | `Name`             | `string`        | `name`               | Human-readable instance identifier                  |
| `Deployment`    | `Environment`      | `string`        | `environment`         | `development`, `sandbox`, `production`, `airgapped`, ... |
| `Server`        | `Host`             | `string`        | `host`               | HTTP bind address                                   |
| `Server`        | `Port`             | `int`           | `port`               | HTTP bind port (1–65535)                            |
| `Server`        | `ReadTimeout`      | `time.Duration` | `read_timeout`       | Max time to read a request                          |
| `Server`        | `WriteTimeout`     | `time.Duration` | `write_timeout`      | Max time to write a response                        |
| `Server`        | `IdleTimeout`      | `time.Duration` | `idle_timeout`       | Max idle keep-alive duration                        |
| `Server`        | `ShutdownTimeout`  | `time.Duration` | `shutdown_timeout`   | Graceful shutdown deadline                          |
| `Database`      | `DSN`              | `string`        | `dsn`                | Connection string; **secret-bearing**, redacted in logs |
| `Database`      | `MaxOpenConns`     | `int`           | `max_open_conns`     | Max open DB connections                             |
| `Database`      | `MaxIdleConns`     | `int`           | `max_idle_conns`     | Max idle DB connections (≤ `MaxOpenConns`)          |
| `Database`      | `ConnMaxLifetime`  | `time.Duration` | `conn_max_lifetime`  | Max age of a reused connection                      |
| `Observability` | `LogLevel`         | `string`        | `log_level`          | `debug` \| `info` \| `warn` \| `error`              |
| `Observability` | `LogFormat`        | `string`        | `log_format`         | `json` \| `console`                                 |
| `Provider`      | —                  | —               | `provider`           | Empty placeholder; see `packages/provider` (later phase) |

`Default()` returns this struct populated with sane local-development
values (see `config.go`).

## Loading configuration

```go
cfg, err := config.NewLoader(
    config.WithFile("/etc/verdex/config.yaml"), // optional
    config.WithProfile("production"),            // optional, or use VERDEX_PROFILE
).Load()
if err != nil {
    log.Fatalf("config: %v", err)
}
```

`Load()` runs every layer, resolves secret references, validates the
result, and returns either a fully-populated `Config` or a wrapped
error explaining exactly what went wrong (missing file, malformed
YAML, bad env var, unresolved secret, or a validation failure).

### Precedence

Layers are applied in this order, each one only overwriting the fields
it explicitly sets:

1. **`Default()`** — built-in fallback values.
2. **YAML file** (`WithFile`) — optional base config file.
3. **Profile overlay** (`WithProfile` / `VERDEX_PROFILE`) — optional,
   additional YAML layered on top of the base file.
4. **Environment variables** — always considered; always wins.

Because each layer only touches the keys it mentions, the merge is
field-level: a value set in the YAML file but never mentioned by an
env var survives unchanged, and a profile can override one field while
leaving everything else from the base file alone.

## Environment variable convention

Every recognized variable is prefixed `VERDEX_`. Nested struct fields
are joined with underscores, converting the Go field name to
`SCREAMING_SNAKE_CASE`:

```
VERDEX_SERVER_PORT=9090
VERDEX_SERVER_READ_TIMEOUT=2s
VERDEX_DEPLOYMENT_NAME=verdex-api
VERDEX_DATABASE_DSN=env://VERDEX_DATABASE_DSN_VALUE
```

Supported field kinds: `string`, integers, `bool`, and `time.Duration`
(parsed with `time.ParseDuration`, e.g. `2s`, `500ms`, `1h30m`).

A field can opt out of env loading with `env:"-"`, or use an explicit
override name with `env:"CUSTOM_NAME"`.

This mapping is implemented via reflection in `env.go`
(`loadEnv`/`loadEnvStruct`/`toScreamingSnakeCase`) rather than a fixed
table, so newly added `Config` fields are automatically loadable from
the environment without touching `env.go`.

## YAML file layer

Any YAML file matching the `Config` shape (see field table above) can
be passed via `WithFile`. A documented, fully-populated example lives
at [`testdata/config.example.yaml`](./testdata/config.example.yaml).
Only the keys present in the file are applied — omitted keys keep
whatever the previous layer set.

## Secret reference syntax

Any **string** field (most usefully `database.dsn`) may hold a secret
reference instead of a literal value. References are resolved as the
final step of `Load()`, after the defaults/YAML/profile/env merge.

| Scheme  | Example                          | Resolution                                                                 |
|---------|-----------------------------------|------------------------------------------------------------------------------|
| `env://`   | `env://VERDEX_DATABASE_DSN_VALUE`   | Looks up the named OS environment variable. **Fails loudly** (returns an error) if it is unset — a silently-empty secret is treated as a bug, not a fallback. |
| `vault://` | `vault://secret/data/verdex#dsn`    | Routed to `config.VaultResolver`, which is a **documented no-op placeholder**. There is no real HashiCorp Vault (or any other secret-store) integration in this phase. Every `vault://` resolution currently returns an error explaining that the backend is unimplemented, so a misconfigured deployment fails at startup instead of silently using an empty secret. |

You can supply your own resolver (e.g. a real Vault client once one
exists) via `config.WithSecretResolver`, which must implement:

```go
type SecretResolver interface {
    Resolve(ref string) (string, error)
}
```

`ref` is the full reference including its scheme (e.g.
`"vault://secret/data/verdex#dsn"`); a `SecretResolverFunc` adapter is
provided for simple cases.

**Never commit a literal secret, including in test fixtures.** Use an
`env://` reference pointing at a clearly-fake variable name, e.g.
`env://TEST_DB_PASSWORD`, as this package's own tests do.

## Validation

`Config.Validate() error` runs as the last step of `Load()` and checks:

- `deployment.name` / `deployment.environment` are non-empty.
- `server.host` is non-empty and `server.port` is in `[1, 65535]`.
- All four server timeouts are strictly positive.
- `database.max_open_conns >= 1`, `database.max_idle_conns >= 0` and
  `<= max_open_conns`, `database.conn_max_lifetime >= 0`.
- `observability.log_level` is one of `debug|info|warn|error`.
- `observability.log_format` is one of `json|console`.

All violations are collected and joined (via `errors.Join`) into a
single error, so a misconfigured deployment sees every problem at
once rather than fixing them one at a time. `Loader.Load()` wraps the
result with additional context; calling `Validate()` directly returns
the unwrapped joined error.

## Redaction for logging

Never log or print a raw `Config` — use `Redacted()` or `String()`:

```go
log.Printf("loaded config: %s", cfg) // safe: uses Config.String()

safeCopy := cfg.Redacted() // safe: explicit redacted copy
```

Fields are marked sensitive with a `redact:"true"` struct tag (see
`database.dsn` in `config.go`). `Redacted()` returns a copy with every
tagged, non-empty string field replaced by the fixed placeholder
`[REDACTED]`; it never mutates the receiver. `String()` JSON-encodes
the redacted copy, so passing a `Config` value directly to `%s`,
`fmt.Println`, or a structured logger never leaks a credential —
whether that credential came from a literal value or from a resolved
secret reference, since redaction happens after resolution.

To mark a new field as sensitive, add the tag:

```go
type FooConfig struct {
    APIKey string `yaml:"api_key" redact:"true"`
}
```

## Per-deployment profiles

A profile is a named, optional YAML overlay applied after the base
file but before environment variables (so env still wins over a
profile, and a profile still wins over the base file). Select one via:

```go
config.WithProfile("production")
```

or by setting `VERDEX_PROFILE=production` in the environment (an
explicit `WithProfile` call always takes precedence over the env var).

The overlay file is read from `<profileDir>/<profile>.yaml`, where
`profileDir` defaults to a `profiles/` directory next to the base
config file (override with `config.WithProfileDir`). Selecting a
profile name with no matching file is an error — a typo should fail
loudly at startup, not silently fall back to base config.

Example profiles ship under
[`testdata/profiles/`](./testdata/profiles/): `sandbox.yaml`,
`production.yaml`, and `airgapped.yaml`. Copy that layout into a real
deployment's config directory to add your own.

## Testing

```sh
cd packages/config
go test ./...
```

Tests are table-driven and cover, among other things: defaults-only,
YAML-only, env-only, YAML+env precedence (env wins), secret resolution
success/failure, redaction correctness, and profile layering. See
`precedence_test.go` for the consolidated cross-layer matrix, and the
`*_test.go` file next to each layer's implementation for
scenario-specific edge cases.
