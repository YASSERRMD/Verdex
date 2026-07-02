package treevalidation

import (
	"fmt"
	"sort"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeGraphCycle flags a cycle detected across the full assembled tree
// (nodes of any type, edges of any type) — distinct from
// packages/application/chain.go's RuleChain.Validate, which only detects
// a repeated rule ID within one flat, local rule chain.
const CodeGraphCycle = "graph_cycle"

// DetectCycles runs a DFS-based cycle detection over the whole tree's
// directed graph and returns one Finding per distinct cycle found.
//
// A well-formed IRAC tree's legal edge triples (packages/irac/edge.go)
// intentionally declare one bidirectional pair: Application
// --applies_to--> Fact, and Fact --supports--> Application, is the same
// conceptual relationship (a fact feeding an application) expressed in
// both directions by the schema — not a real cycle. To avoid flagging
// every legitimate application/fact pairing as a false-positive cycle,
// EdgeSupports edges (the reverse-direction half of that intentional
// pair) are excluded from the adjacency graph walked here; EdgeAppliesTo
// alone already captures that relationship's direction for cycle-
// detection purposes. Every other edge type (EdgeGoverns, the
// Application->Rule half of EdgeAppliesTo, EdgeConcludesFrom) is included
// as-is, so any other cycle — including one spanning multiple rule
// chains, applications, or node types, or an outright self-loop — is
// still caught.
//
// A nil tree yields no findings. Findings are ordered deterministically
// by the sorted node ID at which each cycle was first detected during a
// DFS that visits nodes in tree.Nodes order.
func DetectCycles(tree treeassembly.Tree) []Finding {
	findings := make([]Finding, 0)

	adjacency := make(map[string][]string)
	for _, e := range tree.Edges {
		if e.Type == irac.EdgeSupports {
			continue
		}
		adjacency[e.FromID] = append(adjacency[e.FromID], e.ToID)
	}
	for from := range adjacency {
		sort.Strings(adjacency[from])
	}

	const (
		white = 0 // unvisited
		gray  = 1 // on current DFS stack
		black = 2 // fully explored
	)
	color := make(map[string]int, len(tree.Nodes))

	var reportedCycleKeys = make(map[string]struct{})

	var dfs func(nodeID string, stack []string)
	dfs = func(nodeID string, stack []string) {
		color[nodeID] = gray
		stack = append(stack, nodeID)

		for _, next := range adjacency[nodeID] {
			switch color[next] {
			case white:
				dfs(next, stack)
			case gray:
				// Found a cycle: next already on the stack. Extract the
				// cycle portion of the stack for a readable message and
				// dedupe by its node-set so the same cycle discovered via
				// different entry points is reported once.
				cycle := cyclePortion(stack, next)
				key := cycleKey(cycle)
				if _, seen := reportedCycleKeys[key]; seen {
					continue
				}
				reportedCycleKeys[key] = struct{}{}
				findings = append(findings, Finding{
					Severity: SeverityCritical,
					Code:     CodeGraphCycle,
					Message:  fmt.Sprintf("cycle detected: %s", describeCycle(cycle)),
					NodeID:   next,
				})
			case black:
				// already fully explored, no cycle through here
			}
		}

		color[nodeID] = black
	}

	for _, n := range tree.Nodes {
		if color[n.GetID()] == white {
			dfs(n.GetID(), nil)
		}
	}

	return findings
}

// cyclePortion returns the suffix of stack starting from the first
// occurrence of target, plus target itself appended at the end to show
// the closing edge.
func cyclePortion(stack []string, target string) []string {
	idx := 0
	for i, id := range stack {
		if id == target {
			idx = i
			break
		}
	}
	cycle := append([]string{}, stack[idx:]...)
	cycle = append(cycle, target)
	return cycle
}

// cycleKey builds a dedupe key for a cycle that is invariant to which
// node in the cycle it was detected from, by rotating to start at the
// lexicographically smallest node ID.
func cycleKey(cycle []string) string {
	if len(cycle) <= 1 {
		return fmt.Sprintf("%v", cycle)
	}
	body := cycle[:len(cycle)-1] // drop the repeated closing element
	minIdx := 0
	for i, id := range body {
		if id < body[minIdx] {
			minIdx = i
		}
	}
	rotated := append(append([]string{}, body[minIdx:]...), body[:minIdx]...)
	key := ""
	for _, id := range rotated {
		key += id + ">"
	}
	return key
}

func describeCycle(cycle []string) string {
	out := ""
	for i, id := range cycle {
		if i > 0 {
			out += " -> "
		}
		out += id
	}
	return out
}
