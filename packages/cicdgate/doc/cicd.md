# CI/CD pipeline hardening (Phase 095)

This phase draws together two earlier threads -- the secrets-scan gate
added in Phase 083 (`packages/threatmodel`) and the SCA/SAST gates
added in Phase 084 (`packages/vulnmanagement`), both already wired into
`.github/workflows/ci.yml` -- plus this repository's own documented
branch/commit/no-squash policy (`CONTRIBUTING.md`,
`.github/branch-protection.md`, `.github/pull_request_template.md`)
into a single package: `packages/cicdgate`, and a corresponding set of
additive changes to `.github/workflows/ci.yml` itself.

## Goal

Secure, gated delivery: a full test-suite and security-scan gate that
actually blocks a merge, a branch/commit policy check that is
mechanically enforced instead of merely documented, a structured model
for signed builds and release provenance, staged rollout and automated
rollback logic, and release-notes generation -- all as real, tested Go
code plus real (additive) CI workflow wiring.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/threatmodel` (Phase 083) | `secrets-scan` CI job (gitleaks) | Named in the `gate` job's `needs:` list explicitly (task 2); no changes to the job itself |
| `packages/vulnmanagement` (Phase 084) | `sca-scan` (govulncheck) / `sast-scan` (gosec) CI jobs, both `continue-on-error: true` | Left non-blocking -- see "SCA/SAST gate decision" below (task 2) |
| `CONTRIBUTING.md` / `.github/branch-protection.md` / `.github/pull_request_template.md` | Documented phase-NNN-slug branch naming, >=10-commit minimum, no-squash merge-commit-only policy | `ValidateBranchName`/`ValidatePRCommitCount` (branchpolicy.go) make that policy checkable in CI via a new `branch-policy` job (task 3) |
| `packages/iac` (Phase 094) | Not present in this repository as of this phase (`ls packages/iac` returned nothing when checked) | `RolloutTrigger`/`RollbackTrigger` are minimal local types; `packages/iac` is the natural long-term composition point once it exists -- see "packages/iac composition" below (tasks 6-7) |

No other package is imported by `packages/cicdgate` -- its `go.mod`
declares no dependencies at all, matching `packages/irac`'s and
`packages/category`'s minimal-module pattern rather than
`packages/compliance`'s persistence-backed one, because every type
here operates on plain data (strings, counts, commit lists, health
samples) that CI or a caller supplies, not on a tenant-scoped
repository this package would need to own.

## Branch and commit policy enforcement (task 3)

`ValidateBranchName` (branchpolicy.go) accepts either
`phase-NNN-slug` (`CONTRIBUTING.md`'s documented convention; `NNN` is
at least two digits so historical branches like `phase-001` still
validate) or `fix-slug` (this repository's own observed convention for
small corrective work -- e.g. `fix-notifications-access-check`,
`fix-annotations-audit-permission` -- now documented in
`CONTRIBUTING.md`'s "Branching" section). `ValidatePRCommitCount`
checks a pull request's commit count against `MinimumCommitCount`
(10) for `phase-NNN-slug` branches, matching `CONTRIBUTING.md`'s
"Minimum 10 atomic commits per phase" and `pull_request_template.md`'s
checklist item of the same number; `fix-slug` branches are exempt,
since the minimum is explicitly phase-sized and every `fix-*` pull
request merged before this check existed shipped with a single
commit.

Both are wired into a new `branch-policy` job in
`.github/workflows/ci.yml` that runs on `pull_request`, builds a tiny
Go program invoking these two functions against
`github.event.pull_request.head.ref` and
`github.event.pull_request.commits` (available directly on the
`pull_request` event payload, no extra API call needed), and fails the
job -- and therefore the `gate` job, once wired into its `needs:` list
-- when either check fails.

No-squash itself (whether a merge used squash vs. a merge commit) is
enforced by the branch-protection settings documented in
`.github/branch-protection.md` ("Allow squash merging: disabled"),
which is a GitHub repository setting, not something a workflow step
observes after the fact -- there is no `pull_request` event field that
reports which merge strategy *would* be used before a maintainer
clicks merge. This phase's contribution is making the two properties
that *are* observable pre-merge (branch name, commit count) real,
enforced CI checks rather than only a documentation checklist.

## Full test-suite and security-scan gates (tasks 1-2)

The `gate` job's `needs:` list is updated to add `branch-policy`
alongside its existing `build-go`, `build-ts`, `secrets-scan`, and
`container-scan` entries -- all four of which already existed and
already had to pass before this phase; this phase did not need to add
any of them; it only makes the addition of `branch-policy` explicit
and documents, in this file, why the pre-existing set is correct as
being aggregated.

**SCA/SAST gate decision:** `sca-scan` (govulncheck, Phase 084) and
`sast-scan` (gosec, Phase 084) remain `continue-on-error: true` and
are **not** promoted to `gate` dependencies in this phase. Phase 084's
own comment in `ci.yml` documents why, as of that phase, `sca-scan`
surfaced pervasive standard-library findings unrelated to any specific
PR's change (crypto/tls, crypto/x509, net/url fixes shipped in Go
1.25.6-1.25.9, while this repository pins `go-version: '1.25'`) and
`sast-scan` surfaced findings this repository's own `//nolint:gosec`
suppressions already address (just not recognized by the standalone
`gosec` binary the way `golangci-lint`'s own gosec integration
recognizes them). Re-verifying at the time of this phase: neither
condition has changed -- the repository still pins the same Go
toolchain minor version, and the same `//nolint:gosec` suppressions
are still present in `packages/adapters/*/adapter.go`. Promoting
either to a hard merge-blocker today would gate every unrelated PR on
a pre-existing condition this phase (and Phase 084) did not introduce
and this phase does not fix. The correct fix -- bumping the pinned Go
toolchain patch version repo-wide, and adding native `//#nosec`-style
suppressions recognized by the standalone `gosec` binary -- is real
work but out of scope for a CI/CD-pipeline-hardening phase; leaving
both jobs `continue-on-error: true` and undocumented-but-visible in
the PR checks list remains the correct call until that follow-up
lands. This decision is deliberately re-stated here rather than
silently carried over, so a future phase revisiting this file has an
explicit record of why the gate looks the way it does.

