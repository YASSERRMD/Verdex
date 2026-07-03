package auditlog

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/observability"
)

// Kind classifies the sensitive-action category an Event belongs to,
// giving Query a stable, closed taxonomy to filter on (task 5)
// independent of the free-form Action verb-phrase inherited from
// observability.AuditEvent.
type Kind string

// Kind values. This taxonomy is deliberately small and closed: new
// kinds should be added here (and documented in doc/audit-trail.md)
// rather than encoded only in Action strings, so Query's KindIn filter
// stays meaningful.
const (
	// KindDataAccess covers read access to case materials, documents,
	// or other sensitive records (task 2).
	KindDataAccess Kind = "data_access"

	// KindReasoning covers AI reasoning/synthesis pipeline events that
	// touch a case's substantive output.
	KindReasoning Kind = "reasoning"

	// KindSignoff covers human sign-off decisions: approvals,
	// rejections, and re-review triggers (task 3).
	KindSignoff Kind = "signoff"

	// KindDataChange covers writes: case edits, filings, metadata
	// changes.
	KindDataChange Kind = "data_change"

	// KindAdmin covers administrative actions: user management,
	// tenant configuration, key lifecycle operations.
	KindAdmin Kind = "admin"

	// KindExport covers regulator/compliance exports and report
	// generation (task 7 uses this to audit its own exports).
	KindExport Kind = "export"

	// KindSystem covers system-triggered events with no human actor,
	// e.g. automatic re-review reversions or retention purges.
	KindSystem Kind = "system"
)

// IsValid reports whether k is one of the named Kind constants.
func (k Kind) IsValid() bool {
	switch k {
	case KindDataAccess, KindReasoning, KindSignoff, KindDataChange, KindAdmin, KindExport, KindSystem:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k Kind) String() string { return string(k) }

// Event is the canonical, immutable audit record this package
// persists. It embeds observability.AuditEvent rather than inventing a
// competing schema: observability.AuditEvent already establishes the
// minimal who/what/when/outcome contract (Phase 003), and its own doc
// comment explicitly defers "richer event taxonomy, retention/storage
// guarantees, tamper evidence, query interfaces" to this phase. Event
// adds exactly those fields — tenant scoping, a closed Kind taxonomy,
// a stable CaseID for filtering, a free-form Detail payload, and the
// hash-chain fields — without redefining Time/Actor/Action/Target/
// Outcome.
type Event struct {
	// ID uniquely identifies this audit event.
	ID uuid.UUID `json:"id"`

	// TenantID scopes the event to a tenant. Required for every event
	// except cross-tenant system events, which this package does not
	// currently emit.
	TenantID uuid.UUID `json:"tenant_id"`

	// AuditEvent carries the Phase 003 fields this schema extends:
	// Time, Actor, Action, Target, Outcome.
	observability.AuditEvent

	// Kind classifies the event into the closed taxonomy above.
	Kind Kind `json:"kind"`

	// CaseID optionally scopes the event to a single case, letting
	// Query filter "everything that happened on case X" independently
	// of Target (which may hold a document ID, a key ID, etc. rather
	// than a case ID).
	CaseID uuid.UUID `json:"case_id,omitempty"`

	// Detail carries a short, free-form, human-readable elaboration on
	// the event (e.g. a sign-off's reviewer notes, a break-glass
	// justification). Never used to store sensitive document content
	// itself — only metadata about the action.
	Detail string `json:"detail,omitempty"`

	// PrevHash is the ChainHash of the immediately preceding event in
	// this event's tenant-scoped chain, or the empty string for the
	// first event. Mirrors packages/provenance's ChainBuilder
	// convention: PrevHash + ID + a content hash of this event's fields
	// are combined to derive ChainHash.
	PrevHash string `json:"prev_hash"`

	// ChainHash is SHA-256(PrevHash + ID + content fields) — see
	// chain.go. Any modification to a stored event's fields, or to any
	// earlier event's ChainHash, is detectable because it no longer
	// matches the recomputed value.
	ChainHash string `json:"chain_hash"`
}

// Validate reports the first structural problem with e, or nil if e is
// well-formed enough to append. Validate does not check the hash chain
// itself (see VerifyChain in chain.go) — only the fields a caller
// supplies before ChainHash is computed.
func (e *Event) Validate() error {
	if e == nil {
		return ErrNilEvent
	}
	if e.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if strings.TrimSpace(e.Action) == "" {
		return ErrEmptyAction
	}
	if strings.TrimSpace(e.Actor) == "" {
		return ErrEmptyActor
	}
	if !e.Kind.IsValid() {
		return fmt.Errorf("auditlog: invalid kind %q", string(e.Kind))
	}
	return nil
}

// Filter narrows Query to a subset of a tenant's audit events. All
// non-zero fields are ANDed together. TenantID is set by Query itself
// from the authenticated actor's context, never taken from Filter
// directly, so a caller cannot widen a query beyond their own tenant.
type Filter struct {
	// Actor, if non-empty, restricts results to events with this exact
	// Actor string.
	Actor string

	// CaseID, if non-nil, restricts results to this case.
	CaseID uuid.UUID

	// Kinds, if non-empty, restricts results to events whose Kind is in
	// this list.
	Kinds []Kind

	// Action, if non-empty, restricts results to events with this exact
	// Action string.
	Action string

	// Since, if non-zero, restricts results to events with Time >= Since.
	Since time.Time

	// Until, if non-zero, restricts results to events with Time <= Until.
	Until time.Time

	// Limit caps the number of results returned. Zero means "use the
	// store's default limit" (see store.go).
	Limit int
}

// RetentionPolicy configures how long Event rows are retained before
// they become eligible for Purge (task 6).
type RetentionPolicy struct {
	// Window is how long an event is retained after its Time, e.g.
	// 7*365*24*time.Hour for a seven-year regulatory retention window.
	// Must be positive.
	Window time.Duration
}

// Validate reports whether p is usable.
func (p RetentionPolicy) Validate() error {
	if p.Window <= 0 {
		return ErrInvalidRetention
	}
	return nil
}

// CutoffBefore returns the time before which events are eligible for
// purge, given now as the current time.
func (p RetentionPolicy) CutoffBefore(now time.Time) time.Time {
	return now.Add(-p.Window)
}

// ExportFormat selects the rendering Export produces (task 7).
type ExportFormat string

const (
	// ExportFormatCSV renders matching events as CSV, one row per
	// event, header row first.
	ExportFormatCSV ExportFormat = "csv"

	// ExportFormatJSON renders matching events as a JSON array.
	ExportFormatJSON ExportFormat = "json"
)

// IsValid reports whether f is a recognized ExportFormat.
func (f ExportFormat) IsValid() bool {
	return f == ExportFormatCSV || f == ExportFormatJSON
}
