# Documentation site and internal-link checker (Phase 098)

This phase adds the bulk of this repository's operator, admin, and
developer documentation under `docs/` at the repository root, plus a
small Go package -- `packages/docsite` -- that automates the "review
and link docs" task rather than leaving it a manual claim.

## Goal

Complete, navigable documentation tying together the ~90 packages'
individual `doc/*.md` files that Phases 001-097 already wrote, without
duplicating any of their content -- plus a real, tested mechanism
proving every internal link in that documentation tree actually
resolves.

## The doc-site structure

`docs/` at the repository root is the "doc site": a set of
higher-level documents that link out to, rather than restate, every
package's own `doc/*.md` file.

| Path | Purpose |
|---|---|
| `docs/README.md` | The doc-site index -- links to every document below, plus every package's own `doc/*.md`, organized by topic. This is "publish documentation site" (task 10): a real navigable index, not a hosted-site build. |
| `docs/architecture/overview.md` | The 8-part system architecture, naming every real package by phase. |
| `docs/deployment/{cloud,onprem,airgapped}.md` | Per-tier deployment guides, operationalizing `packages/iac` (Phase 094) and `packages/airgapped` (Phase 079). |
| `docs/admin/setup-guide.md` | First-run tenant provisioning, walking `packages/setup` (Phase 008) and `packages/jurisdiction` (Phase 007). |
| `docs/user-guide/judges-advocates.md` | The practitioner-facing case-workspace walkthrough. |
| `docs/operations/runbooks.md` | Routine reliability posture and post-deploy verification, over `packages/reliability` (Phase 093) and `packages/iac` (Phase 094). |
| `docs/operations/incident-response.md` | A general incident-response runbook, mirroring `packages/backupdr/doc/dr-runbook.md`'s (Phase 085) shape. |
| `docs/api/reference.md` | An index into `packages/gateway`'s (Phase 009) conventions and `packages/knowledgeapi`'s (Phase 048) facade, not a re-specification. |
| `docs/security-compliance/overview.md` | Part 7 of the architecture, indexed by phase number. |

## `packages/docsite`: the automated link checker

Every document above -- and every existing package's `doc/*.md` -- is
full of relative markdown links to other documents in this
repository. Nothing enforced that those links stay valid as the
documentation tree grew; a renamed file or a typo'd path would sit
silently broken until a human happened to click it. `packages/docsite`
replaces that manual review with a real, runnable check.

### `CheckLinks`

```go
report, err := docsite.CheckLinks(repoRoot)
```

`CheckLinks` walks `repoRoot`'s `docs/` tree (recursively) and every
`packages/*/doc/` directory (non-recursively -- no package nests a
subdirectory under its own `doc/` as of this phase), extracts every
markdown inline link (link text in square brackets, immediately
followed by a parenthesized target) from every file found, and
verifies each internal link target resolves to a real file or
directory on disk, relative to the linking file's own directory.
External links (`http://`, `https://`, `mailto:`) are skipped
entirely -- this checker only ever asserts about paths inside this
repository. A target's trailing `#fragment` is stripped before
resolution: `CheckLinks` verifies the linked file or directory exists,
not that a specific heading-anchor slug exists within it (a reasonable
future extension, not something this phase claims to do).

The result is a `Report`: every file scanned, a count of internal
links checked, and a `[]BrokenLink` -- each recording the exact source
file, line number, link text, raw target, and the path `CheckLinks`
attempted to resolve it to. `Report.OK()` reports whether zero broken
links were found.

### Why fenced code blocks are skipped

This repository's documentation is dense with Go code examples, and
Go's generic function-call syntax is otherwise indistinguishable from
a markdown inline link to a regex-based scanner. For a real example
this phase's own `checklinks` run found inside
`packages/reliability/doc/reliability.md`:

```go
guard := reliability.NewIdempotencyGuard[PaymentResult](10 * time.Minute)
```

`extractLinks` tracks fenced-code-block state (triple-backtick and
triple-tilde delimiters) and skips every line inside one, rather than
trying to special-case Go syntax specifically. See
`linkcheck_test.go`'s `TestExtractLinks_SkipsFencedCodeBlocks` for the
fixture proving this holds for both fence styles, and that scanning
resumes correctly once a block closes.

### Proof this actually catches broken links

`linkcheck_test.go` exercises three fixture trees under `testdata/`:

- `testdata/clean/` -- a real cross-linked tree (a `docs/` index, an
  `architecture/overview.md`, a `packages/foo/doc/foo.md`, and a
  fenced-code-block fixture) proving `CheckLinks` passes real links,
  directory links, and same-page anchors clean, while never resolving
  external/`mailto:` links against disk.
- `testdata/broken/` -- a deliberately broken tree with two links that
  do not resolve, proving `CheckLinks` reports the exact file, line,
  link text, and target for each one (not just "something is broken
  somewhere").
- `testdata/empty/` -- an empty tree, proving `CheckLinks` returns
  `ErrNoMarkdownFiles` (an error, not a silently "clean" `Report`) when
  pointed at a root with nothing to check -- almost always a
  misconfigured root rather than a legitimate empty documentation tree.

### `cmd/checklinks`: wired into CI

```
go run ./packages/docsite/cmd/checklinks .
```

prints every file scanned and link checked, then either reports a
clean run or prints every broken link's file/line/text/target and
exits non-zero. `.github/workflows/ci.yml`'s `docs-link-check` job
(added by this phase, additive to every existing job) runs exactly
this command against the checked-out repository root on every pull
request, so a broken documentation link fails CI the same way a broken
test does -- it is no longer something a reviewer has to notice by
eye.

Run for real against the documentation tree this phase produced (every
file under `docs/` plus every `packages/*/doc/*.md`), it reports:

```
checklinks: scanned 96 markdown file(s), checked N internal link(s)
checklinks: no broken links found
```

(the exact scanned/checked counts grow as later phases add more
documentation; see `linkcheck_test.go` for the fixed counts the test
suite itself asserts on).

## What this package deliberately does not do

- It is not a CommonMark parser -- it matches the inline-link form
  every link in this repository's documentation uses as of this
  phase, not reference-style links or bare autolinks.
- It does not verify a `#fragment` heading anchor actually exists
  within the resolved file, only that the file or directory itself
  exists.
- It does not check external links for reachability.

See `packages/docsite/doc.go` for the same summary in Go-doc form.
