package ingestion

import "sync"

// RecoveryStore persists the last-completed WorkflowState for a job so that
// a mid-pipeline failure can be resumed from the last-completed stage
// rather than restarting the whole pipeline from StageIntake.
//
// Implementations must be safe for concurrent use.
type RecoveryStore interface {
	// Checkpoint persists state as the job's latest recovery point.
	Checkpoint(state WorkflowState)

	// Load returns the last checkpointed WorkflowState for jobID, and
	// ok=false if no checkpoint has been recorded.
	Load(jobID string) (state WorkflowState, ok bool)

	// Delete removes any checkpoint for jobID (called once a job reaches a
	// terminal stage and no further resume is meaningful).
	Delete(jobID string)
}

// InMemoryRecoveryStore is a map-backed RecoveryStore.
type InMemoryRecoveryStore struct {
	mu          sync.Mutex
	checkpoints map[string]WorkflowState
}

// NewInMemoryRecoveryStore constructs an empty InMemoryRecoveryStore.
func NewInMemoryRecoveryStore() *InMemoryRecoveryStore {
	return &InMemoryRecoveryStore{checkpoints: make(map[string]WorkflowState)}
}

// Checkpoint implements RecoveryStore.
func (s *InMemoryRecoveryStore) Checkpoint(state WorkflowState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkpoints[state.JobID] = state.Clone()
}

// Load implements RecoveryStore.
func (s *InMemoryRecoveryStore) Load(jobID string) (WorkflowState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.checkpoints[jobID]
	if !ok {
		return WorkflowState{}, false
	}
	return st.Clone(), true
}

// Delete implements RecoveryStore.
func (s *InMemoryRecoveryStore) Delete(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.checkpoints, jobID)
}

// ResumePlan describes where a resumed job should restart from.
type ResumePlan struct {
	// JobID identifies the job being resumed.
	JobID string

	// FromStage is the stage execution should resume at. This is the
	// last-checkpointed stage itself (not the stage after it): the
	// orchestrator re-runs FromStage via RunWithRetry, which is a no-op if
	// that stage's idempotency record is already Completed, and otherwise
	// re-attempts it.
	FromStage Stage
}

// PlanResume looks up jobID's last checkpoint in recovery and returns a
// ResumePlan describing where to continue. Returns ErrNotResumable if no
// checkpoint exists, or if the checkpointed state is already terminal
// (StageComplete or StageFailed) with no failure to recover from.
func PlanResume(recovery RecoveryStore, jobID string) (ResumePlan, error) {
	if recovery == nil {
		return ResumePlan{}, ErrNotResumable
	}
	state, ok := recovery.Load(jobID)
	if !ok {
		return ResumePlan{}, ErrNotResumable
	}
	if state.Stage == StageComplete {
		return ResumePlan{}, ErrNotResumable
	}
	// A checkpoint recorded at StageFailed still carries the stage that was
	// being attempted when retries were exhausted (see orchestrator.go),
	// which is exactly what Resume should re-attempt.
	return ResumePlan{JobID: jobID, FromStage: state.Stage}, nil
}
