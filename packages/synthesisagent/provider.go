package synthesisagent

import (
	"context"
	"fmt"
	"log"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// generatedBy identifies this package as the irac.Provenance.GeneratedBy
// value attached to every irac.ConclusionNode Provider produces.
const generatedBy = "synthesisagent-v1"

// Provider adapts an Opinion into treeassembly.ConclusionProvider, the
// pluggable extension point treeassembly.ComposeTree has accepted since
// Phase 039 specifically so that "once Phase 055 lands, it need only
// implement ConclusionProvider and be wired in" (see
// packages/treeassembly's own doc.go and compose.go). This type is that
// wiring: pass a Provider wrapping an already-synthesized Opinion to
// treeassembly.ComposeTree in place of treeassembly.NoOpConclusionProvider,
// and every one of the Opinion's TentativeConclusions becomes an
// irac.ConclusionNode in the assembled tree.
//
// Provider enforces the non-binding guardrail at this exact boundary: it
// only ever constructs an irac.ConclusionNode via irac.NewConclusionNode
// (which unconditionally attaches the mandatory draft_analysis label),
// and it rejects — logging rather than emitting a node for — any
// TentativeConclusion whose Text trips irac.ContainsVerdictLanguage. A
// caller wanting stronger-than-log handling of a rejected conclusion
// (blocking the whole run, surfacing a review flag, etc.) is Phase 057's
// guardrail-POLICY layer, layered on top of this package rather than
// duplicated here.
type Provider struct {
	// Opinion is the already-synthesized draft opinion this Provider
	// converts into irac.ConclusionNodes.
	Opinion Opinion
}

// Provide implements treeassembly.ConclusionProvider. It converts every
// TentativeConclusion in p.Opinion.Conclusions into an
// irac.ConclusionNode, deriving each node's Provenance.UpstreamNodeIDs
// from the conclusion's SupportingFactIDs/SupportingRuleIDs matched
// against input's Applications (Conclusion --concludes_from--> Application
// is the only legal edge triple for a ConclusionNode per
// packages/irac/edge.go, so an ApplicationNode is linked as upstream when
// it shares an upstream fact/rule reference with the conclusion,
// connecting the conclusion to the application(s) reasoning about the
// same evidence).
//
// A TentativeConclusion whose Text contains verdict/directive language
// (per irac.ContainsVerdictLanguage) is rejected: it is logged and
// excluded from the returned slice rather than ever reaching
// irac.NewConclusionNode. Provide never returns a non-nil error for this
// case — a rejected conclusion is a data-quality finding, not a
// synthesis-provider failure — mirroring
// treeassembly.NoOpConclusionProvider's own "no error path" convention
// for the common case.
func (p Provider) Provide(_ context.Context, input treeassembly.AssemblyInput) ([]irac.ConclusionNode, error) {
	appsByUpstream := indexApplicationsByUpstreamRef(input.Applications)

	nodes := make([]irac.ConclusionNode, 0, len(p.Opinion.Conclusions))
	for i, tc := range p.Opinion.Conclusions {
		if irac.ContainsVerdictLanguage(tc.Text) {
			log.Printf("synthesisagent: rejecting conclusion for issue %q: %v: text contains verdict language", tc.IssueNodeID, ErrVerdictLanguage)
			continue
		}

		upstream := upstreamApplicationIDs(tc, appsByUpstream)

		id := fmt.Sprintf("%s-conclusion-%d", tc.IssueNodeID, i)
		node := irac.NewConclusionNode(
			id,
			p.Opinion.CaseID,
			tc.Text,
			p.Opinion.GeneratedAt,
			tc.Confidence,
			irac.Provenance{
				GeneratedBy:     generatedBy,
				GeneratedAt:     p.Opinion.GeneratedAt,
				UpstreamNodeIDs: upstream,
			},
		)
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// indexApplicationsByUpstreamRef indexes every ApplicationNode in apps by
// each fact/rule ID in its own Provenance.UpstreamNodeIDs, so
// upstreamApplicationIDs can look up "which applications reasoned about
// this fact/rule" in O(1) per reference.
func indexApplicationsByUpstreamRef(apps []irac.ApplicationNode) map[string][]string {
	out := make(map[string][]string)
	for _, app := range apps {
		for _, ref := range app.Provenance.UpstreamNodeIDs {
			out[ref] = append(out[ref], app.ID)
		}
	}
	return out
}

// upstreamApplicationIDs returns the deduplicated set of ApplicationNode
// IDs that reasoned about any of tc's SupportingFactIDs or
// SupportingRuleIDs, per appsByUpstream.
func upstreamApplicationIDs(tc TentativeConclusion, appsByUpstream map[string][]string) []string {
	seen := make(map[string]struct{})
	var out []string
	addRefs := func(ids []string) {
		for _, id := range ids {
			for _, appID := range appsByUpstream[id] {
				if _, ok := seen[appID]; ok {
					continue
				}
				seen[appID] = struct{}{}
				out = append(out, appID)
			}
		}
	}
	addRefs(tc.SupportingFactIDs)
	addRefs(tc.SupportingRuleIDs)
	return out
}
