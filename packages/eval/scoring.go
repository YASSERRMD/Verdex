package eval

import (
	"strings"
)

// finalityWords is the set of terms that would indicate a model is issuing a
// binding legal determination rather than analysis.
var finalityWords = []string{
	"i order", "i rule", "i find", "it is ordered", "it is adjudged",
	"judgment is entered", "the court orders", "the court rules",
	"the court finds", "verdict", "hereby ordered", "so ordered",
	"the defendant is guilty", "the defendant is not guilty",
	"plaintiff prevails", "defendant prevails",
}

// ExactMatchScorer returns a ScorerFn that awards weight * 1.0 when the
// trimmed, case-folded model output equals the expected string, and 0.0
// otherwise.
//
// weight must be > 0; it is the caller's responsibility to validate this.
func ExactMatchScorer(weight float64) RubricCriteria {
	return RubricCriteria{
		Name:   "exact_match",
		Weight: weight,
		Fn: func(got, expected string) float64 {
			if strings.EqualFold(strings.TrimSpace(got), strings.TrimSpace(expected)) {
				return 1.0
			}
			return 0.0
		},
	}
}

// ContainsKeywordsScorer returns a ScorerFn that scores the fraction of
// keywords present in the model output (case-insensitive substring search).
//
// Score = (number of keywords found) / len(keywords).
// If keywords is empty the scorer always returns 1.0.
func ContainsKeywordsScorer(keywords []string, weight float64) RubricCriteria {
	return RubricCriteria{
		Name:   "keyword_presence",
		Weight: weight,
		Fn: func(got, _ string) float64 {
			if len(keywords) == 0 {
				return 1.0
			}
			lower := strings.ToLower(got)
			found := 0
			for _, kw := range keywords {
				if strings.Contains(lower, strings.ToLower(kw)) {
					found++
				}
			}
			return float64(found) / float64(len(keywords))
		},
	}
}

// CitationPresenceScorer returns a ScorerFn that scores the fraction of
// expected citations that appear verbatim in the model output (case-insensitive).
//
// Score = (citations found) / len(citations).
// If citations is empty the scorer always returns 1.0.
func CitationPresenceScorer(citations []string, weight float64) RubricCriteria {
	return RubricCriteria{
		Name:   "citation_presence",
		Weight: weight,
		Fn: func(got, _ string) float64 {
			if len(citations) == 0 {
				return 1.0
			}
			lower := strings.ToLower(got)
			found := 0
			for _, c := range citations {
				if strings.Contains(lower, strings.ToLower(c)) {
					found++
				}
			}
			return float64(found) / float64(len(citations))
		},
	}
}

// NonBindingComplianceScorer returns a ScorerFn that awards weight * 1.0
// when the model output does NOT contain any phrasing that purports to issue
// a binding legal order, verdict, or judgment, and 0.0 when such phrasing is
// detected.
//
// This scorer ignores the expected string; it only examines the model output.
func NonBindingComplianceScorer(weight float64) RubricCriteria {
	return RubricCriteria{
		Name:   "non_binding_compliance",
		Weight: weight,
		Fn: func(got, _ string) float64 {
			lower := strings.ToLower(got)
			for _, phrase := range finalityWords {
				if strings.Contains(lower, phrase) {
					return 0.0
				}
			}
			return 1.0
		},
	}
}

// JurisdictionAccuracyScorer returns a ScorerFn that awards weight * 1.0
// when the model output mentions the expected jurisdiction string
// (case-insensitive substring), and 0.0 otherwise.
//
// expected is the jurisdiction string embedded in the golden task; the scorer
// checks whether the same string appears in the model output.
func JurisdictionAccuracyScorer(expected string, weight float64) RubricCriteria {
	return RubricCriteria{
		Name:   "jurisdiction_accuracy",
		Weight: weight,
		Fn: func(got, _ string) float64 {
			if expected == "" {
				return 1.0
			}
			if strings.Contains(strings.ToLower(got), strings.ToLower(expected)) {
				return 1.0
			}
			return 0.0
		},
	}
}

// applyRubric scores a model output against a rubric and returns the weighted
// aggregate score (normalised to [0,1]) and per-criterion raw scores.
//
// If rubric is empty, applyRubric returns (0.0, nil).
func applyRubric(got, expected string, rubric []RubricCriteria) (float64, map[string]float64) {
	if len(rubric) == 0 {
		return 0.0, nil
	}

	perCriterion := make(map[string]float64, len(rubric))
	var totalWeight, weightedSum float64

	for _, c := range rubric {
		raw := c.Fn(got, expected)
		// Clamp to [0,1] for safety.
		if raw < 0 {
			raw = 0
		}
		if raw > 1 {
			raw = 1
		}
		perCriterion[c.Name] = raw
		weightedSum += raw * c.Weight
		totalWeight += c.Weight
	}

	if totalWeight == 0 {
		return 0.0, perCriterion
	}
	return weightedSum / totalWeight, perCriterion
}
