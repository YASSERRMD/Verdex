package eval

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// GoldenSet is a versioned collection of EvalTasks that together form the
// authoritative benchmark for a given evaluation run.
type GoldenSet struct {
	// Tasks is the ordered list of evaluation tasks.
	Tasks []EvalTask

	// Version is a semantic or date-stamped label (e.g. "v1.0.0", "2024-01").
	Version string

	// CreatedAt is the UTC instant at which this set was created.
	CreatedAt time.Time
}

// GoldenStore is the persistence contract for GoldenSets.
//
// Implementations must be safe for concurrent use from multiple goroutines.
type GoldenStore interface {
	// Save persists the given GoldenSet.  A GoldenSet with the same Version
	// replaces any previously saved set with that version.
	Save(ctx context.Context, gs GoldenSet) error

	// Load retrieves the GoldenSet with the given version.  Returns
	// ErrNoGoldenSet when no set with that version exists.
	Load(ctx context.Context, version string) (*GoldenSet, error)

	// Latest returns the most-recently saved GoldenSet.  Returns
	// ErrNoGoldenSet when the store is empty.
	Latest(ctx context.Context) (*GoldenSet, error)
}

// InMemoryGoldenStore is a thread-safe in-process implementation of
// GoldenStore.  All data is lost when the process exits.
type InMemoryGoldenStore struct {
	mu     sync.RWMutex
	sets   map[string]GoldenSet
	latest string
}

// NewInMemoryGoldenStore returns a ready-to-use InMemoryGoldenStore.
func NewInMemoryGoldenStore() *InMemoryGoldenStore {
	return &InMemoryGoldenStore{
		sets: make(map[string]GoldenSet),
	}
}

// Save stores gs by its Version.
func (s *InMemoryGoldenStore) Save(_ context.Context, gs GoldenSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets[gs.Version] = gs
	s.latest = gs.Version
	return nil
}

// Load retrieves the GoldenSet for the given version.
func (s *InMemoryGoldenStore) Load(_ context.Context, version string) (*GoldenSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	gs, ok := s.sets[version]
	if !ok {
		return nil, fmt.Errorf("%w: version %q", ErrNoGoldenSet, version)
	}
	cp := gs
	return &cp, nil
}

// Latest returns the most-recently saved GoldenSet.
func (s *InMemoryGoldenStore) Latest(_ context.Context) (*GoldenSet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.latest == "" {
		return nil, ErrNoGoldenSet
	}
	gs := s.sets[s.latest]
	cp := gs
	return &cp, nil
}

