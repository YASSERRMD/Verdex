package analytics

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when ctx carries no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("analytics: unauthenticated")

	// ErrForbidden is returned when the authenticated identity.User
	// lacks the permission required for the requested view.
	ErrForbidden = errors.New("analytics: forbidden")

	// ErrEmptyTenantID is returned when a call is made with a nil
	// tenant ID.
	ErrEmptyTenantID = errors.New("analytics: tenant id is required")

	// ErrInvalidFormat is returned by Export when asked to render an
	// unrecognized ExportFormat.
	ErrInvalidFormat = errors.New("analytics: invalid export format")

	// ErrNilMetrics is returned by Export when called with a nil
	// Metrics pointer.
	ErrNilMetrics = errors.New("analytics: metrics is required")

	// ErrComposerNotConfigured is returned by Dashboard.QualityTrend or
	// Dashboard.UsageView when the Dashboard was built without the
	// corresponding composer.
	ErrComposerNotConfigured = errors.New("analytics: composer not configured")
)