## Signed builds and artifacts (task 4)

`ReleaseArtifact` (types.go) models one built, checksummed,
(placeholder-)signed pipeline output: `Name`, `DigestAlgorithm`
(`sha256`/`sha512`) + `Digest` (validated as well-formed lowercase hex
of the expected length for that algorithm), `SignatureState`
(`unsigned`/`placeholder`/`verified`) + `SignatureRef`, and `BuiltAt`.
`Validate` checks structural well-formedness only -- it cannot and
does not check that `Digest` actually matches any real file's bytes,
since this package receives only the recorded artifact metadata, not
the artifact's bytes themselves.

## Provenance / attestation for releases (task 5)

`BuildAttestation` (types.go) models build provenance in the shape of
in-toto/SLSA provenance's core fields -- `SourceCommit` (a full 40- or
64-character lowercase-hex git object ID), `BuilderID`,
`BuildTimestamp` (rejected if more than `clockSkewTolerance` in the
future), and an `InputsDigestAlgorithm`/`InputsDigest` pair -- without
importing an actual in-toto library, since this phase models the data
shape and validation logic, not a full attestation-framework
integration.

`Verify(artifact, attestation) error` (attestation.go) checks both
records' own `Validate` first, then mutual consistency: an unsigned
artifact has nothing further to check; a signed/placeholder-signed
artifact's `BuiltAt` must not precede its attestation's
`BuildTimestamp`, and the gap between the two must stay within
`maxBuildToArtifactSkew` (1 hour) -- a generous bound for a CI build
step's own wall-clock duration, without being so generous that two
records from unrelated builds would pass as if they were the same
build.

### What is simulated/placeholder versus real

This phase does **not** stand up a real Sigstore/cosign signing
pipeline. Doing so requires a KMS or OIDC signing identity that is
external infrastructure this environment does not have configured --
no cloud KMS key, no GitHub OIDC-to-Sigstore Fulcio binding, no
`cosign` keypair is provisioned anywhere in this repository or its CI
secrets. `SignatureStatePlaceholder` documents this honestly:
`.github/workflows/ci.yml`'s new `sign-artifacts` step records a
`SignatureState` of `"placeholder"` and a `SignatureRef` that is a
simulated identifier (e.g. `placeholder://sigstore/<artifact-name>`),
never a value dressed up to look like a real, independently verifiable
cryptographic signature. `Verify`'s consistency checks are the
mechanical layer a real signature/transparency-log check would sit on
top of once that infrastructure exists -- promoting
`SignatureStatePlaceholder` to `SignatureStateVerified` is future work,
not something this phase claims to have already done.

## Staged rollout automation (task 6)

`RolloutStage` (`canary` -> `partial` -> `full`, rollout.go) is a
closed, ordered enum. `RolloutTrigger` names a release, a target
stage, and `RequiredHealthySamples`. `EvaluatePromotion` walks a
chronological list of `StageHealth` samples and promotes only once a
consecutive run of at least `RequiredHealthySamples` healthy samples
trails the list -- an early unhealthy sample resets the count rather
than being outweighed by enough later good ones, mirroring how a real
canary analyzer should not let one bad signal be diluted by volume.

