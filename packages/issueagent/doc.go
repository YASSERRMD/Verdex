// Package issueagent is a tool-using reasoning agent, built on
// packages/agentframework, that frames an already-assembled case's legal
// issues for adjudication.
//
// # Scope
//
// This is the first concrete agent built on packages/agentframework
// (Phase 049). It reads irac.IssueNodes that packages/issue (Phase 033)
// already extracted and packages/treeassembly already wired into a case's
// reasoning tree — via packages/knowledgeapi, never directly — and
// produces a structured, jurisdiction-aware IssueAnalysisResult: each
// issue ranked by materiality, its governing legal questions identified,
// any ambiguities or gaps surfaced, and a [0,1] confidence score
// attached. See doc/issue-agent.md for the full model and its place in
// the Part 5 pipeline (feeding Phases 051/052's argument agents and
// Phase 055's synthesis agent).
//
// # Composes with, does not duplicate
//
// packages/issue performs deterministic, rule-based EXTRACTION of
// candidate issues from raw case facts/segments into irac.IssueNodes,
// before a case's tree is assembled. This package is downstream and
// different in kind: it is an LLM-backed reasoning agent that operates on
// an ALREADY-ASSEMBLED tree, reading existing IssueNodes (and their
// linked RuleNodes) via knowledgeapi and reasoning about, ranking, and
// framing them for adjudication. It never constructs a new IssueNode or
// mutates the tree — its output (FramedIssue / IssueAnalysisResult)
// references existing IssueNode IDs by string, for downstream agents to
// consume, and is never written back into the reasoning tree itself.
//
// # Model-agnostic by construction
//
// Every model call is routed through a caller-supplied *router.Router,
// via a packages/agentframework Runner — this package never imports
// packages/adapters or talks to a provider.LLMProvider directly.
//
// See doc/issue-agent.md for the prompt template(s), the exact output
// shape, and the explicit boundary against packages/issue.
package issueagent
