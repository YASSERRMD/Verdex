package e2e

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/reasoningorchestration"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
)

// journeyOptions parameterizes runFullJourney so the various scenario
// variants (multi-jurisdiction, sign-off enforcement) can drive the
// identical underlying journey with a different jurisdiction context
// or SignoffGate, rather than each duplicating the
// intake->reasoning->guardrail wiring.
type journeyOptions struct {
	// JurisdictionCode/JurisdictionName/LegalFamily seed both the
	// ingestion job's language-adjacent context and the
	// reasoningorchestration.RunConfig's jurisdiction fields.
	JurisdictionCode string
	JurisdictionName string
	LegalFamily      string

	// IngestionText, when non-empty, is used verbatim as the "audio
	// transcript" payload driven through the intake phase, letting
	// multilingual scenarios (task 4) submit real Arabic/Urdu/Tamil/
	// English source text. Empty falls back to synthAudioPayload.
	IngestionText string

	// IngestionLanguage seeds StageNormalize's language fallback.
	IngestionLanguage string

	// SignoffGate, if non-nil, is threaded into
	// reasoningorchestration.RunConfig.SignoffGate. Nil uses that
	// package's own fail-closed guardrail.NoSignoffRecordedGate
	// default.
	SignoffGate guardrail.SignoffGate
}

// fullJourneyOutcome is what runFullJourney produces: the real
// WorkflowState from the ingestion phase, the real RunResult from the
// reasoning phase, the CheckpointStore that run persisted to (so a
// caller can read back any stage's real typed Checkpoint, notably
// StageGuardrailCheck's GuardrailApproved), and the fixture itself (so
// a caller can inspect the underlying case's graph/vector stores for
// isolation assertions).
type fullJourneyOutcome struct {
	fixture         *journeyFixture
	ingestionJobID  string
	ingestionStage  string
	checkpoints     reasoningorchestration.CheckpointStore
	reasoningResult reasoningorchestration.RunResult
	weights         ScenarioWeights
}

// runFullJourney composes a genuine, in-process, in-memory,
// no-Docker "setup to draft opinion" journey (task 2's centerpiece):
//
//  1. Seed a minimal reasoning tree for the synthetic case.
//  2. Drive a synthetic exhibit through
//     ingestion.IngestionOrchestrator.Process end to end
//     (StageIntake -> StageExtraction -> StageNormalize ->
//     StageSegment -> StageClassify).
//  3. Drive the case through
//     reasoningorchestration.Run end to end (issue framing ->
//     first/second-party arguments -> evidence weighing -> law
//     application -> synthesis -> uncertainty surfacing -> the
//     guardrail check), resolving reasoningprofile.Weights for
//     opts.LegalFamily alongside it so multi-jurisdiction scenario
//     variants can assert two different families produce two
//     different profiles.
//
// Every call here is real: the same orchestrator/pipeline entry points
// packages/perf's own benchmarks and packages/reasoningorchestration's
// own tests exercise, not a description or a stub standing in for one.
func runFullJourney(ctx context.Context, caseIDPrefix string, opts journeyOptions) (*fullJourneyOutcome, error) {
	f, err := newJourneyFixture(caseIDPrefix)
	if err != nil {
		return nil, wrapf("runFullJourney", err)
	}

	jurisdictionCode := opts.JurisdictionCode
	if jurisdictionCode == "" {
		jurisdictionCode = "US-CA"
	}
	legalFamily := opts.LegalFamily
	if legalFamily == "" {
		legalFamily = "common_law"
	}

	if err := f.seedStandardTree(jurisdictionCode, legalFamily); err != nil {
		return nil, wrapf("runFullJourney: seed tree", err)
	}

	jobID := f.caseID + "-job-1"
	ingestState, err := f.runIntakePhase(ctx, jobID, opts.IngestionText, opts.IngestionLanguage)
	if err != nil {
		return nil, wrapf("runFullJourney: ingestion phase", err)
	}

	checkpoints := reasoningorchestration.NewInMemoryCheckpointStore()
	cfg, err := defaultReasoningConfig(f, checkpoints, legalFamily, jurisdictionCode, opts.JurisdictionName)
	if err != nil {
		return nil, wrapf("runFullJourney: build reasoning config", err)
	}
	if opts.SignoffGate != nil {
		cfg.SignoffGate = opts.SignoffGate
	}

	// reasoningorchestration.Run's tree-reading stages fetch through
	// knowledgeapi.KnowledgeAPI, which requires an authenticated
	// identity.User on ctx (see knowledgeapi's own unauthenticated-
	// request guard) -- mirroring
	// packages/reasoningorchestration/helpers_test.go's own
	// authedContext() convention exactly, rather than propagating the
	// caller's raw, unauthenticated ctx into a call that requires one.
	reasoningCtx := authenticatedContext(uuid.Nil, identity.RoleAdvocate)
	result := reasoningorchestration.Run(reasoningCtx, f.caseID, cfg)

	weights := resolveScenarioWeights(legalFamily)

	return &fullJourneyOutcome{
		fixture:         f,
		ingestionJobID:  jobID,
		ingestionStage:  string(ingestState.Stage),
		checkpoints:     checkpoints,
		reasoningResult: result,
		weights:         weights,
	}, nil
}

