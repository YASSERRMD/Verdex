package e2e

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/ingestion"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/jurisdiction"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
	"github.com/YASSERRMD/verdex/packages/provider"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
	"github.com/YASSERRMD/verdex/packages/router"
	"github.com/YASSERRMD/verdex/packages/stt"
	"github.com/YASSERRMD/verdex/packages/treeindex"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// journeyFixture wires one synthetic case's real, in-memory knowledge
// tree plus every dependency a full setup-to-opinion journey needs:
// an ingestion.IngestionOrchestrator (task 2's intake->STT/OCR->
// normalize->segment->classify pipeline, Phase 029), a
// knowledgeapi.KnowledgeAPI over an isolated case-scoped graph/vector
// store (Phase 047/032/044), and a *router.Router dispatching to a
// deterministic, provider-agnostic sequencedProvider (no real LLM call
// anywhere -- see doc.go's "what this suite does not cover"). This
// mirrors packages/reasoningorchestration's own test fixture
// (helpers_test.go) and packages/perf's benchmark_ingestion_test.go
// wiring pattern exactly, composed together into one journey rather
// than run separately.
type journeyFixture struct {
	caseID string

	inner *graph.InMemoryGraphStore
	api   *knowledgeapi.KnowledgeAPI

	ingestionOrch *ingestion.IngestionOrchestrator
}

// newJourneyFixture builds a journeyFixture for a fresh, unique caseID.
func newJourneyFixture(caseIDPrefix string) (*journeyFixture, error) {
	caseID := fmt.Sprintf("%s-%s", caseIDPrefix, uuid.New().String())

	inner := graph.NewInMemoryGraphStore()
	store, err := knowledgeisolation.NewCaseScopedStore(inner, caseID, nil)
	if err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}

	vectors, err := knowledgeisolation.NewCaseScopedVectorStore(
		vectorindex.NewInMemoryVectorStore(vectorindex.IndexConfig{}), caseID, nil,
	)
	if err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}

	indexer, err := treeindex.NewIndexer(store, treeindex.IndexerOptions{})
	if err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}

	retriever, err := hybridretrieval.NewRetriever(vectors, store)
	if err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}

	api, err := knowledgeapi.NewKnowledgeAPI(caseID, store, vectors, indexer, retriever)
	if err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}
	api = api.WithCitationResolver(fakeCitationResolver)

	sttRegistry := stt.NewRegistry()
	if err := sttRegistry.Register("noop", stt.DefaultNoOpSTTProvider()); err != nil {
		return nil, wrapf("newJourneyFixture", err)
	}
	orch := ingestion.NewIngestionOrchestrator(ingestion.OrchestratorConfig{
		STT: stt.NewSTTService(sttRegistry, nil, nil),
	})

	return &journeyFixture{caseID: caseID, inner: inner, api: api, ingestionOrch: orch}, nil
}

// fakeCitationResolver resolves every irac.Node into a deterministic,
// verifiable citation, mirroring
// packages/reasoningorchestration/helpers_test.go's own fakeResolver
// exactly.
func fakeCitationResolver(_ context.Context, node irac.Node) (citation.ResolvedCitation, error) {
	return citation.ResolvedCitation{
		Text:      "Verdex E2E Reporter " + node.ID,
		Origin:    citation.OriginStatute,
		Certainty: citation.CertaintyExact,
	}, nil
}

// seedIssue, seedRule, seedFact, seedGoverns mirror
// packages/reasoningorchestration/helpers_test.go's fixture seeding
// methods, giving a scenario a minimal-but-real reasoning tree (one
// issue, one governing rule, one supporting fact) to drive the
// reasoning pipeline's stages with non-trivial input.
func (f *journeyFixture) seedIssue(id, text string, confidence float64) error {
	node := irac.NewIssueNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	return f.inner.CreateNode(context.Background(), node.Node)
}

func (f *journeyFixture) seedRule(id, text, jurisdictionCode, legalFamily string, confidence float64) error {
	node := irac.NewRuleNode(id, f.caseID, text, jurisdictionCode, legalFamily, time.Now(), confidence, irac.Provenance{})
	return f.inner.CreateNode(context.Background(), node.Node)
}

