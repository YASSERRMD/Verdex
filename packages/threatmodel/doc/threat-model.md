# Threat modeling & hardening (Phase 083)

This phase systematically reduces this platform's attack surface: it
catalogues, in a structured STRIDE-style form, what can go wrong per
named platform component and what already-shipped (or newly added)
control mitigates it, then adds the concrete hardening primitives the
catalogue's own entries point back to. It composes six earlier
threads -- the API gateway added in Phase 009 (`packages/gateway`), the
safe-variable-injection prompt registry added in Phase 016
(`packages/prompts`), the ingestion pipeline's transcribe-and-discard/
provenance guarantees added in Phases 019/029 (`packages/intake`,
`packages/ingestion`), the non-binding guardrail added in Phase 057
(`packages/guardrail`), the provider-locality residency guard added in
Phase 078 (`packages/dataresidency`), and the durable audit trail added
in Phase 077 (`packages/auditlog`) -- into one hardening layer rather
than duplicating any of them.

## Goal

Produce a real, per-component STRIDE threat catalogue; harden input
validation, prompt-injection defenses, and output handling; pin
dependencies and generate an SBOM; add secrets-scanning as an enforced
CI gate; add a container-hardening policy; and add a network-
segmentation policy -- across the whole platform, not just this new
package.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/gateway` (Phase 009) | `ValidateRequest`/`validateStruct` HTTP request-body validation; `RateLimitMiddleware`; `Timeout` | `Validator`/`Sanitize` (validate.go): a lower-level, protocol-agnostic hardening library for untrusted strings below the HTTP layer -- referenced by the gateway `ThreatModel`'s seed entries, not duplicated |
| `packages/prompts` (Phase 016) | `SanitizeValue`/`ErrInjectionAttempt`: rejects `{{`/`}}` template delimiters once a value is already being placed into a rendered prompt | `DetectInjectionAttempt` (injection.go): screens arbitrary ingested text one stage earlier, for role-override phrases, fake instruction markers, delimiter-breaking sequences, and data-exfiltration attempts |
| `packages/intake` / `packages/ingestion` (Phase 019/029) | `stt.Discard`/`ocr.Discard` (source-byte zeroing); `ingestion.VerifyAudioDiscard`/`VerifyImageDiscard` (independent re-verification); `packages/provenance.ProvenanceRecord` (chain-of-custody hash) | The ingestion `ThreatModel`'s seed entries name these by `ReferenceTag`; `DetectInjectionAttempt` is the new mitigation for the "injected text reaches an LLM prompt unexamined" threat |
| `packages/guardrail` (Phase 057) | `RequireLabel`/`CheckText`/`RequireDisclaimer`/`HasDisclaimer`: the only non-binding-guardrail enforcement in this codebase, purely output-side | `SanitizeOutput`/`VerifyGuardrailIntact` (output.go): output-handling safeguards that call through to `guardrail.RequireLabel`/`CheckText` for a defensive re-check immediately before output is surfaced or persisted, rather than reimplementing either |
| `packages/dataresidency` (Phase 078) | `ResidencyPolicy`/`CheckProviderLocality`: the provider-locality guard governing which geographic *regions* tenant data may live in | `SegmentationPolicy`/`Zone` (segmentation.go): a structurally analogous but independent policy governing which network *zones* may talk to which other zones -- region policy and network-zone policy are different concerns that compose, not duplicate |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store | `AuditSink` records every mitigation status transition into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewThreatmodel`/`PermManageThreatmodel`: the two fine-grained permissions this package's `Engine` gates every mitigation-transition operation on |

## STRIDE threat catalogue (task 1)

