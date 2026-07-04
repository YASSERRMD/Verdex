// Package dataresidency guarantees that a Verdex deployment's data
// stays within its declared jurisdiction boundaries. It is Phase 078:
// largely a policy/enforcement layer composed over existing pieces --
// packages/router's air-gapped/local-only flag (Phase 012),
// packages/provider's capability descriptor (Phase 011),
// packages/tenancy/packages/persistence's deployment concept, and
// packages/auditlog's centralized audit trail (Phase 077) -- not a new
// data plane.
//
// # What is new here
//
//   - ResidencyPolicy (types.go): AllowedRegions, DataClassRules, and
//     StrictMode, keyed by DeploymentID. Composes with
//     packages/tenancy's deployment concept the same way
//     packages/keymanagement.KeyMetadata composes with TenantID --
//     as a separate, keyed value, not an embedded field.
//   - RegionPin (region_pin.go): pins a deployment's expected storage
//     region and validates it against the live database DSN by
//     parsing the DSN with the same pgxpool.ParseConfig
//     packages/persistence.Open uses -- a real, testable check, not a
//     comment.
//   - CheckTransfer / CheckTransferForDataClass (transfer.go): guard
//     functions callable before any cross-region operation.
//   - CheckProviderLocality / FilterProvidersByLocality
//     (provider_check.go): guard functions over
//     provider.Capability.Region (the one additive field this phase
//     adds to packages/provider -- see types.go's Region doc comment
//     there).
//   - Verifier.Verify (verify.go): a startup/periodic check that the
//     live configuration (DB region, provider regions in use, air-gap
//     composition) actually satisfies the policy, returning a Report
//     enumerating every check performed.
//   - AirGappedPreset / ComposeWithRouterAirGap (types.go, airgap.go):
//     the policy-side preset for a fully air-gapped deployment
//     (StrictMode=true, AllowedRegions=[]), plus a check that it is
//     paired with router.RoutingPolicy.AirGappedOnly rather than
//     applied independently. Phase 079 builds the full offline
//     deployment tier around this preset; this phase stops at the
//     policy/flag composition.
//   - AuditSink (audit.go): records every transfer-check and
//     verification result via packages/auditlog.Store, reusing the
//     existing hash-chained audit trail rather than a new logging
//     channel.
//   - AlertSink / ViolationEvent (alert.go): a violation-alerting hook
//     mirroring packages/accounting.AlertSink's idiom, with a real
//     LoggingAlertSink implementation.
//   - Guard (guard.go): composes CheckTransfer + AuditSink + AlertSink
//     into one call so every check is recorded and every failure
//     alerts, without each call site wiring both by hand.
//
// # What is explicitly reused, not duplicated
//
//   - Provider locality enforcement checks provider.Capability.Region
//     against a policy; it does not invent a parallel provider-locality
//     registry. packages/router itself is unmodified -- callers (or a
//     future router change) invoke CheckProviderLocality /
//     FilterProvidersByLocality as a composable guard before or
//     alongside router's existing LocalOnlyEnforcer selector.
//   - Air-gapped mode composes with, rather than replaces,
//     router.RoutingPolicy.AirGappedOnly (packages/router/local_mode.go).
//   - Key material locality remains packages/keymanagement/
//     packages/encryption's concern; this package does not duplicate
//     key-residency logic.
//   - Alerting reuses the AlertSink shape packages/accounting and
//     packages/signoff already established, not a new alerting
//     primitive from packages/observability.
//
// See doc/data-residency.md for the full sovereignty model writeup.
package dataresidency
