package application

import (
	"strings"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// RuleMatch pairs a candidate OriginatedRule with a text-overlap Score
// against a specific irac.IssueNode, produced by MatchIssueToRules.
type RuleMatch struct {
	// Rule is the candidate rule this match scores.
	Rule OriginatedRule

	// Score is the keyword/text-overlap score between the issue's text
	// and the rule's text, in the closed interval [0, 1]. Higher means
	// more textually related.
	Score float64
}

// MatchIssueToRules scores every rule in rules against issue's text
// using a symmetric keyword/token-overlap heuristic (mirroring
// packages/fact's temporalTokenOverlap and packages/issue's claim-mapping
// overlap conventions: no ML model, deterministic lexical scoring), and
// returns the results sorted by descending Score. Rules that share no
// tokens with the issue score 0 but are still included in the result.
//
// An empty issue text or an empty rules slice yields an empty result
// slice (not an error) — callers that need to distinguish "no candidate
// rules supplied" from "no rule matched" should check len(rules) before
// calling, or test the returned slice's length/scores; ErrNoMatchingRules
// is reserved for higher-level pipeline stages (see service.go) that
// require at least one positively-scoring match.
func MatchIssueToRules(issue irac.IssueNode, rules []OriginatedRule) []RuleMatch {
	issueTokens := tokenize(issue.Text)

	matches := make([]RuleMatch, len(rules))
	for i, rule := range rules {
		ruleTokens := tokenize(rule.Rule.Text)
		matches[i] = RuleMatch{
			Rule:  rule,
			Score: tokenOverlapScore(issueTokens, ruleTokens),
		}
	}

	sortMatchesDescending(matches)
	return matches
}

// tokenize lowercases and splits text into a deduplicated set of
// word tokens, discarding empty fragments. This is the same
// lightweight, dependency-free lexical normalization used across
// packages/fact and packages/issue's overlap heuristics.
func tokenize(text string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= '0' && r <= '9':
			return false
		default:
			return true
		}
	})
	tokens := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if f == "" {
			continue
		}
		tokens[f] = struct{}{}
	}
	return tokens
}

// tokenOverlapScore computes a Jaccard-similarity-style overlap ratio
// between two token sets: the number of shared tokens divided by the
// size of the smaller set. Using the smaller set as the denominator
// (rather than the union, as strict Jaccard would) rewards a short,
// precise rule statement that is fully contained in a longer issue
// statement (or vice versa) with a high score, which better reflects
// "this rule is clearly about this issue" than strict Jaccard would.
func tokenOverlapScore(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}
	shared := 0
	for tok := range small {
		if _, ok := large[tok]; ok {
			shared++
		}
	}
	return float64(shared) / float64(len(small))
}

// sortMatchesDescending sorts matches in place by descending Score,
// using a stable insertion sort (match counts per issue are small — this
// is not a hot path) so equal-scoring rules retain their input order.
func sortMatchesDescending(matches []RuleMatch) {
	for i := 1; i < len(matches); i++ {
		for j := i; j > 0 && matches[j].Score > matches[j-1].Score; j-- {
			matches[j], matches[j-1] = matches[j-1], matches[j]
		}
	}
}
