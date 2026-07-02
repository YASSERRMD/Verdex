package treevalidation

import (
	"fmt"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/treeassembly"
)

// CodeJurisdictionMismatch flags a RuleNode whose JurisdictionCode does
// not match the case's declared jurisdiction (and is not present in an
// explicit override allow-list).
const CodeJurisdictionMismatch = "jurisdiction_mismatch"

// CheckJurisdictionConsistency enforces that every irac.RuleNode in tree
// carries a JurisdictionCode matching caseJurisdictionCode, preventing
// cross-jurisdiction leakage (e.g. a common-law precedent silently
// governing a civil-law case's issue). A RuleNode's JurisdictionCode is
// permitted to differ from caseJurisdictionCode only if it appears in
// allowedOverrides — e.g. for cases that explicitly cite persuasive
// foreign authority.
//
// A nil tree yields no findings. A blank caseJurisdictionCode disables
// the check entirely (returns no findings), since there is nothing to
// compare against. Order is deterministic: input order of tree.Nodes,
// restricted to RuleNodes.
func CheckJurisdictionConsistency(tree treeassembly.Tree, caseJurisdictionCode string, allowedOverrides ...string) []Finding {
	findings := make([]Finding, 0)

	if caseJurisdictionCode == "" {
		return findings
	}

	allowed := make(map[string]struct{}, len(allowedOverrides))
	for _, code := range allowedOverrides {
		allowed[code] = struct{}{}
	}

	for _, n := range tree.Nodes {
		rule, ok := n.(irac.RuleNode)
		if !ok {
			continue
		}
		if rule.JurisdictionCode == caseJurisdictionCode {
			continue
		}
		if _, ok := allowed[rule.JurisdictionCode]; ok {
			continue
		}
		findings = append(findings, Finding{
			Severity: SeverityCritical,
			Code:     CodeJurisdictionMismatch,
			Message: fmt.Sprintf(
				"rule %q has jurisdiction %q, expected %q (case jurisdiction)",
				rule.ID, rule.JurisdictionCode, caseJurisdictionCode,
			),
			NodeID: rule.ID,
		})
	}

	return findings
}