func (f *journeyFixture) seedFact(id, text string, confidence float64) error {
	node := irac.NewFactNode(id, f.caseID, text, time.Now(), confidence, irac.Provenance{})
	return f.inner.CreateNode(context.Background(), node.Node)
}

func (f *journeyFixture) seedGoverns(ruleID, issueID string) error {
	edge := irac.Edge{FromID: ruleID, ToID: issueID, Type: irac.EdgeGoverns}
	return f.inner.CreateEdge(context.Background(), edge)
}

// seedStandardTree seeds the one-issue/one-rule/one-fact tree every
// setup-to-opinion scenario in this package drives, parameterized by
// jurisdiction code and legal family so multi-jurisdiction variants can
// seed a jurisdiction-appropriate rule (task 3).
func (f *journeyFixture) seedStandardTree(jurisdictionCode, legalFamily string) error {
	if err := f.seedIssue("issue-1", "Did the respondent breach the governing obligation?", 0.9); err != nil {
		return err
	}
	if err := f.seedRule("rule-1", "The governing code requires written evidence of the obligation's terms.", jurisdictionCode, legalFamily, 0.8); err != nil {
		return err
	}
	if err := f.seedGoverns("rule-1", "issue-1"); err != nil {
		return err
	}
	if err := f.seedFact("fact-1", "The parties signed a written memorandum recording the obligation's terms.", 0.75); err != nil {
		return err
	}
	return nil
}

// --- deterministic, provider-agnostic model-call fixture ---
//
// The four responses below are realistic structured completions
// matching each stage's own documented JSON schema (see
// packages/issueagent, packages/firstpartyagent,
// packages/secondpartyagent, packages/synthesisagent), copied from the
// exact fixture strings packages/reasoningorchestration's own test
// suite (helpers_test.go) uses to drive the identical pipeline. No real
// LLM provider is called anywhere in this package -- see doc.go.

const fakeIssueFramingJSON = `{
  "framed_issues": [
    {
      "source_issue_node_id": "issue-1",
      "materiality_score": 0.9,
      "governing_questions": ["Did the respondent breach the governing obligation?"],
      "ambiguities": [],
      "confidence": 0.85
    }
  ]
}`

const fakeFirstPartyArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The obligation was validly formed and memorialized in writing.",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "counterarguments": ["The memorandum lacked a witness signature."],
      "confidence": 0.8
    }
  ]
}`

const fakeSecondPartyArgumentJSON = `{
  "arguments": [
    {
      "issue_node_id": "issue-1",
      "claim": "The memorandum fails the governing code's formal-evidence requirement.",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "rebuts_argument_ids": ["issue-1-0"],
      "counterarguments": [],
      "confidence": 0.7
    }
  ]
}`

const fakeSynthesisJSON = `{
  "conclusions": [
    {
      "issue_node_id": "issue-1",
      "text": "The record suggests the obligation was likely validly formed, though the formal-evidence argument raises a genuine question the evidence does not fully resolve.",
      "favored_party": "first-party",
      "supporting_fact_ids": ["fact-1"],
      "supporting_rule_ids": ["rule-1"],
      "weakest_link": "fact-1 is not independently corroborated",
      "confidence": 0.65
    }
  ]
}`

// standardReasoningResponses returns the four fixed model responses, in
// call order, every setup-to-opinion journey's reasoning phase drives
// StageIssueFraming through StageSynthesis with.
func standardReasoningResponses() []string {
	return []string{
		fakeIssueFramingJSON,
		fakeFirstPartyArgumentJSON,
		fakeSecondPartyArgumentJSON,
		fakeSynthesisJSON,
	}
}

// sequencedProvider is a deterministic, provider-agnostic
// provider.LLMProvider fake that returns one fixed response per call,
// advancing through responses in call order -- mirroring
// packages/reasoningorchestration/helpers_test.go's own
// sequencedProvider exactly. It models the pipeline's real call
// sequence: StageIssueFraming, StageFirstPartyArguments,
// StageSecondPartyArguments, and StageSynthesis each call the Router's
// Chat method exactly once, in exactly this order.
type sequencedProvider struct {
	provider.NoOpProvider
	mu        sync.Mutex
	responses []string
	calls     int
}

func (p *sequencedProvider) ID() string { return "e2e-sequenced-fake" }

func (p *sequencedProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	idx := p.calls
	p.calls++
	if idx >= len(p.responses) {
		return nil, fmt.Errorf("e2e: sequencedProvider: no more responses configured (call %d)", idx)
	}
	return &provider.ChatResponse{
		ID:           fmt.Sprintf("e2e-fake-%d", idx),
		Content:      p.responses[idx],
		FinishReason: "stop",
	}, nil
}

// newSequencedRouter builds a *router.Router backed by a
// sequencedProvider carrying responses, routed via a FallbackChain so
// no explicit TaskRoutes configuration is required -- mirroring every
// Part-5 reasoning package's own test helper convention.
func newSequencedRouter(responses []string) (*router.Router, error) {
	p := &sequencedProvider{responses: responses}

	registry := provider.NewRegistry()
	if err := registry.Register(p.ID(), p); err != nil {
		return nil, wrapf("newSequencedRouter", err)
	}

	r, err := router.NewRouter(router.RouterConfig{
		Registry: registry,
		Policy:   router.RoutingPolicy{FallbackChain: []string{p.ID()}},
	})
	if err != nil {
		return nil, wrapf("newSequencedRouter", err)
	}
	return r, nil
}

// authenticatedContext returns a context.Context carrying an
// authenticated identity.User holding roles, mirroring every sibling
// package's own newTestUser/authedContext test-helper convention. Used
// by scenarios to drive real permission-gated calls
// (signoff.RequireSignoffPermission, identity.HasPermission) rather
// than bypassing them.
func authenticatedContext(tenantID uuid.UUID, roles ...identity.Role) (context.Context, *identity.User) {
	user := &identity.User{
		ID:        uuid.New(),
		TenantID:  tenantID,
		Email:     "e2e-suite@example.test",
		Name:      "E2E Suite Actor",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return identity.WithUser(context.Background(), user), user
}

// defaultReasoningConfig builds a reasoningorchestration.RunConfig
// wired to f's KnowledgeAPI, a fresh deterministic router carrying
// standardReasoningResponses, the given legal family/jurisdiction
// context, and a fresh InMemoryCheckpointStore -- the same wiring every
// scenario in this package's journeys shares. Callers that need
// sign-off enforcement set the returned RunConfig's SignoffGate field
// directly afterward (see signoff_scenario.go); a nil SignoffGate falls
// back to reasoningorchestration's own fail-closed default.
func defaultReasoningConfig(f *journeyFixture, store reasoningorchestration.CheckpointStore, legalFamily, jurisdictionCode, jurisdictionName string) (reasoningorchestration.RunConfig, error) {
	r, err := newSequencedRouter(standardReasoningResponses())
	if err != nil {
		return reasoningorchestration.RunConfig{}, err
	}
	return reasoningorchestration.RunConfig{
		API:              f.api,
		Router:           r,
		FirstParty:       reasoningorchestration.PartyConfig{ID: "first-party", Label: "the first party"},
		SecondParty:      reasoningorchestration.PartyConfig{ID: "second-party", Label: "the second party"},
		LegalFamily:      legalFamily,
		JurisdictionCode: jurisdictionCode,
		JurisdictionName: jurisdictionName,
		Checkpoints:      store,
	}, nil
}

// legalFamilyForJurisdiction resolves the jurisdiction.LegalFamily for
// one of this package's seeded multi-jurisdiction test scenarios,
// looking the jurisdiction up by CountryCode in
// jurisdiction.SeedData() rather than hardcoding a second copy of the
// mapping (task 3 references packages/jurisdiction's legal-family enum
// "by reference", not by duplicating its seed catalogue).
func legalFamilyForJurisdiction(countryCode string) (jurisdiction.Jurisdiction, error) {
	for _, j := range jurisdiction.SeedData() {
		if j.CountryCode == countryCode {
			return j, nil
		}
	}
	return jurisdiction.Jurisdiction{}, fmt.Errorf("e2e: legalFamilyForJurisdiction(%q): %w", countryCode, ErrScenarioNotFound)
}
