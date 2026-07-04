# Air-gapped deployment tier (Phase 079)

## Goal

Fully offline deployment for sensitive courts: an air-gapped
deployment must never depend on a network call to operate correctly,
must only ever use a local model runtime, must provision and update
its legal corpus and its own configuration from a local bundle rather
than a network fetch, and must be able to prove -- with a single
pass/fail report -- that it actually satisfies all of the above.

`packages/airgapped` is deliberately **not** a new data plane or a
second router/residency system. Phase 078 (`packages/dataresidency`)
explicitly scoped its `AirGappedPreset` as "the policy-side preset";
this phase is the operational/deployment side it handed off.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/dataresidency` (Phase 078) | `AirGappedPreset`, `IsAirGappedPolicy`, `ComposeWithRouterAirGap` | `Profile.Validate` calls these directly; no residency logic is re-derived |
| `packages/router` (Phase 012) | `RoutingPolicy.AirGappedOnly`, `LocalOnlyEnforcer` (`local:`-prefix filtering) | `Profile.Validate` requires `AirGappedOnly` set; `GuardRegister`/`GuardRegistry`/`AuditRegistry` reuse the exact same `local:` prefix convention at the provider-registry level |
| `packages/keymanagement` (Phase 076) | `FileProvider`, `DeriveMasterKey` -- the file-backed, per-tenant, wrapped-at-rest offline key store | `Profile` mandates a live `*FileProvider` be supplied (`NewProfile`) and records it as configured; no key-storage logic is duplicated |
| `packages/adapters/local` (Phase 014) | `LocalAdapter` (OpenAI-compatible local runtime), `OfflineModeEnforcer` (checks the adapter's own `BaseURL`) | `NetworkPolicy` is a second, broader allow-list guard usable by *any* caller in the deployment, not just the local adapter's configured endpoint; both share the `local:` provider-ID convention |
| `packages/statute` / `packages/precedent` | `Loader` interface, `DefaultLoader` (JSON / structured-text corpus grammar) | `FileBundleStatuteLoader` / `FileBundlePrecedentLoader` implement the same `Loader` interfaces over a local bundle directory, delegating all parsing to the existing `DefaultLoader` |
| `packages/provenance` | `Signer` interface, `HMACSigner`, `SignRecord`/`VerifyRecord` canonical-payload convention | `Signer` (a narrow local interface matching `provenance.Signer`'s method set) is reused directly for update-bundle and license signature verification -- no new crypto |
| `packages/config` (Phase 002) | Named deployment-profile overlay pattern (`VERDEX_PROFILE`, `profiles/<name>.yaml`) | `ConfigProfileName = "airgapped"` is the profile name `Profile.Validate` requires a deployment to have selected |
| `packages/provider` | `Registry`, `LLMProvider`, `Capability` | `GuardRegister`/`GuardRegistry`/`AuditRegistry` wrap `Registry.Register`/`Registry.List` with an air-gapped-only check; `Registry` itself is unmodified |

### `packages/router` and `packages/dataresidency` are unmodified

Exactly as Phase 078 did for `packages/router`, this phase makes no
code changes to `packages/router` or `packages/dataresidency`. Every
check this package performs against those packages' types goes through
their existing exported API (`ComposeWithRouterAirGap`,
`IsAirGappedPolicy`, `RoutingPolicy.Validate`).

## The pieces

### `Profile`

```go
type Profile struct {
    DeploymentID          uuid.UUID
    ConfigProfile         string                        // must equal ConfigProfileName ("airgapped")
    Residency             dataresidency.ResidencyPolicy  // must be the AirGappedPreset shape
    Routing                router.RoutingPolicy          // must have AirGappedOnly = true
    KeyProviderConfigured bool                           // must be true (a live FileProvider was supplied)
    AllowedNetworkTargets []string                       // additional non-loopback allow-list entries
}
```

`NewProfile(deploymentID, residency, routing, keyProvider, allowedTargets)`
builds one from already-constructed pieces; `Validate()` rejects a
`Profile` whose residency policy is not air-gapped-shaped, whose
routing policy does not set `AirGappedOnly`, whose two policies are
not consistently composed (delegates to
`dataresidency.ComposeWithRouterAirGap`), or whose key provider was
never supplied.

### Provider allowlist: `GuardRegister` / `GuardRegistry` / `AuditRegistry`

`IsLocalProviderID` recognizes the same `"local:"` prefix
`packages/router.LocalOnlyEnforcer` and
`packages/adapters/local.LocalAdapter.ID()` already use.
`GuardRegister` rejects registering a non-local provider under an
air-gapped `Profile`; `GuardRegistry` composes that check with
`provider.Registry.Register` itself; `AuditRegistry` is the read-only
counterpart, scanning an already-populated registry for violations
(used by `Conformance`'s `CheckProviderAllowlist`).

### `NetworkPolicy` and the guarded dialer

`NetworkPolicy.CheckAddress` allows loopback/localhost unconditionally
and anything in `Profile.AllowedNetworkTargets` explicitly; everything
else returns `ErrDisallowedAddress`. `GuardedDialContext` wraps any
`DialContextFunc` (matching `net.Dialer.DialContext`'s signature) so a
disallowed address never reaches the real dialer; `NewGuardedDialer`
builds one backed by a real `net.Dialer`, suitable for
`http.Transport.DialContext`.

### Offline corpus provisioning: `ProvisionCorpus`

`ProvisionCorpus(ctx, bundlePath)` walks a local directory (or accepts
a single file) for `*.statute.json`/`*.statute.txt` and
`*.precedent.json`/`*.precedent.txt` files, reading each with
`FileBundleStatuteLoader`/`FileBundlePrecedentLoader` -- both of which
satisfy `statute.Loader`/`precedent.Loader` and delegate parsing
entirely to `statute.NewDefaultLoader()`/`precedent.NewDefaultLoader()`.
The result (`CorpusBundleResult`) is a plain aggregation of
`[]statute.RawStatute` and `[]precedent.RawPrecedent`, ready to be
handed to `statute.StatuteIngestionService`/
`precedent.PrecedentIngestionService` exactly as a network-sourced
corpus would be (those services only ever needed an `io.Reader`).

### Offline update bundles: `UpdateManifest` / `ApplyUpdateBundle`

An update bundle is a directory containing a `manifest.json`
(`UpdateManifest`: version, a map of relative file path -> SHA-256
checksum, and a signature over that map) plus the files it references.
`SignManifest` produces the signature offline, before distribution,
using any `Signer` (typically a `*provenance.HMACSigner` keyed with an
operator-distributed key). `ApplyUpdateBundle` verifies the manifest's
signature, then recomputes and compares every referenced file's
checksum -- a tampered file or a tampered/unsigned manifest both fail
closed with `ErrChecksumMismatch`/`ErrSignatureInvalid`. No network
call is made at any point.

### Offline license/activation: `LicenseKey` / `Activate`

`LicenseKey` (license ID, deployment ID, feature list, validity
window, revoked flag, signature) is issued and signed offline by the
vendor via `SignLicenseKey`. `Activate` verifies the signature,
rejects a revoked or expired license, and rejects a license issued for
a different deployment -- entirely locally, with no license-server
callout.

### Zero-egress verification: `VerifyZeroEgress`

`VerifyZeroEgress(ctx, profile, targets)` checks a caller-supplied list
of `EgressTarget{Name, Address}` values (every provider endpoint,
database host, etc. the active configuration declares) against the
profile's `NetworkPolicy`, returning an `EgressReport` enumerating
every check performed -- the startup assertion that a deployment's
*configured* endpoints are all local/offline, before any of them are
actually dialed.

### Conformance: `Conformance`

`Conformance(ctx, profile, input)` is the single rollup an air-gapped
deployment's startup path calls: profile validity
(`CheckProfileValid`), the provider registry allowlist
(`CheckProviderAllowlist`, via `AuditRegistry`), that the network
policy itself is well-formed (`CheckNetworkPolicy`), and zero-egress
verification over `input.EgressTargets` (`CheckZeroEgress`). A missing
`Registry` or empty `EgressTargets` in `ConformanceInput` does not fail
those checks -- they pass vacuously, matching this package's other
report types' "explicit check succeeded" semantics rather than
silently skipping.

## What is explicitly out of scope here

- Actually running a local model server (Ollama/LM Studio/vLLM/etc.)
  -- that is `packages/adapters/local`'s job; this package only
  gates *registration* of a non-local provider.
- Deciding *which* regions/data classes are allowed -- that remains
  `packages/dataresidency`'s policy.
- Parsing statute/precedent corpus grammar -- that remains
  `packages/statute`/`packages/precedent`'s job; this package only
  supplies the local-bundle-backed `io.Reader` source.
- Any new signature scheme -- `Signer` is satisfied directly by
  `*provenance.HMACSigner`; this package invents no cryptography.
