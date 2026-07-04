package integration

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("integration: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission an integration operation requires.
	ErrForbidden = errors.New("integration: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/compliance.ErrCrossTenantAccess and
	// packages/privacy.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("integration: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("integration: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("integration: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("integration: audit sink must not be nil")

	// ErrNilConnector is returned when an operation is invoked with a
	// nil Connector.
	ErrNilConnector = errors.New("integration: connector must not be nil")

	// ErrInvalidConnectorConfig is returned when a ConnectorConfig fails
	// structural validation.
	ErrInvalidConnectorConfig = errors.New("integration: invalid connector configuration")

	// ErrConnectorNotFound is returned when a referenced connector ID
	// does not resolve to any registered Connector or stored
	// ConnectorConfig.
	ErrConnectorNotFound = errors.New("integration: connector not found")

	// ErrDuplicateConnector is returned when Registry.Register is called
	// with an ID already present in the registry.
	ErrDuplicateConnector = errors.New("integration: connector already registered")

	// ErrInvalidCredentials is returned when ConnectorCredentials fails
	// structural validation (e.g. references raw secret material
	// instead of a handle, or names no secret reference at all).
	ErrInvalidCredentials = errors.New("integration: invalid connector credentials")

	// ErrCredentialsNotFound is returned when a referenced
	// ConnectorCredentials ID does not resolve to any stored record for
	// the tenant.
	ErrCredentialsNotFound = errors.New("integration: connector credentials not found")

	// ErrCredentialsNotVerified is returned when a connector call is
	// attempted before its credentials have passed Validate/authorize.
	ErrCredentialsNotVerified = errors.New("integration: connector credentials not verified")

	// ErrInvalidFieldMapping is returned when a FieldMapping fails
	// structural validation.
	ErrInvalidFieldMapping = errors.New("integration: invalid field mapping")

	// ErrMappingNotFound is returned when a referenced FieldMapping ID
	// does not resolve to any stored mapping for the tenant.
	ErrMappingNotFound = errors.New("integration: field mapping not found")

	// ErrUnmappedField is returned by FieldMapping.Apply/Reverse when a
	// required target field has no source mapping.
	ErrUnmappedField = errors.New("integration: required field has no mapping")

	// ErrInvalidImportRun is returned when an ImportRun fails structural
	// validation.
	ErrInvalidImportRun = errors.New("integration: invalid import run")

	// ErrImportRunNotFound is returned when a referenced ImportRun ID
	// does not resolve to any stored record for the tenant.
	ErrImportRunNotFound = errors.New("integration: import run not found")

	// ErrInvalidDeliveryRun is returned when a DeliveryRun fails
	// structural validation.
	ErrInvalidDeliveryRun = errors.New("integration: invalid delivery run")

	// ErrDeliveryRunNotFound is returned when a referenced DeliveryRun ID
	// does not resolve to any stored record for the tenant.
	ErrDeliveryRunNotFound = errors.New("integration: delivery run not found")

	// ErrInvalidReconciliation is returned when a ReconciliationResult
	// fails structural validation.
	ErrInvalidReconciliation = errors.New("integration: invalid reconciliation result")

	// ErrReconciliationNotFound is returned when a referenced
	// ReconciliationResult ID does not resolve to any stored record for
	// the tenant.
	ErrReconciliationNotFound = errors.New("integration: reconciliation result not found")

	// ErrRetriesExhausted is returned by WithRetry when every attempt
	// has failed.
	ErrRetriesExhausted = errors.New("integration: retries exhausted")

	// ErrConnectorUnavailable is returned by Ping (and surfaced by
	// SandboxConnector) when the simulated/real upstream endpoint is
	// unreachable.
	ErrConnectorUnavailable = errors.New("integration: connector unavailable")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("integration: %s: %w", fn, err)
}

// isNotFound reports whether err is target via errors.Is. Small helper
// mirroring packages/compliance.isNotFound.
func isNotFound(err, target error) bool {
	return errors.Is(err, target)
}
