// Package cicdgate is Phase 095: hardening this repository's own
// delivery pipeline -- enforcing the test/security/branch-policy gates
// this codebase already documents, plus a structured model for signed
// builds, release provenance, staged rollout, automated rollback, and
// release-notes generation. It draws on the secrets-scan gate added in
// Phase 083 (packages/threatmodel) and the SCA/SAST gates added in
// Phase 084 (packages/vulnmanagement) -- both already wired into
// .github/workflows/ci.yml -- and on this repository's own documented
// branch/commit/no-squash policy (CONTRIBUTING.md,
// .github/branch-protection.md, .github/pull_request_template.md),
// making that policy checkable in CI rather than leaving it as
// documentation a reviewer must remember to enforce by hand.
//
// # What is new here
//
//   - ValidateBranchName / ValidatePRCommitCount (branchpolicy.go): a
//     real, runnable check of this repository's phase-NNN-slug /
//     fix-slug branch-naming convention and its documented >=10-commit
//     minimum per phase-NNN-slug pull request (fix-slug pull requests,
//     this repository's convention for small non-phase corrective
//     work, are exempt) -- wired into a new branch-policy CI job
//     (task 3).
//   - ReleaseArtifact / BuildAttestation / Verify (types.go,
//     attestation.go): a structured data model for a signed build
//     artifact (name, digest algorithm/digest, SignatureState/
//     SignatureRef) and its build provenance (source commit, builder
//     identity, build timestamp, inputs digest), plus a Verify
//     function checking the two are each individually well-formed and
//     mutually consistent (tasks 4-5). See "What is simulated versus
//     real" below -- this models the data and consistency-check logic
//     honestly, without claiming to perform real cryptographic
//     signing this environment has no KMS/Sigstore identity configured
//     for.
//   - RolloutStage / RolloutTrigger / EvaluatePromotion (rollout.go):
//     a canary -> partial -> full staged-rollout model that promotes
//     only after a consecutive run of healthy StageHealth samples
//     (task 6).
//   - RollbackTrigger / EvaluateRollback (rollback.go): an
//     error-rate/latency-ceiling rollback model that fires on the
//     first unhealthy sample -- deliberately more trigger-happy than
//     EvaluatePromotion's sustained-health bar for promoting forward
//     (task 7).
//   - GenerateReleaseNotes (releasenotes.go): groups a release's
//     commits by their leading imperative verb (Add/Fix/Remove/
//     Document/etc, surveyed from this repository's own git history)
//     into a markdown changelog, matching this repository's actual
//     commit-message convention rather than a generic Conventional
//     Commits scheme this repository does not use (task 8).
//   - A branch-policy CI job, a signed-builds-and-provenance CI step,
//     and staged-rollout/rollback-trigger/release-notes CI steps added
//     to .github/workflows/ci.yml, plus the gate job's needs: list
//     updated to require build-go, build-ts, and secrets-scan (all of
//     which already existed and already ran to completion; this phase
//     did not need to add any of them) explicitly, alongside the new
//     branch-policy job (tasks 1-2, 6-8).
//
// # What is simulated/placeholder versus real
//
// This phase does not stand up a real Sigstore/cosign signing
// pipeline: doing so requires a KMS or OIDC signing identity that is
// external infrastructure this environment does not have configured.
// SignatureStatePlaceholder documents this honestly -- a
// ReleaseArtifact produced by this phase's CI step records a
// SignatureState of "placeholder" and a SignatureRef that is a
// simulated identifier, never a value claiming to be a real,
// independently verifiable cryptographic signature. Verify checks
// structural well-formedness and cross-record consistency (the
// mechanical layer a real signature check would sit on top of), not
// cryptographic authenticity. Promoting SignatureStatePlaceholder to
// SignatureStateVerified, backed by a real signing identity and
// transparency log, is future work once that infrastructure exists.
//
// Similarly, RolloutTrigger/RollbackTrigger's StageHealth samples are
// modeled as an input this package's logic evaluates, not wired to a
// real metrics pipeline in this phase -- a production deployment would
// populate StageHealth from packages/observability, exactly as
// packages/vulnmanagement's SLA tracking documents its own severity
// thresholds as logic ready for a real scanner feed.
//
// # packages/iac composition (Phase 094)
//
// At the time this phase was written, packages/iac (Phase 094) did
// not yet exist in this repository (verified with `ls packages/iac`
// immediately before writing rollout.go/rollback.go). RolloutTrigger
// and RollbackTrigger are therefore minimal, local types scoped to
// this package's own release-pipeline needs (which artifact/release,
// which stage, what health gate) rather than a composition with a
// RolloutStrategy/PromotionPipeline type from packages/iac. Once
// packages/iac lands, it is the natural long-term home for
// staged-rollout orchestration across this platform generally (not
// just CI/CD releases); this package's types are deliberately narrow
// enough that composing with (or being superseded by) packages/iac's
// broader model later is a straightforward follow-up, not a breaking
// redesign.
//
// # What is explicitly reused, not duplicated
//
//   - .github/workflows/ci.yml's existing lint-go, build-go, build-ts,
//     lint-ts, secrets-scan, sca-scan, sast-scan, and container-scan
//     jobs (Phases 083-084) are untouched by this phase except for the
//     gate job's needs: list and additive new jobs/steps -- this phase
//     does not remove, rename, or restructure any existing job.
//   - packages/threatmodel's secrets-scan gate (Phase 083) and
//     packages/vulnmanagement's SCA/SAST gates (Phase 084) remain the
//     only security-scanning jobs in this pipeline; this package adds
//     a branch-policy check and a release/provenance/rollout model
//     alongside them, not a second scanning implementation.
//   - CONTRIBUTING.md, .github/branch-protection.md, and
//     .github/pull_request_template.md remain the single documented
//     source of this repository's branch/commit/no-squash policy;
//     ValidateBranchName/ValidatePRCommitCount make that existing,
//     already-written-down policy mechanically checkable, rather than
//     inventing a second, competing policy.
//
// See doc/cicd.md for the full write-up, including the CI workflow
// diff summary and the composition table.
package cicdgate