// guardrailApproved reads back whether StageGuardrailCheck's real
// Checkpoint recorded GuardrailApproved true, from o's own
// CheckpointStore. Returns false (not an error) if the stage never
// completed (e.g. the journey failed or halted earlier) -- an
// incomplete journey never had a chance to approve anything, which is
// itself the correct, honest answer rather than a propagated error.
func (o *fullJourneyOutcome) guardrailApproved(ctx context.Context) bool {
	cp, err := o.checkpoints.GetCheckpoint(ctx, o.fixture.caseID, reasoningorchestration.StageGuardrailCheck)
	if err != nil {
		return false
	}
	return cp.GuardrailApproved
}

// toScenarioResult converts a fullJourneyOutcome into the public
// ScenarioResult shape, classifying Outcome from the real
// reasoningorchestration.TerminationReason reached. Every caller
// currently owns its own success-path Detail message (overwritten
// after this call returns for OutcomePassed) and relies on this
// method's own generated message for the failure path, so Detail is
// not a caller-supplied parameter.
func (o *fullJourneyOutcome) toScenarioResult(ctx context.Context, startedAt time.Time) ScenarioResult {
	outcome := OutcomePassed
	detail := ""
	if o.reasoningResult.State.Termination != reasoningorchestration.TerminationComplete {
		outcome = OutcomeFailed
		detail = "reasoning phase did not reach TerminationComplete: " + string(o.reasoningResult.State.Termination)
	}

	return ScenarioResult{
		Outcome:                  outcome,
		Detail:                   detail,
		CaseID:                   o.fixture.caseID,
		IngestionStage:           o.ingestionStage,
		ReasoningTermination:     string(o.reasoningResult.State.Termination),
		CompletedReasoningStages: len(o.reasoningResult.State.CompletedStages),
		GuardrailApproved:        o.guardrailApproved(ctx),
		ResolvedWeights:          o.weights,
		StartedAt:                startedAt,
		FinishedAt:               time.Now().UTC(),
	}
}

// resolveScenarioWeights resolves the real
// reasoningprofile.WeightsForFamily for legalFamily (the same
// resolution reasoningorchestration.Run performs internally alongside
// issue framing, see resolveWeights in
// packages/reasoningorchestration/run.go) and copies it into this
// package's plain-data ScenarioWeights mirror (see types.go). An
// unrecognized legalFamily resolves to the zero ScenarioWeights value
// rather than erroring: a multi-jurisdiction scenario comparing two
// families' resolved weights treats "no profile resolved" as a
// legitimate, distinguishable outcome in its own right.
func resolveScenarioWeights(legalFamily string) ScenarioWeights {
	w, err := reasoningprofile.WeightsForFamily(reasoningprofile.Family(legalFamily))
	if err != nil {
		return ScenarioWeights{}
	}
	return ScenarioWeights{
		TestimonyEmphasis:   w.TestimonyEmphasis,
		DocumentaryEmphasis: w.DocumentaryEmphasis,
		StatuteEmphasis:     w.StatuteEmphasis,
		PrecedentEmphasis:   w.PrecedentEmphasis,
	}
}
