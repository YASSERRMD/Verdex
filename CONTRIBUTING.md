# Contributing to Verdex

## Git Identity

All commits must use:

- **Username:** `YASSERRMD`
- **Email:** `arafath.yasser@gmail.com`

## Branching

- One branch per phase, named `phase-NNN-short-slug`
  (e.g. `phase-007-jurisdiction-loader`)
- Small, non-phase corrective work (a bug fix, a security hardening
  change, a stale-test repair) may instead use `fix-short-slug`
  (e.g. `fix-guardrail-whitespace-obfuscation`) — see "Commits" below
  for how the atomic-commit minimum applies to this branch type
- Never commit directly to `main`

## Commits

- Minimum **10 atomic commits** per phase (`phase-NNN-slug` branches).
  `fix-slug` branches are exempt from this minimum — they are
  small, non-phase corrective work by definition, and are sized to
  the fix itself rather than to a fixed commit quota
- Each commit covers a single logical change
- Use the imperative mood: `Add X`, `Fix Y`, `Remove Z`
- No squash merges — full history is required for audit

## Pull Requests

- One PR per phase
- PR title: `[Phase NNN] Short description`
- No squash merges; merge commits only
- At least one reviewer approval required before merge
- The PR description must summarise the phase goal and list the commits

## Code Standards

### Non-Binding Guardrail

Every module that produces reasoning output **must** attach the `draft_analysis`
label. Verdict or directive language is rejected by the output pipeline. Tests
that assert guardrail behaviour are not optional.

### Provider Abstraction

No phase may hardcode a model provider. All inference calls must route through
the `LLMProvider` interface defined in `packages/provider`.

### Transcribe-and-Discard

Any code that handles binary ingestion artifacts must discard them after
extraction and assert the discard in tests. Provenance hashes must be recorded
before discard.

## Testing

- Unit tests alongside implementation files
- Integration tests in `*_integration_test.*` files
- All tests must pass in CI before a PR can merge

## Security

- No secrets in code or committed files
- Use `.env.example` as the template; actual `.env` files are gitignored
- Dependency changes require review of the security impact

## Review Checklist

- [ ] Branch named correctly
- [ ] ≥10 atomic commits (phase-NNN-slug branches; not required for fix-slug)
- [ ] All tests pass
- [ ] Non-binding guardrail enforced where applicable
- [ ] No hardcoded provider references
- [ ] No binary artifacts committed
- [ ] CI pipeline green
