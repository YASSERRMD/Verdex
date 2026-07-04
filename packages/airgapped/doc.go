// Package airgapped assembles Verdex's fully offline deployment tier
// for sensitive courts (Phase 079). It is, deliberately, mostly a
// composition layer: the residency, routing, key-management, and
// local-model pieces it depends on already exist from earlier phases;
// this package is the glue that turns them into one coherent,
// validated, conformance-checked deployment profile plus the handful
// of genuinely new pieces (network policy, offline corpus/update/
// license flows) an air-gapped deployment still needs.
//
// # What is new here
//
//   - Profile (types.go): composes dataresidency.AirGappedPreset,
//     router.RoutingPolicy{AirGappedOnly: true}, and a mandated
//     keymanagement.FileProvider into one validated deployment
//     profile, keyed by a packages/config deployment-profile name
//     (ConfigProfileName) matching Phase 002's per-deployment config
//     profile pattern. Validate rejects a Profile whose components
//     are not actually configured for offline operation.
//   - GuardRegister / GuardRegistry / AuditRegistry
//     (providerguard.go): a real check that rejects registering any
//     non-"local:"-prefixed provider.LLMProvider under an air-gapped
//     Profile, and an audit pass over an existing provider.Registry.
//   - NetworkPolicy / GuardedDialContext / NewGuardedDialer
//     (networkpolicy.go): a real, testable allow-list guard (plus a
//     net.Dialer wrapper) that blocks any outbound address that is
//     neither loopback nor explicitly declared in
//     Profile.AllowedNetworkTargets.
//   - ProvisionCorpus / FileBundleStatuteLoader /
//     FileBundlePrecedentLoader (corpus.go): loads statute/precedent
//     data from a local bundle directory instead of a network fetch,
//     implementing statute.Loader and precedent.Loader by delegating
//     parsing to their own DefaultLoader.
//   - UpdateManifest / SignManifest / ApplyUpdateBundle (update.go):
//     a signed, checksum-verified local update bundle mechanism with
//     no network call, reusing a provenance.Signer-shaped
//     verification step (see Signer).
//   - LicenseKey / Activation / SignLicenseKey / Activate
//     (license.go): offline license/activation verified via the same
//     local Signer, with no license-server callout.
//   - VerifyZeroEgress (zeroegress.go): asserts every configured
//     provider/endpoint in a constructed list of EgressTargets
//     resolves to an allowed local/offline address under the
//     Profile's NetworkPolicy.
//   - Conformance (conformance.go): rolls up profile validity, the
//     provider allowlist, the network policy, and zero-egress
//     verification into one ConformanceReport.
//
// # What is explicitly reused, not duplicated
//
//   - packages/dataresidency's AirGappedPreset and
//     ComposeWithRouterAirGap remain the residency-policy side of
//     air-gap; Profile.Validate calls them directly rather than
//     re-deriving the same check.
//   - packages/router's RoutingPolicy.AirGappedOnly flag and
//     LocalOnlyEnforcer remain the routing-enforcement side; this
//     package never constructs or mutates a router.RoutingPolicy on
//     its own.
//   - packages/keymanagement's FileProvider remains the offline key
//     store; Profile only records that a live *FileProvider was
//     supplied (KeyProviderConfigured), it does not re-implement any
//     of FileProvider's file-backed, per-tenant, wrapped-at-rest key
//     storage.
//   - packages/adapters/local's LocalAdapter and OfflineModeEnforcer
//     remain the bundled local model runtime and its own BaseURL
//     guard; GuardRegister/NetworkPolicy compose alongside them (the
//     provider-ID prefix convention is shared, see
//     localProviderPrefix) rather than replacing them.
//   - packages/statute's and packages/precedent's Loader interfaces
//     and DefaultLoader remain the corpus grammar; ProvisionCorpus's
//     file-bundle loaders are pure pass-throughs to them.
//   - packages/provenance's signature primitives are consumed through
//     a narrow local Signer interface matching provenance.Signer's
//     method set exactly (so a *provenance.HMACSigner satisfies it
//     directly) rather than this package inventing new crypto for
//     update-bundle or license verification.
//
// See doc/airgapped-tier.md for the full deployment-tier writeup.
package airgapped
