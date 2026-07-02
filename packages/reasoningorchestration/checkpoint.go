package reasoningorchestration

import (
	"context"
	"sync"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
	"github.com/YASSERRMD/verdex/packages/uncertainty"
)

// Checkpoint is one stage's persisted, typed output for a case. Exactly
// one of the typed fields is meaningful for any given Stage value (see
// each field's doc comment); the others are left at their zero value.
// This "one struct, many optional fields" shape (rather than a
// CheckpointStore keyed by Stage with an `any` payload) keeps every
// stored value statically typed end to end, so Resume and a future
// case-workspace UI can read back e.g. a case's IssueAnalysisResult
// without a type assertion.
type Checkpoint struct {
	// Stage identifies which stage this Checkpoint's non-zero field
	// corresponds to.
	Stage Stage

	// IssueAnalysis is populated when Stage is StageIssueFraming.
	IssueAnalysis issueagent.IssueAnalysisResult

	// FirstPartyArguments is populated when Stage is
	// StageFirstPartyArguments.
	FirstPartyArguments firstpartyagent.ArgumentSet

	// SecondPartyArguments is populated when Stage is
	// StageSecondPartyArguments.
	SecondPartyArguments secondpartyagent.ArgumentSet

	// Evidence is populated when Stage is StageEvidenceWeighing.
	Evidence evidenceweighing.Result

	// Law is populated when Stage is StageLawApplication.
	Law lawapplication.Result

	// Opinion is populated when Stage is StageSynthesis.
	Opinion synthesisagent.Opinion

	// Uncertainty is populated when Stage is StageUncertaintySurfacing.
	Uncertainty uncertainty.Report

	// GuardrailApproved is populated when Stage is StageGuardrailCheck:
	// true if CheckText and the sign-off gate both passed.
	GuardrailApproved bool

	// IssueFramingRun is the agentframework.Result (including the full
	// step-by-step Scratchpad of every model call, tool call, and
	// observation) produced alongside IssueAnalysis when Stage is
	// StageIssueFraming. issueagent.Analyze already returns this value;
	// it is captured here so a later auditability layer (see
	// packages/reasoningtrace) has a real data source for "every agent
	// step and tool call this stage took" instead of only this stage's
	// typed IssueAnalysisResult.
	IssueFramingRun agentframework.Result

	// FirstPartyRun is the agentframework.Result produced alongside
	// FirstPartyArguments when Stage is StageFirstPartyArguments. See
	// IssueFramingRun's doc comment for why this is captured.
	FirstPartyRun agentframework.Result

	// SecondPartyRun is the agentframework.Result produced alongside
	// SecondPartyArguments when Stage is StageSecondPartyArguments. See
	// IssueFramingRun's doc comment for why this is captured.
	SecondPartyRun agentframework.Result

	// SynthesisRun is the agentframework.Result produced alongside
	// Opinion when Stage is StageSynthesis. See IssueFramingRun's doc
	// comment for why this is captured.
	SynthesisRun agentframework.Result
}

// CheckpointStore persists a Checkpoint per (CaseID, Stage) and the
// overall RunState per CaseID, so a caller can retrieve any completed
// stage's typed result after a run — doubling as both Resume's mechanism
// and an audit trail — without recomputing (and re-billing) an expensive
// LLM call. Mirrors evidenceweighing.Repository's and
// lawapplication.Repository's shared Save/Get/DeleteByCase convention,
// extended with the (CaseID, Stage) compound key this package's
// multi-stage nature requires.
type CheckpointStore interface {
	// SaveCheckpoint persists checkpoint for caseID, keyed by
	// (caseID, checkpoint.Stage). Calling SaveCheckpoint again for the
	// same (caseID, Stage) overwrites the previous value (idempotent
	// upsert), mirroring evidenceweighing.Repository.Save's convention.
	SaveCheckpoint(ctx context.Context, caseID string, checkpoint Checkpoint) error

	// GetCheckpoint returns the Checkpoint saved for (caseID, stage), or
	// ErrCheckpointNotFound if none was ever saved.
	GetCheckpoint(ctx context.Context, caseID string, stage Stage) (Checkpoint, error)

	// SaveRunState persists state, keyed by state.CaseID. Calling
	// SaveRunState again for the same CaseID overwrites the previous
	// value.
	SaveRunState(ctx context.Context, state RunState) error

	// GetRunState returns the RunState saved for caseID, or
	// ErrRunNotFound if none was ever saved.
	GetRunState(ctx context.Context, caseID string) (RunState, error)

	// DeleteByCase removes every Checkpoint and the RunState saved for
	// caseID. Not an error to delete a case with nothing saved.
	DeleteByCase(ctx context.Context, caseID string) error
}

// caseCheckpoints holds every Checkpoint and the RunState saved for one
// case.
type caseCheckpoints struct {
	byStage map[Stage]Checkpoint
	state   RunState
	hasRun  bool
}

// InMemoryCheckpointStore is a fully in-memory CheckpointStore
// implementation backed by a map, mirroring
// evidenceweighing.InMemoryRepository's and lawapplication.
// InMemoryRepository's shared convention of being the default,
// always-available implementation used by tests and by any deployment
// that does not yet need a durable backend. Safe for concurrent use.
type InMemoryCheckpointStore struct {
	mu    sync.RWMutex
	cases map[string]*caseCheckpoints
}

// NewInMemoryCheckpointStore constructs an empty InMemoryCheckpointStore.
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{cases: make(map[string]*caseCheckpoints)}
}

// SaveCheckpoint implements CheckpointStore.
func (s *InMemoryCheckpointStore) SaveCheckpoint(_ context.Context, caseID string, checkpoint Checkpoint) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.cases[caseID]
	if c == nil {
		c = &caseCheckpoints{byStage: make(map[Stage]Checkpoint)}
		s.cases[caseID] = c
	}
	c.byStage[checkpoint.Stage] = checkpoint
	return nil
}

// GetCheckpoint implements CheckpointStore.
func (s *InMemoryCheckpointStore) GetCheckpoint(_ context.Context, caseID string, stage Stage) (Checkpoint, error) {
	if caseID == "" {
		return Checkpoint{}, ErrEmptyCaseID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.cases[caseID]
	if !ok {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	cp, ok := c.byStage[stage]
	if !ok {
		return Checkpoint{}, ErrCheckpointNotFound
	}
	return cp, nil
}

// SaveRunState implements CheckpointStore.
func (s *InMemoryCheckpointStore) SaveRunState(_ context.Context, state RunState) error {
	if state.CaseID == "" {
		return ErrEmptyCaseID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	c := s.cases[state.CaseID]
	if c == nil {
		c = &caseCheckpoints{byStage: make(map[Stage]Checkpoint)}
		s.cases[state.CaseID] = c
	}
	c.state = state.Clone()
	c.hasRun = true
	return nil
}

// GetRunState implements CheckpointStore.
func (s *InMemoryCheckpointStore) GetRunState(_ context.Context, caseID string) (RunState, error) {
	if caseID == "" {
		return RunState{}, ErrEmptyCaseID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.cases[caseID]
	if !ok || !c.hasRun {
		return RunState{}, ErrRunNotFound
	}
	return c.state.Clone(), nil
}

// DeleteByCase implements CheckpointStore.
func (s *InMemoryCheckpointStore) DeleteByCase(_ context.Context, caseID string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.cases, caseID)
	return nil
}

// Len returns the number of cases currently stored. Useful for tests
// asserting on store state.
func (s *InMemoryCheckpointStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cases)
}
