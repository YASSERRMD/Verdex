// Package treeassembly assembles the full IRAC reasoning tree for a
// case from components already extracted by upstream phases: issues
// (packages/issue, Phase 033), facts (packages/fact, Phase 034), rules
// linked into applications (packages/application, Phases 035-037). It
// validates the assembled tree's structural integrity, detects semantic
// gaps (unaddressed issues, unresolved applications), versions each
// assembly as a new irac.TreeRevision, records lightweight telemetry
// about the run, and persists the result via packages/graph.
//
// # Core concepts
//
//   - AssemblyInput / Tree (workflow.go): the input components and the
//     assembled output.
//   - ComposeTree (compose.go): gathers nodes and reconstructs edges
//     from each node's recorded irac.Provenance, per the legal edge
//     triples in packages/irac/edge.go.
//   - ValidateIntegrity (integrity.go): wraps irac.ValidateTree.
//   - DetectGaps (gap.go): semantic checks beyond structural validation.
//   - NextRevision (revision.go): versions a Tree via irac.NextRevision.
//   - ReassembleIncremental (incremental.go): extends a Tree with new
//     evidence without rebuilding it from scratch.
//   - AssemblyTelemetry / Recorder (telemetry.go): records per-run
//     counts and duration.
//   - PersistTree / SnapshotStore (persist.go): persists nodes/edges via
//     graph.GraphStore and snapshots the tree via graph.Export.
//   - TreeAssemblyService (service.go): orchestrates all of the above.
//
// # The ConclusionProvider extension point
//
// This package composes issues, rules, facts, and applications into a
// tree, but it does NOT generate irac.ConclusionNodes. Synthesizing a
// reasoned, non-binding conclusion from an Application requires an LLM
// reasoning agent — that is Phase 055 ("Synthesis & reasoned-opinion
// agent"), which is explicitly out of scope here.
//
// Instead, ComposeTree and TreeAssemblyService accept any
// ConclusionProvider implementation (compose.go) and call it to obtain
// whatever ConclusionNodes, if any, should be included in the tree.
// NoOpConclusionProvider — the default — always returns an empty slice,
// so today's assembled trees simply have zero conclusions. Once Phase
// 055 exists, it need only implement ConclusionProvider; no change to
// this package's composition, validation, gap-detection, revisioning,
// telemetry, or persistence logic is required. See doc/tree-assembly.md
// for a fuller discussion of why conclusions are pluggable rather than
// generated here.
package treeassembly
