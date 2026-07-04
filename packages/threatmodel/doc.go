// Package threatmodel is Phase 083: systematically reducing this
// platform's attack surface by cataloguing, in a structured
// STRIDE-style form, what can go wrong per named platform component
// and what already-shipped (or newly added) control mitigates it, then
// adding the concrete hardening primitives the catalogue's own entries
// point back to. It draws on the durable, hash-chained audit trail
// added in Phase 077 (packages/auditlog), the role/permission model
// added in Phase 006 (packages/identity), the non-binding-analysis
// guardrail added in Phase 057 (packages/guardrail), the API gateway
// added in Phase 009 (packages/gateway), the safe-variable-injection
// prompt registry added in Phase 016 (packages/prompts), the
// ingestion pipeline's transcribe-and-discard/provenance guarantees
// added in Phases 019/029 (packages/intake, packages/ingestion), and
// the provider-locality residency guard added in Phase 078
// (packages/dataresidency), composing them into one hardening layer
// rather than duplicating any of them.
//
// # What is new here
//
//   - StrideCategory / Severity / MitigationStatus / Component /
//     Threat / Mitigation / ThreatModel (types.go): a structured,
//     STRIDE-style threat catalogue -- one ThreatModel per named
//     platform component/service (referenced by tag/name, e.g.
//     "gateway", "ingestion", "reasoning-orchestration" -- this
//     package never imports those packages just to name them, the
//     same MappedTo-by-string-tag convention
//     packages/compliance.Control and
//     packages/privacy.DataInventoryEntry.SourceTag established).
//     Each Threat carries a StrideCategory, a Severity, and one or
//     more Mitigations; each Mitigation carries a MitigationStatus and
//     a ReferenceTag naming, by convention, the real control that
//     implements it (e.g. "packages/identity.RequirePermission",
//     "packages/guardrail.CheckText") (task 1).
//   - SeedThreatModels (seed.go): a real starter catalogue covering
//     the API gateway (packages/gateway, Phase 009), the ingestion
//     pipeline (packages/intake / packages/ingestion, Phase 019/029),
//     and the reasoning orchestration path
//     (packages/reasoningorchestration, Phase 059) -- concrete, named
//     threats and mitigations, not placeholders (task 1).
//   - Validator / Sanitize functions (validate.go): a hardening
//     library of size-limit, charset/structure, and control-character
//     checks usable as a building block wherever untrusted input is
//     accepted -- distinct from, and does not duplicate,
//     packages/gateway's existing ValidateRequest/validateStruct
//     request-validation middleware (Phase 009), which this package
//     references rather than re-implements (task 2).
//   - DetectInjectionAttempt (injection.go): scans ingested text for
//     role-override phrases, instruction-injection markers, and
//     delimiter-breaking sequences before that text ever reaches an
//     LLM prompt, returning every Finding rather than a bare bool, so
//     a caller cannot silently ignore a partial match. This operates
//     one stage earlier than, and is composable with,
//     packages/prompts.SanitizeValue's own template-delimiter
//     injection guard (Phase 016, which rejects "{{"/"}}" once a value
//     is already being placed into a template variable) and
//     packages/ingestion's pipeline -- neither of which this package
//     duplicates (task 3).
//   - SanitizeOutput / VerifyGuardrailIntact (output.go):
//     output-handling safeguards applied to model output before it is
//     surfaced or persisted -- stripping/flagging unexpected control
//     sequences and confirming the non-binding guardrail label
//     survived intact, by calling through to
//     packages/guardrail.RequireLabel / packages/guardrail.CheckText
//     rather than reimplementing guardrail enforcement (task 4).
//   - GenerateSBOM (sbom.go): walks go.work and every listed package's
//     go.mod to emit a structured, CycloneDX-lite Software Bill of
//     Materials (module name + version list); a generated snapshot is
//     committed under doc/sbom.json (task 5).
//   - Container hardening checklist (container.go): since this
//     repository ships no Dockerfile today (verified by search), this
//     phase adds ContainerHardeningChecklist as a documented,
//     versioned policy type plus a reference hardened Dockerfile
//     template under doc/Dockerfile.hardened, rather than retrofitting
//     a container that does not exist (task 7).
//   - SegmentationPolicy / Zone (segmentation.go): a structured
//     network-segmentation policy -- named zones (e.g. public-gateway,
//     internal-services, data) and allow-rules between them, with real
//     IsAllowed/Validate evaluation logic, composing conceptually with
//     packages/dataresidency.CheckProviderLocality's provider-locality
//     guard (Phase 078) without importing or duplicating it (task 8).
//   - identity.PermViewThreatmodel / identity.PermManageThreatmodel
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every mitigation-status-change
//     operation on, following the exact
//     PermViewCompliance/PermManageCompliance precedent from Phase
//     082.
//   - AuditSink (audit.go): records every mitigation status transition
//     into packages/auditlog.Store -- the same durable, hash-chained
//     sink the rest of the platform already writes to and queries. No
//     second audit table.
//
// # Persistence: why this package skips a SQL migration
//
// Every other recent phase (081 privacy, 082 compliance) added a
// tenant-scoped Postgres migration. This phase deliberately does not,
// for a reason specific to what a ThreatModel actually is: a Control
// (packages/compliance) or DataInventoryEntry (packages/privacy)
// describes per-tenant operational state that changes as a deployment
// operates (evidence gets collected, a profile gets set). A
// ThreatModel describes an engineering artifact that changes when
// engineers change the system -- a threat is discovered, a mitigation
// ships, a severity gets re-assessed after a design review -- the same
// cadence and ownership model as the code itself. It is not
// tenant-scoped at all: the gateway's threat surface is identical for
// every tenant on a given deployment. Treating it as
// versioned-in-code data (SeedThreatModels, reviewed and merged via
// the exact same PR process as the mitigations it references) keeps
// the catalogue auditable through git history and code review --
// arguably a stronger, more tamper-evident record for "who approved
// this mitigation as sufficient and when" than an admin-editable table
// would be.
//
// The one part of this domain that does benefit from durable,
// queryable history is a *mitigation's status transition over time*
// (Planned -> Implemented -> Verified is an operational fact, not a
// code fact: "Verified" means someone actually checked, at a point in
// time, that the referenced control works). Rather than adding a new
// table (and a second history/audit mechanism) purely to track that,
// Engine.TransitionMitigation (engine.go) records every transition
// through the exact same AuditSink -> packages/auditlog.Store
// composition every other package in this codebase already uses --
// no new persistence primitive, no parallel audit trail, just this
// domain's mitigation-status history riding on infrastructure that
// already exists. See doc/threat-model.md for the full write-up.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/privacy's and packages/compliance's own
//     AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate
//     Engine.TransitionMitigation calls through authorizeManage before
//     doing anything threatmodel-specific.
//   - packages/gateway's ValidateRequest/validateStruct (Phase 009)
//     remains the only HTTP-request-body validation middleware in this
//     codebase; Validator/Sanitize here is a lower-level,
//     protocol-agnostic hardening building block, not a competing
//     middleware layer.
//   - packages/prompts.SanitizeValue/ErrInjectionAttempt (Phase 016)
//     remain the only template-rendering injection guard in this
//     codebase (it rejects "{{"/"}}" template delimiters once a value
//     is already being placed into a template variable);
//     DetectInjectionAttempt operates one stage earlier, on raw
//     ingested text before it is ever placed into a template variable,
//     and does not re-implement SanitizeValue's delimiter check.
//   - packages/guardrail.RequireLabel / CheckText / HasDisclaimer /
//     RequireDisclaimer (Phase 057) remain the only non-binding-
//     guardrail enforcement mechanism in this codebase --
//     purely output-side, not an ingestion/input-side control;
//     SanitizeOutput/VerifyGuardrailIntact call through them rather
//     than re-implementing any verdict-language or label check.
//   - packages/stt.Discard / packages/ocr.Discard (the actual
//     source-byte zeroing) and packages/ingestion.VerifyAudioDiscard /
//     VerifyImageDiscard (independent re-verification that the
//     zeroing happened) and packages/intake (Phases 019/029) remain
//     the only transcribe-and-discard/provenance machinery in this
//     codebase; the ingestion ThreatModel seed entries name them by
//     ReferenceTag, without importing any of those packages.
//   - packages/dataresidency.CheckProviderLocality (Phase 078) remains
//     the only cross-border/provider-locality guard in this codebase;
//     SegmentationPolicy is a structurally analogous but independent
//     zone-to-zone policy for network segmentation, not a dependency
//     on, wrapper around, or duplicate of dataresidency's logic.
//
// See doc/threat-model.md for the full write-up.
package threatmodel