// SeedLegalGoldenSet returns a GoldenSet populated with 5+ hand-crafted legal
// reasoning tasks that span all four task categories.
//
// This set is intended for development, CI smoke tests, and as a starting
// point for customisation.  The golden answers are intentionally concise
// reference responses; real production evaluation may require richer rubrics.
func SeedLegalGoldenSet() GoldenSet {
	return GoldenSet{
		Version:   "v1.0.0",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Tasks: []EvalTask{
			{
				ID:       "negligence-001",
				Name:     "Negligence issue spotting",
				Category: CategoryReasoning,
				Prompt: `A pedestrian was injured when a city bus failed to stop at a red light and
struck her in the crosswalk. The city's transit authority claims governmental
immunity. Identify and analyse the legal issues.`,
				GoldenAnswer: `Issues: (1) Negligence — duty of care owed by transit authority to pedestrians,
breach by failing to obey traffic signal, causation, damages. (2) Governmental
immunity — whether the transit authority is protected by sovereign immunity and
whether a motor-vehicle negligence exception applies. (3) Notice requirements —
many jurisdictions require timely filing of a tort claim against a government
entity. Analysis should apply the IRAC framework to each issue.`,
				ScoringRubric: []RubricCriteria{
					ContainsKeywordsScorer([]string{
						"duty", "breach", "causation", "damages", "immunity",
						"negligence", "notice",
					}, 0.4),
					NonBindingComplianceScorer(0.3),
					ContainsKeywordsScorer([]string{"irac", "analysis"}, 0.3),
				},
				Seed: 1001,
			},
			{
				ID:       "statute-apply-001",
				Name:     "Apply statute to facts — GDPR Art. 17",
				Category: CategoryReasoning,
				Prompt: `Under Article 17 of the GDPR (Right to Erasure), a data subject requests
that a social-media platform delete all personal data it holds about her.
The platform argues the data is necessary for compliance with a legal
obligation under EU law. Advise the platform.`,
				GoldenAnswer: `Article 17(3)(b) GDPR provides an exception to the right of erasure where
processing is necessary for compliance with a legal obligation under Union or
Member State law. The platform should: (1) identify the specific legal
obligation requiring retention; (2) retain only the minimum data necessary;
(3) document the legal basis; (4) inform the data subject of the reason for
refusal and their right to lodge a complaint with a supervisory authority.`,
				ScoringRubric: []RubricCriteria{
					ContainsKeywordsScorer([]string{
						"article 17", "erasure", "exception", "legal obligation",
						"retention", "supervisory authority",
					}, 0.4),
					CitationPresenceScorer([]string{"article 17(3)(b)", "gdpr"}, 0.3),
					NonBindingComplianceScorer(0.3),
				},
				Seed: 1002,
			},
			{
				ID:       "precedent-cite-001",
				Name:     "Cite controlling precedent — US 4th Amendment",
				Category: CategoryCitationFidelity,
				Prompt: `Police officers attached a GPS tracking device to a suspect's vehicle
without a warrant and monitored his movements for 28 days. The suspect
seeks suppression of the evidence. Identify the controlling Supreme Court
precedent and explain its holding.`,
				GoldenAnswer: `The controlling case is United States v. Jones, 565 U.S. 400 (2012), in
which the Supreme Court held that attaching a GPS device to a vehicle and
using it to monitor the vehicle's movements constitutes a "search" under the
Fourth Amendment. The majority relied on the trespass theory: physically
occupying private property to obtain information is a search. Justice
Alito's concurrence addressed the reasonable-expectation-of-privacy analysis
for prolonged surveillance.`,
				ScoringRubric: []RubricCriteria{
					CitationPresenceScorer([]string{
						"united states v. jones", "565 u.s. 400", "2012",
					}, 0.5),
					ContainsKeywordsScorer([]string{
						"fourth amendment", "gps", "search", "trespass",
						"reasonable expectation of privacy",
					}, 0.3),
					NonBindingComplianceScorer(0.2),
				},
				Seed: 1003,
			},
			{
				ID:       "jurisdiction-001",
				Name:     "Jurisdiction identification — international contract",
				Category: CategoryJurisdictionAccuracy,
				Prompt: `A software company incorporated in Delaware, USA, entered into a SaaS
agreement with a German GmbH. The agreement has no choice-of-law clause.
A dispute arises over unpaid invoices. Which jurisdiction's law is likely
to govern, and which court is likely to have jurisdiction?`,
				GoldenAnswer: `Without a choice-of-law clause, courts typically apply conflict-of-laws
rules. In the EU, Regulation (EC) No 593/2008 (Rome I) would apply if
proceedings are brought in Germany: under Art. 4(1)(b), the law of the
country of the service provider's habitual residence governs, which points
to the USA (Delaware law). However, an EU court may apply mandatory
provisions of German law. For jurisdiction, if suit is filed in Germany,
the Brussels Ibis Regulation (Regulation (EU) No 1215/2012) governs;
the defendant may be sued in its place of domicile or where the obligation
was performed. If suit is filed in the USA, personal jurisdiction over
the German GmbH requires minimum contacts analysis under International
Shoe Co. v. Washington, 326 U.S. 310 (1945).`,
				ScoringRubric: []RubricCriteria{
					JurisdictionAccuracyScorer("delaware", 0.2),
					JurisdictionAccuracyScorer("germany", 0.2),
					ContainsKeywordsScorer([]string{
						"rome i", "conflict of laws", "choice of law",
						"jurisdiction", "domicile",
					}, 0.3),
					CitationPresenceScorer([]string{"rome i", "brussels ibis"}, 0.1),
					NonBindingComplianceScorer(0.2),
				},
				Seed: 1004,
			},
			{
				ID:       "contract-retrieval-001",
				Name:     "Retrieve limitation period — breach of contract",
				Category: CategoryRetrieval,
				Prompt: `Under New York law, what is the statute of limitations for a written
contract claim, and when does the limitations period begin to run?`,
				GoldenAnswer: `Under New York CPLR § 213(2), the statute of limitations for an action
upon a contractual obligation or liability, express or implied, is six
years. The period begins to run from the date of the breach (accrual),
not from the date the plaintiff discovers the breach, unless the discovery
rule applies (e.g., in cases involving fraud or fiduciary duties).`,
				ScoringRubric: []RubricCriteria{
					ContainsKeywordsScorer([]string{
						"six years", "6 years", "cplr", "213", "accrual", "breach",
					}, 0.5),
					JurisdictionAccuracyScorer("new york", 0.2),
					CitationPresenceScorer([]string{"cplr § 213", "cplr 213"}, 0.1),
					NonBindingComplianceScorer(0.2),
				},
				Seed: 1005,
			},
			{
				ID:       "constitutional-001",
				Name:     "First Amendment — compelled speech",
				Category: CategoryReasoning,
				Prompt: `A state law requires all licensed attorneys to include a specific
government-approved disclaimer on their websites stating that prior
results do not guarantee a similar outcome. An attorney challenges the
law as compelled speech under the First Amendment. Analyse the claim.`,
				GoldenAnswer: `The attorney's challenge implicates the compelled-speech doctrine.
Under Wooley v. Maynard, 430 U.S. 705 (1977), and Barnette, the government
generally cannot compel individuals to speak a government-prescribed message.
However, under Zauderer v. Office of Disciplinary Counsel, 471 U.S. 626
(1985), purely factual and uncontroversial disclosure requirements for
commercial speech may survive under a rational-basis-like standard if they
are reasonably related to a substantial government interest. Courts have
split on whether attorney advertising disclaimers satisfy Zauderer.
The attorney should argue the disclaimer is controversial or ideological
to trigger heightened scrutiny.`,
				ScoringRubric: []RubricCriteria{
					CitationPresenceScorer([]string{"wooley", "zauderer", "barnette"}, 0.4),
					ContainsKeywordsScorer([]string{
						"compelled speech", "first amendment", "commercial speech",
						"rational basis", "scrutiny", "disclosure",
					}, 0.3),
					NonBindingComplianceScorer(0.3),
				},
				Seed: 1006,
			},
		},
	}
}