## Automated rollback triggers (task 7)

`RollbackTrigger` (rollback.go) names a release, a stage, and
`MaxErrorRate`/`MaxLatencyP99Ms` ceilings. `EvaluateRollback` looks at
only the single most recent `StageHealth` sample and fires a
`RollbackDecision` (with a `RollbackReason` of
`error_rate_exceeded`/`latency_exceeded`) on the first ceiling breach.
This is deliberately more trigger-happy than `EvaluatePromotion`'s
sustained-health bar for promoting forward: rolling back on the first
sign of trouble, but promoting only after sustained health, is an
intentionally asymmetric safety posture -- the cost of a false-positive
rollback (a brief automatic revert) is much lower than the cost of a
false-negative promotion (a real regression let through because one
bad sample was outvoted).

### packages/iac composition (Phase 094)

At the time this phase was written, `packages/iac` (Phase 094) was
checked with `ls packages/iac` and did not exist in this repository.
`RolloutTrigger`/`RollbackTrigger` are therefore minimal, local types
scoped to this package's own release-pipeline needs, not a composition
with a `RolloutStrategy`/`PromotionPipeline` type from `packages/iac`.
Once `packages/iac` lands, it is the natural long-term home for
staged-rollout orchestration across this platform generally (not just
CI/CD releases specifically); this package's types stay deliberately
narrow (a release name, a stage, a health-sample list) so that
composing with -- or later being superseded by -- `packages/iac`'s
broader model is a straightforward follow-up, not a breaking redesign.

## Release notes automation (task 8)

`GenerateReleaseNotes(commits []CommitInfo) string` (releasenotes.go)
groups commits by their leading imperative verb into a markdown
changelog. The verb vocabulary
(`Added`/`Fixed`/`Removed`/`Changed`/`Documented`/`Tests`, plus an
`Other Changes` catch-all for merge commits and unrecognized verbs)
was built from a real survey of this repository's own commit history
(`git log --format=%s` over the last 1000 commits: `Add` appears 484
times, `Merge` 73, `Test` 70, `Document` 58, `Scaffold` 41, `Define`
35, plus a long tail of other verbs), matching
`CONTRIBUTING.md`'s actual imperative-mood convention rather than a
generic Conventional Commits (`feat:`/`fix:`) scheme this repository
does not use. `releasenotes_test.go`'s `TestGenerateReleaseNotes_GoldenOutput`
asserts the exact rendered markdown against a fixed sample commit set
drawn from real subject lines in this repository's history.

## Tests for pipeline gates (task 9)

Every validator/generator/evaluator above has a corresponding
table-driven `_test.go` file: `branchpolicy_test.go`,
`types_test.go`, `attestation_test.go`, `rollout_test.go`,
`rollback_test.go`, `releasenotes_test.go`. All exercise both the
success path and every documented failure mode (nil receivers, boundary
values, malformed digests/timestamps, an early bad health sample
resetting a promotion streak, etc), following this repository's
`errors.Is`-based assertion convention (see
`packages/category/validate_test.go`).

## CI workflow changes (`.github/workflows/ci.yml`)

All changes below are additive; no existing job's steps were removed
or restructured:

- New `branch-policy` job (task 3): runs on `pull_request`, builds and
  runs a small Go program in this package invoking `ValidateBranchName`
  and `ValidatePRCommitCount` against the `pull_request` event's `head.ref`
  and `commits` count.
- New `sign-artifacts` job (tasks 4-5): builds `packages/cicdgate`,
  constructs a `ReleaseArtifact` + `BuildAttestation` pair for the
  build output (recording `SignatureState: placeholder`, per "What is
  simulated versus real" above), and runs `Verify` against them.
- New `release-automation` job (tasks 6-8): a placeholder/documentation
  job (this repository has no deployed environment or release-tag
  trigger to evaluate `StageHealth` against yet -- mirroring
  `container-scan`'s own "this repository has no Dockerfile yet"
  placeholder pattern) that documents the rollout/rollback/release-notes
  integration points this package's functions are ready for once a real
  deploy target and release-tagging flow exist.
- `gate` job's `needs:` list: adds `branch-policy` and `sign-artifacts`
  alongside the existing `build-go`, `build-ts`, `secrets-scan`, and
  `container-scan` entries. `sca-scan`/`sast-scan` remain excluded per
  the "SCA/SAST gate decision" above; `release-automation` is a
  documentation placeholder and is not gated on, mirroring
  `container-scan`'s own placeholder-job treatment before this phase.
