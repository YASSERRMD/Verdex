package ingestion

import "sync"

// StatusStore is the read/write surface IngestionStatusAPI queries and the
// orchestrator updates as jobs progress. It is intentionally narrower than
// RecoveryStore (which persists resume checkpoints): StatusStore is the
// "current state of the world" view, while RecoveryStore is specifically
// about resuming failed jobs.
//
// Implementations must be safe for concurrent use.
type StatusStore interface {
	// Put records the current WorkflowState for a job.
	Put(state WorkflowState)

	// Get returns the current WorkflowState for jobID, and ok=false if the
	// job is unknown.
	Get(jobID string) (state WorkflowState, ok bool)

	// ListByCase returns the current WorkflowState of every known job
	// belonging to caseID, in no particular order.
	ListByCase(caseID string) []WorkflowState
}

// InMemoryStatusStore is a map-backed StatusStore.
type InMemoryStatusStore struct {
	mu    sync.Mutex
	byJob map[string]WorkflowState
}

// NewInMemoryStatusStore constructs an empty InMemoryStatusStore.
func NewInMemoryStatusStore() *InMemoryStatusStore {
	return &InMemoryStatusStore{byJob: make(map[string]WorkflowState)}
}

// Put implements StatusStore.
func (s *InMemoryStatusStore) Put(state WorkflowState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byJob[state.JobID] = state.Clone()
}

// Get implements StatusStore.
func (s *InMemoryStatusStore) Get(jobID string) (WorkflowState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.byJob[jobID]
	if !ok {
		return WorkflowState{}, false
	}
	return st.Clone(), true
}

// ListByCase implements StatusStore.
func (s *InMemoryStatusStore) ListByCase(caseID string) []WorkflowState {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []WorkflowState
	for _, st := range s.byJob {
		if st.CaseID == caseID {
			out = append(out, st.Clone())
		}
	}
	return out
}

// IngestionStatusAPI is a small in-process query surface over a job's (or a
// case's) ingestion status. It wraps a StatusStore rather than exposing it
// directly so it can be handed to an HTTP handler layer later without
// leaking the store's mutation methods (Put).
type IngestionStatusAPI struct {
	store StatusStore
}

// NewIngestionStatusAPI constructs an IngestionStatusAPI backed by store.
// If store is nil, a new InMemoryStatusStore is allocated (the resulting
// API will only ever report ErrJobNotFound, since nothing else can write
// to a store the caller cannot reach).
func NewIngestionStatusAPI(store StatusStore) *IngestionStatusAPI {
	if store == nil {
		store = NewInMemoryStatusStore()
	}
	return &IngestionStatusAPI{store: store}
}

// GetStatus returns the current WorkflowState for jobID. Returns
// ErrJobNotFound if the job is unknown.
func (a *IngestionStatusAPI) GetStatus(jobID string) (*WorkflowState, error) {
	st, ok := a.store.Get(jobID)
	if !ok {
		return nil, ErrJobNotFound
	}
	return &st, nil
}

// GetCaseStatus returns the current WorkflowState of every known job for
// caseID. Returns an empty (nil) slice, not an error, when the case has no
// known jobs.
func (a *IngestionStatusAPI) GetCaseStatus(caseID string) []WorkflowState {
	return a.store.ListByCase(caseID)
}