`ThreatModel`/`Component`/`Threat`/`Mitigation` (types.go) form a
structured, STRIDE-style catalogue: one `ThreatModel` per named
platform component/service, referenced by tag/name (e.g. `"gateway"`,
`"ingestion"`, `"reasoning-orchestration"`) rather than by importing
those packages -- the same `MappedTo`-by-string-tag convention
`packages/compliance.Control` and
`packages/privacy.DataInventoryEntry.SourceTag` established. Each
`Threat` carries a `StrideCategory` (a closed enum -- Spoofing,
Tampering, Repudiation, Information Disclosure, Denial of Service,
Elevation of Privilege -- since STRIDE is a fixed taxonomy, unlike
`Framework`'s deliberately open set), a `Severity`, and one or more
`Mitigation`s, each with a `MitigationStatus` (Planned, Implemented,
Verified) and a `ReferenceTag` naming the real control that implements
it by string convention (e.g. `"packages/identity.RequirePermission"`,
`"packages/guardrail.CheckText"`).

`SeedThreatModels` (seed.go) returns a real starter catalogue covering
three components:

- **`gateway`** (`packages/gateway`, Phase 009): identity spoofing via
  forged tokens, oversized/malformed request DoS, request-flood DoS,
  and cross-tenant data leakage via a manipulated tenant identifier --
  mitigated by `identity.RequirePermission`, this package's own
  `ValidateSize`, `gateway.RateLimitMiddleware`/`Timeout`, and
  `tenancy.WithTenantScope`/Row-Level Security respectively.
- **`ingestion`** (`packages/intake`/`packages/ingestion`, Phase
  019/029): a retained source binary surviving past its required
  discard point, tampering between upload and extraction, a malicious
  file masquerading as an allowed MIME type, and an embedded
  prompt-injection payload reaching an LLM prompt unexamined --
  mitigated by `stt.Discard`/`ingestion.VerifyAudioDiscard`,
  `provenance.ProvenanceRecord`, `intake.DetectMIME`, and this
  package's own `DetectInjectionAttempt`/`prompts.SanitizeValue`
  respectively.
- **`reasoning-orchestration`** (`packages/reasoningorchestration`,
  Phase 059): synthesized output losing the mandatory non-binding
  guardrail label, verdict/directive language surviving despite the
  label being present, a mid-pipeline failure silently treated as a
  completed run, and a single tenant's run exhausting shared capacity
  -- mitigated by `guardrail.RequireLabel`/`CheckText`, this package's
  own `VerifyGuardrailIntact`, `reasoningorchestration.RunState`, and
  `reasoningorchestration.PipelineBudget` respectively.

Every threat is concrete and named, not a placeholder; every
`Mitigation` names a real, existing (or newly added) control.

## Input-validation hardening library (task 2)

`Validator`/`Sanitize` (validate.go) is a real, testable hardening
library: `ValidateSize` (byte-length ceiling), `ValidateCharset`
(rejects disallowed control characters), `SanitizeControlChars` (the
strip-rather-than-reject counterpart), `ValidateNonBlank`,
`ValidateMaxRunes`, and a combined `Validate(s, ValidatorOptions)` entry
point. This is deliberately lower-level and protocol-agnostic compared
to `packages/gateway.ValidateRequest`/`validateStruct` (Phase 009),
which already handles HTTP-request-body JSON decoding and
`validate:"required"` struct-tag checking -- this package does not
duplicate that middleware, and exists for the many places untrusted
strings enter the system below the HTTP layer (ingested document text,
prompt template values, mitigation reference tags, and so on).

## Prompt-injection defenses on ingestion (task 3)

`DetectInjectionAttempt(text string) (bool, []Finding)` (injection.go)
scans ingested text for four categories of suspicious pattern before
that text ever reaches an LLM prompt: role-override phrases ("ignore
previous instructions"), fake instruction/turn markers (`system:`,
`[INST]`, `<|system|>`), delimiter-breaking sequences (`</document>`,
"----- end of instructions -----"), and data-exfiltration attempts
("reveal your system prompt"). It returns every `Finding` located, not
just the first, so a caller cannot silently ignore a partial match.
Patterns are case-insensitive and tolerant of common obfuscation, and
are tested both against real attack phrasing and against legitimate
case-document prose (references to "prior rulings," "security
systems," "the court's instructions") to guard against false
positives.

This operates one stage earlier than, and composes with,
`packages/prompts.SanitizeValue`'s own template-delimiter injection
guard (Phase 016), which only rejects a value once it is already being
placed into a rendered prompt's template variable.

## Output-handling safeguards (task 4)

`SanitizeOutput(raw string) SanitizedOutput` (output.go) strips
disallowed control characters from model output and screens the
cleaned text for injection-style patterns that may indicate the model
echoed back or complied with an injected instruction -- output that
"complies" with an injected instruction is itself worth flagging, not
just ingestion-time input.

`VerifyGuardrailIntact(label, text string) GuardrailVerification` is a
defensive re-verification of the two guarantees
`packages/reasoningorchestration`'s own pipeline-stage guardrail check
already enforces once, applied again immediately before output is
surfaced or persisted: it calls `guardrail.RequireLabel` and
`guardrail.CheckText` directly, never reimplementing either check, so
a mutation occurring after the pipeline's own check (a lossy
serialization round-trip, a downstream transform) cannot silently drop
the label or reintroduce verdict language without this second check
catching it.

## SBOM / dependency pinning (task 5)

`GenerateSBOM(workspaceRoot string) (SBOM, error)` (sbom.go) walks
`go.work`'s `use` directives and every listed package's `go.mod`
`require` block to emit a structured, CycloneDX-lite Software Bill of
Materials: one `SBOMComponent` per distinct module (first-party
application modules and third-party libraries), each with every
observed version and which package(s) declared it, deduplicated and
sorted for deterministic output. Indirect (`// indirect`) requires and
sibling first-party requires are excluded -- an SBOM's purpose is
auditing genuine direct dependencies, and every direct dependency's
own transitive graph is already covered by that dependency's own SBOM.
It parses `go.work`/`go.mod` with a small dependency-free line
scanner rather than importing `golang.org/x/mod/modfile`, since this
repository does not otherwise depend on that module and the syntax
involved is simple enough not to warrant a new dependency purely to
parse it.

`cmd/gensbom` regenerates the committed snapshot at `doc/sbom.json`;
run `go run ./packages/threatmodel/cmd/gensbom` from the repository
root to refresh it.

## Secrets-scanning in CI (task 6)

`gitleaks` was already wired into `.pre-commit-config.yaml` before
this phase, but a pre-commit hook is opt-in and only fires for
contributors who have it installed locally. This phase adds an
explicit `secrets-scan` job to `.github/workflows/ci.yml` using the
official `gitleaks/gitleaks-action@v2`, and makes it a required input
to the `gate` job alongside the existing build/test checks -- defense
in depth: pre-commit catches a secret before it is ever committed
locally; CI catches it regardless of whether that hook ran. This phase
does not reinvent secrets-scanning logic; it makes the existing tool
an enforced gate rather than an optional one.

## Container hardening (task 7)

A repository-wide search (`find . -iname "Dockerfile*"`) confirms
Verdex ships no Dockerfile today -- every package/service currently
runs as a plain Go binary with no containerization step. Rather than
retrofitting hardening onto a container that does not exist, this
phase adds `ContainerHardeningChecklist` (container.go): a versioned,
policy-as-code set of rules (non-root user, minimal/pinned base image,
no unnecessary packages, no embedded secrets, read-only root
filesystem, no privileged capabilities), with heuristic
`Satisfied`/`Unsatisfied` evaluation against raw Dockerfile content.
`doc/Dockerfile.hardened` is a committed reference template a future
phase adding the first Dockerfile can start from; a test confirms the
template itself actually satisfies every automatically-checkable rule
in the default checklist.

## Network segmentation policy (task 8)

`SegmentationPolicy`/`Zone`/`SegmentationRule` (segmentation.go) model
a default-deny-unless-listed network topology: `IsAllowed(fromZone,
toZone, port)` and `Validate` enforce that every zone pair -- including
a zone and itself -- must be explicitly permitted by a
`SegmentationRule`, so a policy author cannot rely on an implicit
same-zone-trusts-itself default they never actually wrote down.
`DefaultSegmentationPolicy` seeds the platform's real three-zone
shape:

- **`public-gateway`**: the internet-facing API gateway
  (`packages/gateway`) -- the only zone reachable from outside the
  deployment.
- **`internal-services`**: every internal backend package (ingestion,
  reasoning orchestration, and so on) not directly internet-facing.
- **`data`**: the database and any durable store -- the most
  restricted zone.

The single most important invariant this policy encodes is an
*absent* rule: the public gateway can never reach the data zone
directly, on any port, bypassing internal services entirely --
`TestDefaultSegmentationPolicy_GatewayCannotReachDataDirectly` asserts
this explicitly. This is structurally analogous to, but independent
of, `packages/dataresidency.ResidencyPolicy`/`CheckProviderLocality`
(Phase 078): residency policy governs which geographic *regions*
tenant data may live in; segmentation policy governs which network
*zones* may talk to which other zones. The two compose conceptually
-- a deployment can have both active at once -- but this package does
not import `packages/dataresidency`.

## Tests for hardening controls (task 9)

Every file above has a corresponding `_test.go` with real, non-trivial
assertions: real attack phrasing alongside legitimate case-document
prose for `DetectInjectionAttempt` (guarding against false positives,
not just false negatives); an integration test asserting
`GenerateSBOM` actually lists `github.com/google/uuid` (a genuine
dependency of this very package) and correctly excludes a known
indirect-only dependency; an integration test asserting the committed
`doc/Dockerfile.hardened` template itself satisfies
`DefaultContainerHardeningChecklist`; and the segmentation policy's
gateway-cannot-reach-data invariant. `engine_test.go` covers
authentication, permission, tenant-isolation, illegal-transition, and
audit-recording paths for `Engine.TransitionMitigation`/
`ResetMitigation`.

## Access control

Two new `identity.Permission` constants gate every `Engine`
mitigation-transition operation, added following `permission.go`'s
exact `PermViewCompliance`/`PermManageCompliance` precedent from Phase
082:

- `threatmodel:view` (`identity.PermViewThreatmodel`): read-only
  access to the STRIDE threat catalogue and a mitigation's recorded
  status-transition history.
- `threatmodel:manage` (`identity.PermManageThreatmodel`): transition a
  catalogued `Mitigation`'s status.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Persistence: why this phase adds no SQL migration

Every other recent phase (081 privacy, 082 compliance) added a
tenant-scoped Postgres migration. This phase deliberately does not.

A `Control` (`packages/compliance`) or `DataInventoryEntry`
(`packages/privacy`) describes per-tenant operational state that
changes as a deployment operates -- evidence gets collected, a profile
gets set. A `ThreatModel` describes an engineering artifact that
changes when engineers change the system: a threat is discovered, a
mitigation ships, a severity gets re-assessed after a design review --
the same cadence and ownership model as the code itself. It is not
tenant-scoped at all: the gateway's threat surface is identical for
every tenant on a given deployment. Treating it as versioned-in-code
data (`SeedThreatModels`, reviewed and merged via the exact same PR
process as the mitigations it references) keeps the catalogue
auditable through git history and code review -- arguably a stronger,
more tamper-evident record for "who approved this mitigation as
sufficient and when" than an admin-editable table would be.

The one part of this domain that does benefit from durable, queryable
history is a *mitigation's status transition over time* (Planned ->
Implemented -> Verified is an operational fact, not a code fact:
"Verified" means someone actually checked, at a point in time, that
the referenced control works). Rather than adding a new table (and a
second history/audit mechanism) purely to track that,
`Engine.TransitionMitigation`/`ResetMitigation` record every
transition through the exact same `AuditSink` ->
`packages/auditlog.Store` composition every other package in this
codebase already uses -- no new persistence primitive, no parallel
audit trail, just this domain's mitigation-status history riding on
infrastructure that already exists.

