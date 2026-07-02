package category

import (
	"context"
	"sort"
	"strings"
)

// Suggestion carries one candidate Category proposed for a piece of case
// text, together with a Confidence score reflecting how strong the
// suggester's signal was.
type Suggestion struct {
	// Category is the candidate category.
	Category Category

	// Confidence is this suggestion's confidence score, in the closed
	// interval [0, 1].
	Confidence float64
}

// Suggester proposes candidate Categories for a piece of case text, scoped
// to the categories present in a given Taxonomy.
//
// This mirrors packages/evidence's Classifier interface: the default
// implementation in this file (KeywordSuggester) is a deterministic
// function of lexical pattern matching, with no dependency on a machine
// learning model at runtime. A future phase can swap in a real model-backed
// suggester by implementing this same interface — no caller needs to
// change.
type Suggester interface {
	// Suggest inspects text and returns candidate Suggestions drawn from
	// taxonomy, ordered by descending Confidence. Returns ErrEmptyInput if
	// text is empty or whitespace-only.
	Suggest(ctx context.Context, text string, taxonomy Taxonomy) ([]Suggestion, error)
}

// KeywordSuggester is the default, deterministic Suggester implementation.
// It scores each top-level CategoryCode present anywhere in taxonomy by
// counting lexical keyword matches against text, and returns the
// candidates whose score is non-zero, sorted by descending confidence.
//
// KeywordSuggester performs no machine learning and calls out to no
// external service, so its output is fully reproducible given the same
// input text and taxonomy.
type KeywordSuggester struct{}

// NewKeywordSuggester constructs a KeywordSuggester. It has no
// configuration today; the constructor exists so call sites can be updated
// uniformly if configuration is added later.
func NewKeywordSuggester() *KeywordSuggester {
	return &KeywordSuggester{}
}

// categoryKeywords maps each top-level CategoryCode to the lexical markers
// that suggest a piece of case text belongs to it. Keywords are matched
// case-insensitively as substrings.
var categoryKeywords = map[CategoryCode][]string{
	CodeCivil: {
		"breach of contract", "tort", "negligence", "civil suit",
		"plaintiff", "damages", "injunction", "specific performance",
	},
	CodeCriminal: {
		"prosecution", "the accused", "criminal charge", "indictment",
		"felony", "misdemeanor", "sentencing", "guilty", "acquittal",
		"penal code", "prison", "bail",
	},
	CodeDomesticViolence: {
		"domestic violence", "protective order", "restraining order",
		"spousal abuse", "intimate partner violence", "battery at home",
		"cohabitant abuse",
	},
	CodeConsumer: {
		"consumer protection", "defective product", "warranty claim",
		"unfair trade practice", "consumer complaint", "product liability",
		"deceptive advertising",
	},
	CodeFamily: {
		"divorce", "child custody", "alimony", "child support",
		"marital dissolution", "adoption", "guardianship", "visitation",
	},
	CodeCommercial: {
		"commercial dispute", "shareholder", "partnership agreement",
		"breach of commercial contract", "trade dispute", "merger",
		"corporate governance", "bankruptcy", "insolvency",
	},
	CodeLabor: {
		"wrongful termination", "unpaid wages", "workplace discrimination",
		"labor dispute", "employment contract", "collective bargaining",
		"unfair dismissal", "workers' compensation",
	},
}

// Suggest implements Suggester. For each top-level category present in
// taxonomy for any jurisdiction (categoryKeywords is jurisdiction-agnostic;
// filtering to a specific jurisdiction's valid set is the caller's
// responsibility via validate.go), it counts keyword matches in text and
// converts the count into a bounded confidence score.
func (s *KeywordSuggester) Suggest(_ context.Context, text string, taxonomy Taxonomy) ([]Suggestion, error) {
	if strings.TrimSpace(text) == "" {
		return nil, ErrEmptyInput
	}

	lower := strings.ToLower(text)

	// Collect the distinct set of Categories available anywhere in
	// taxonomy, keyed by code, so a suggestion is only returned for a
	// category the taxonomy actually recognizes.
	available := make(map[CategoryCode]Category)
	for _, cats := range taxonomy {
		for code, cat := range cats {
			if _, ok := available[code]; !ok {
				available[code] = cat
			}
		}
	}

	var suggestions []Suggestion
	for code, cat := range available {
		keywords, ok := categoryKeywords[code]
		if !ok {
			continue
		}
		matches := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		suggestions = append(suggestions, Suggestion{
			Category:   cat,
			Confidence: confidenceFromMatches(matches),
		})
	}

	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Confidence != suggestions[j].Confidence {
			return suggestions[i].Confidence > suggestions[j].Confidence
		}
		return suggestions[i].Category.Code < suggestions[j].Category.Code
	})

	return suggestions, nil
}

// confidenceFromMatches converts a raw keyword-match count into a bounded
// confidence score in (0, 1]: a single match scores 0.5, each additional
// match adds 0.15, capped at 0.95 so a keyword-only heuristic never claims
// full certainty (full certainty is reserved for a ManualOverride, see
// override.go).
func confidenceFromMatches(matches int) float64 {
	if matches <= 0 {
		return 0
	}
	score := 0.5 + float64(matches-1)*0.15
	if score > 0.95 {
		score = 0.95
	}
	return score
}
