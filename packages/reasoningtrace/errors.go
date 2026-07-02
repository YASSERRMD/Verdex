package reasoningtrace

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when Build is called with an empty case
	// ID.
	ErrEmptyCaseID = errors.New("reasoningtrace: case id is required")

	// ErrNilCheckpointStore is returned when Build is called with a nil
	// reasoningorchestration.CheckpointStore.
	ErrNilCheckpointStore = errors.New("reasoningtrace: checkpoint store is required")

	// ErrUnauthenticated is returned by RequireViewPermission when ctx
	// carries no authenticated identity.User.
	ErrUnauthenticated = errors.New("reasoningtrace: unauthenticated request")

	// ErrForbidden is returned by RequireViewPermission when the
	// authenticated actor lacks identity.PermViewCase.
	ErrForbidden = errors.New("reasoningtrace: actor lacks required permission")

	// ErrIncompleteRun is returned by Build when caseID has no
	// checkpointed RunState at all — there is nothing to trace.
	ErrIncompleteRun = errors.New("reasoningtrace: no run found for case")
)