`Catalogue` (engine.go) is the in-process index a `Engine` holds over
a `[]ThreatModel` -- constructed via
`NewCatalogue(SeedThreatModels())` for the platform's starter set --
letting a `Mitigation` be looked up and updated by ID without a
database. There is no `Repository`/`PostgresXRepository`/
`TenantScopedXRepository` three-layer set in this package, unlike
`packages/compliance`/`packages/privacy`, because there is no
persistence layer for `Catalogue` to sit on top of.

## What is explicitly reused, not duplicated

- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission` remain
  the coarse RBAC gate `Engine.TransitionMitigation`/`ResetMitigation`
  call through before doing anything threatmodel-specific.
- `packages/gateway.ValidateRequest`/`validateStruct` (Phase 009)
  remains the only HTTP-request-body validation middleware in this
  codebase; `Validator`/`Sanitize` here is a lower-level,
  protocol-agnostic building block, not a competing middleware layer.
- `packages/prompts.SanitizeValue`/`ErrInjectionAttempt` (Phase 016)
  remains the only template-rendering injection guard in this
  codebase; `DetectInjectionAttempt` operates one stage earlier and
  does not re-implement `SanitizeValue`'s delimiter check.
- `packages/guardrail.RequireLabel`/`CheckText`/`HasDisclaimer`/
  `RequireDisclaimer` (Phase 057) remain the only non-binding-guardrail
  enforcement mechanism in this codebase -- purely output-side;
  `SanitizeOutput`/`VerifyGuardrailIntact` call through them rather
  than reimplementing either check.
- `stt.Discard`/`ocr.Discard` and `packages/ingestion.VerifyAudioDiscard`/
  `VerifyImageDiscard` (Phases 019/029) remain the only
  transcribe-and-discard/provenance machinery in this codebase; the
  ingestion `ThreatModel` seed entries name them by `ReferenceTag`,
  without importing any of those packages.
- `packages/dataresidency.CheckProviderLocality` (Phase 078) remains
  the only cross-border/provider-locality guard in this codebase;
  `SegmentationPolicy` is a structurally analogous but independent
  zone-to-zone policy for network segmentation, not a dependency on,
  wrapper around, or duplicate of `dataresidency`'s logic.
