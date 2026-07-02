package reasoningorchestration

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

// runIssueFraming runs StageIssueFraming: packages/issueagent.Analyze,
// framing and ranking every issue in the case's tree. This is the
// pipeline's first stage and its only stage with a genuine, documented
// concurrency opportunity: resolveWeights (this case's
// reasoningprofile.Weights) needs only cfg.LegalFamily, not this stage's
// own output, and this stage needs nothing from resolveWeights either —
// see doc/reasoning-orchestration.md for the full argument. The two run
// concurrently here via a plain goroutine plus channel, the simplest
// correct construction for "start two independent, side-effect-free
// computations and join both before proceeding" (no shared mutable state
// to protect, so no mutex/errgroup machinery is needed beyond the
// channel handoff itself).
func runIssueFraming(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	weightsDone := make(chan reasoningprofile.Weights, 1)
	go func() {
		weightsDone <- resolveWeights(cfg.LegalFamily)
	}()

	var opts []issueagent.Option
	if cfg.Locale != "" {
		opts = append(opts, issueagent.WithLocale(cfg.Locale))
	}
	if cfg.LegalFamily != "" {
		opts = append(opts, issueagent.WithLegalFamily(cfg.LegalFamily))
	}
	if cfg.JurisdictionName != "" {
		opts = append(opts, issueagent.WithJurisdictionName(cfg.JurisdictionName))
	}
	if cfg.JurisdictionCode != "" {
		opts = append(opts, issueagent.WithJurisdictionCode(cfg.JurisdictionCode))
	}
	agent, err := issueagent.New(cfg.API, opts...)
	if err != nil {
		<-weightsDone
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: construct issue agent: %w", err)
	}

	result, agentResult, err := issueagent.Analyze(ctx, agent, caseID, issueagent.AnalyzeConfig{
		Router:   cfg.Router,
		Budget:   cfg.Budget.PerStageBudget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	<-weightsDone // join: see doc comment above for why this is safe to discard here.
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: issue framing: %w", err)
	}
	if len(result.Issues) == 0 {
		return Checkpoint{}, ErrNoFramedIssues
	}

	pc.issues = result
	return Checkpoint{Stage: StageIssueFraming, IssueAnalysis: result, IssueFramingRun: agentResult}, nil
}

// runFirstPartyArguments runs StageFirstPartyArguments:
// packages/firstpartyagent.Argue, constructing the first party's
// affirmative case over pc.issues.
func runFirstPartyArguments(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	var opts []firstpartyagent.Option
	if cfg.Locale != "" {
		opts = append(opts, firstpartyagent.WithLocale(cfg.Locale))
	}
	if cfg.LegalFamily != "" {
		opts = append(opts, firstpartyagent.WithLegalFamily(cfg.LegalFamily))
	}
	if cfg.JurisdictionName != "" {
		opts = append(opts, firstpartyagent.WithJurisdictionName(cfg.JurisdictionName))
	}
	if cfg.JurisdictionCode != "" {
		opts = append(opts, firstpartyagent.WithJurisdictionCode(cfg.JurisdictionCode))
	}
	if cfg.FirstParty.Label != "" {
		opts = append(opts, firstpartyagent.WithPartyLabel(cfg.FirstParty.Label))
	}
	agent, err := firstpartyagent.New(cfg.API, firstpartyagent.PartyID(cfg.FirstParty.ID), pc.issues.Issues, opts...)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: construct first-party agent: %w", err)
	}

	result, agentResult, err := firstpartyagent.Argue(ctx, agent, caseID, firstpartyagent.ArgueConfig{
		Router:   cfg.Router,
		Budget:   cfg.Budget.PerStageBudget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: first-party arguments: %w", err)
	}

	pc.firstParty = result
	return Checkpoint{Stage: StageFirstPartyArguments, FirstPartyArguments: result, FirstPartyRun: agentResult}, nil
}

// runSecondPartyArguments runs StageSecondPartyArguments:
// packages/secondpartyagent.Argue, constructing the second party's case
// and rebuttal over pc.issues and pc.firstParty.
func runSecondPartyArguments(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	var opts []secondpartyagent.Option
	if cfg.Locale != "" {
		opts = append(opts, secondpartyagent.WithLocale(cfg.Locale))
	}
	if cfg.LegalFamily != "" {
		opts = append(opts, secondpartyagent.WithLegalFamily(cfg.LegalFamily))
	}
	if cfg.JurisdictionName != "" {
		opts = append(opts, secondpartyagent.WithJurisdictionName(cfg.JurisdictionName))
	}
	if cfg.JurisdictionCode != "" {
		opts = append(opts, secondpartyagent.WithJurisdictionCode(cfg.JurisdictionCode))
	}
	if cfg.SecondParty.Label != "" {
		opts = append(opts, secondpartyagent.WithPartyLabel(cfg.SecondParty.Label))
	}
	agent, err := secondpartyagent.New(cfg.API, secondpartyagent.PartyID(cfg.SecondParty.ID), pc.issues.Issues, pc.firstParty, opts...)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: construct second-party agent: %w", err)
	}

	result, agentResult, err := secondpartyagent.Argue(ctx, agent, caseID, secondpartyagent.ArgueConfig{
		Router:   cfg.Router,
		Budget:   cfg.Budget.PerStageBudget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: second-party arguments: %w", err)
	}

	pc.secondParty = result
	return Checkpoint{Stage: StageSecondPartyArguments, SecondPartyArguments: result, SecondPartyRun: agentResult}, nil
}

// runEvidenceWeighing runs StageEvidenceWeighing:
// packages/evidenceweighing.Weigh, a deterministic pure function over
// both parties' ArgumentSets and the case's fact catalog. Fetches the
// case's FactRefs via knowledgeapi.GetTree.
func runEvidenceWeighing(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	snap, err := fetchTreeSnapshot(ctx, cfg.API, caseID)
	if err != nil {
		return Checkpoint{}, err
	}

	// snap.allIssueNodeIDs (every IssueNode in the tree) is used here
	// rather than only pc.issues.Issues (the issues issueagent chose to
	// frame): DetectGaps's GapKindUncitedIssue finding is meant to catch
	// an issue nobody argued evidence for at all, which is a strictly
	// broader set than "issues the framing agent successfully framed".
	result, err := evidenceweighing.Weigh(evidenceweighing.WeighRequest{
		CaseID:       caseID,
		Facts:        snap.facts,
		IssueNodeIDs: snap.allIssueNodeIDs,
		FirstParty:   pc.firstParty,
		SecondParty:  pc.secondParty,
		LegalFamily:  evidenceweighing.LegalFamily(cfg.LegalFamily),
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: evidence weighing: %w", err)
	}

	pc.evidence = result
	return Checkpoint{Stage: StageEvidenceWeighing, Evidence: result}, nil
}

// runLawApplication runs StageLawApplication: packages/lawapplication.
// Apply, a deterministic function mapping controlling authority to each
// issue given both parties' ArgumentSets and the evidence-weighing
// result. Fetches the case's RuleRefs and governs-edges via
// knowledgeapi.GetTree, and wires citation resolution through
// knowledgeapi.ResolveCitation.
func runLawApplication(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	snap, err := fetchTreeSnapshot(ctx, cfg.API, caseID)
	if err != nil {
		return Checkpoint{}, err
	}

	result, err := lawapplication.Apply(lawapplication.Request{
		CaseID:         caseID,
		Issues:         snap.issueInputs(pc.issues.Issues),
		Rules:          snap.rules,
		FirstParty:     pc.firstParty,
		SecondParty:    pc.secondParty,
		Evidence:       pc.evidence,
		LegalFamily:    lawapplication.LegalFamily(cfg.LegalFamily),
		CitationLookup: citationLookup(ctx, cfg.API, caseID),
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: law application: %w", err)
	}

	pc.law = result
	return Checkpoint{Stage: StageLawApplication, Law: result}, nil
}

// runSynthesis runs StageSynthesis: packages/synthesisagent.Synthesize,
// producing the case's draft Opinion from every prior stage's output.
func runSynthesis(ctx context.Context, cfg RunConfig, caseID string, pc *pipelineContext) (Checkpoint, error) {
	var opts []synthesisagent.Option
	if cfg.Locale != "" {
		opts = append(opts, synthesisagent.WithLocale(cfg.Locale))
	}
	if cfg.LegalFamily != "" {
		opts = append(opts, synthesisagent.WithLegalFamily(cfg.LegalFamily))
	}
	if cfg.JurisdictionName != "" {
		opts = append(opts, synthesisagent.WithJurisdictionName(cfg.JurisdictionName))
	}
	if cfg.JurisdictionCode != "" {
		opts = append(opts, synthesisagent.WithJurisdictionCode(cfg.JurisdictionCode))
	}
	agent, err := synthesisagent.New(cfg.API, pc.issues.Issues, pc.firstParty, pc.secondParty, pc.evidence, pc.law, opts...)
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: construct synthesis agent: %w", err)
	}

	result, agentResult, err := synthesisagent.Synthesize(ctx, agent, caseID, synthesisagent.SynthesizeConfig{
		Router:   cfg.Router,
		Budget:   cfg.Budget.PerStageBudget,
		Seed:     cfg.Seed,
		TenantID: cfg.TenantID,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: synthesis: %w", err)
	}

	pc.opinion = result
	return Checkpoint{Stage: StageSynthesis, Opinion: result, SynthesisRun: agentResult}, nil
}

// runUncertaintySurfacing runs StageUncertaintySurfacing:
// packages/uncertainty.Surface, a deterministic function ranking every
// reason to doubt part of the draft analysis, given every prior stage's
// output.
func runUncertaintySurfacing(caseID string, pc *pipelineContext) (Checkpoint, error) {
	report, err := uncertainty.Surface(uncertainty.Request{
		CaseID:   caseID,
		Issues:   pc.issues,
		Evidence: pc.evidence,
		Law:      pc.law,
		Opinion:  pc.opinion,
	})
	if err != nil {
		return Checkpoint{}, fmt.Errorf("reasoningorchestration: uncertainty surfacing: %w", err)
	}

	pc.uncertainty = report
	return Checkpoint{Stage: StageUncertaintySurfacing, Uncertainty: report}, nil
}
