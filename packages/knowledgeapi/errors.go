package knowledgeapi

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when a call is made without an
	// authenticated identity.User present on the request context.
	ErrUnauthenticated = errors.New("knowledgeapi: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// permission required to perform the requested operation.
	ErrForbidden = errors.New("knowledgeapi: actor lacks required permission")

	// ErrEmptyCaseID is returned when a request is missing a required
	// case ID.
	ErrEmptyCaseID = errors.New("knowledgeapi: case id is required")

	// ErrEmptyNodeID is returned when a request is missing a required
	// node ID.
	ErrEmptyNodeID = errors.New("knowledgeapi: node id is required")

	// ErrNilService is returned by a constructor when a required
	// collaborator dependency is nil.
	ErrNilService = errors.New("knowledgeapi: required dependency must not be nil")

	// ErrInvalidPagination is returned when a request's pagination
	// parameters are out of range.
	ErrInvalidPagination = errors.New("knowledgeapi: invalid pagination parameters")

	// ErrEmptyQuery is returned when a hybrid retrieval request carries
	// neither a query vector nor an anchor node ID.
	ErrEmptyQuery = errors.New("knowledgeapi: hybrid query requires a vector or an anchor node id")
)
