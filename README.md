# Verdex

**Jurisdiction-Aware Judicial Reasoning & Case Analysis Platform**

Verdex is a model-agnostic legal reasoning engine that produces non-binding,
reasoned draft opinions to assist judicial and legal practitioners in
structured case analysis.

---

## Non-Binding Disclaimer

> **All outputs produced by Verdex are draft analyses only. They are not legal
> advice, legal opinions, or judicial verdicts. Every output must be reviewed,
> critically evaluated, and signed off by a qualified human practitioner
> before any use. Verdex does not replace human judicial judgement.**

This constraint is enforced in code across every reasoning pipeline and cannot
be overridden at the deployment or user level.

---

## Core Architecture

```
Ingest (transcribe-and-discard)
  → IRAC Reasoning Tree (Issue · Rule · Application · Conclusion)
    → Hybrid Graph + Vector Retrieval
      → Adversarial Argument Synthesis
        → Human-Signed Draft Opinion
```

### Key Properties

| Property | Guarantee |
|---|---|
| **Model-agnostic** | All model calls route through a provider abstraction layer; no provider is hard-coded |
| **Transcribe-and-discard** | Binary artifacts (audio, video, scanned docs) are hashed, then discarded; only extracted text persists |
| **Jurisdiction-aware** | Reasoning weights are parameterized by legal family (common law, civil law, mixed, Sharia-influenced) |
| **Non-binding by design** | Verdict/directive language is blocked in code; human sign-off is a hard gate |
| **Cross-case isolation** | Case-scoped retrieval prevents fact leakage between cases |
| **Provenance** | Every claim traces to a source node; every source node carries a citation |

---

## Monorepo Layout

```
verdex/
├── services/          # Backend microservices (Go)
├── packages/          # Shared libraries and SDKs
├── infra/             # Infrastructure as code (Terraform)
├── docs/              # Architecture, operator, and user documentation
└── temp/              # Scratch space — not committed to production builds
```

---

## Getting Started

See [`docs/setup.md`](docs/setup.md) for deployment and first-run setup.

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) for branching, commit, and PR conventions.

---

## License

Apache 2.0 — see [`LICENSE`](LICENSE).
