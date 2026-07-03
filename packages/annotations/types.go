package annotations

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// AnchorType identifies what part of a case an Annotation is attached
// to. See doc.go, "Anchoring".
type AnchorType string

const (
	// AnchorCase means the annotation applies to the case as a whole,
	// with no narrower anchor. AnchorID is empty for this type.
	AnchorCase AnchorType = "case"

	// AnchorTreeNode means the annotation is anchored to a single node
	// within the case's packages/irac reasoning tree. AnchorID carries
	// that node's ID (irac.Node.ID's string ID space).
	AnchorTreeNode AnchorType = "tree_node"

	// AnchorEvidenceSegment means the annotation is anchored to a single
	// evidence segment. AnchorID carries that segment's ID, the same ID
	// space apps/web/src/components/workspace/EvidenceReviewPanel.tsx
	// (and the underlying packages/evidence/packages/segmentation
	// segment record) uses.
	AnchorEvidenceSegment AnchorType = "evidence_segment"
)

// allAnchorTypes is the exhaustive set of recognized AnchorType values,
// used by IsValid.
var allAnchorTypes = map[AnchorType]struct{}{
	AnchorCase:            {},
	AnchorTreeNode:        {},
	AnchorEvidenceSegment: {},
}

// IsValid reports whether t is one of the recognized AnchorType
// constants.
func (t AnchorType) IsValid() bool {
	_, ok := allAnchorTypes[t]
	return ok
}

// String satisfies fmt.Stringer.
func (t AnchorType) String() string { return string(t) }

// Annotation is a single note, highlight, or discussion comment
// attached to a case, optionally anchored to a tree node or evidence
// segment within it, and optionally a reply within a comment thread.
// See doc.go for the full model.
type Annotation struct {
	// ID uniquely identifies this annotation.
	ID uuid.UUID `json:"id"`

	// CaseID identifies the packages/caselifecycle.Case this annotation
	// belongs to. Required.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is the tenant this annotation belongs to. Every
	// Repository method is scoped to a tenantID and refuses
	// cross-tenant access (see ErrCrossTenantAccess), mirroring
	// packages/casesearch and packages/caselifecycle exactly.
	TenantID uuid.UUID `json:"tenant_id"`

	// AuthorID is the identity.User who wrote this annotation.
	// Required.
	AuthorID uuid.UUID `json:"author_id"`

	// Body is the annotation's free-text content. May contain
	// "@<userID>" mention tokens — see ExtractMentions. Required
	// (non-blank).
	Body string `json:"body"`

	// AnchorType selects what Body is attached to. Required, one of the
	// AnchorType constants.
	AnchorType AnchorType `json:"anchor_type"`

	// AnchorID is the ID within AnchorType's ID space: empty for
	// AnchorCase, an irac tree node ID for AnchorTreeNode, or an
	// evidence segment ID for AnchorEvidenceSegment. Required
	// (non-blank) for the latter two; must be empty for AnchorCase.
	AnchorID string `json:"anchor_id,omitempty"`

	// ParentID, if non-nil, identifies the Annotation this one replies
	// to, forming a thread. Nil for a thread root. See Thread.
	ParentID *uuid.UUID `json:"parent_id,omitempty"`

	// Resolved reports whether this annotation (thread root or reply)
	// has been marked resolved. See Resolve/Reopen.
	Resolved bool `json:"resolved"`

	// ResolvedBy is the identity.User who last called Resolve. Nil
	// while Resolved is false.
	ResolvedBy *uuid.UUID `json:"resolved_by,omitempty"`

	// ResolvedAt is when Resolve was last called. Nil while Resolved is
	// false.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`

	// CreatedAt is when this annotation was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this annotation's Body was last edited, or its
	// resolve state last changed.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks that a has every field required to be persisted: a
// non-nil CaseID, TenantID, and AuthorID, a non-blank Body, and a valid
// AnchorType/AnchorID combination (AnchorCase requires a blank
// AnchorID; AnchorTreeNode and AnchorEvidenceSegment require a
// non-blank AnchorID). Returns a sentinel error identifying which
// check failed.
func (a *Annotation) Validate() error {
	if a == nil {
		return ErrNilAnnotation
	}
	if a.CaseID == uuid.Nil {
		return ErrEmptyCaseID
	}
	if a.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if a.AuthorID == uuid.Nil {
		return ErrUnauthenticated
	}
	if strings.TrimSpace(a.Body) == "" {
		return ErrEmptyBody
	}
	if !a.AnchorType.IsValid() {
		return ErrInvalidAnchorType
	}
	switch a.AnchorType {
	case AnchorCase:
		if strings.TrimSpace(a.AnchorID) != "" {
			return ErrUnexpectedAnchorID
		}
	case AnchorTreeNode, AnchorEvidenceSegment:
		if strings.TrimSpace(a.AnchorID) == "" {
			return ErrEmptyAnchorID
		}
	}
	return nil
}

// IsReply reports whether a is a threaded reply (ParentID set) rather
// than a thread root.
func (a *Annotation) IsReply() bool {
	return a != nil && a.ParentID != nil
}

// RootID returns the ID of the thread this annotation belongs to: a's
// own ID if it is a root (ParentID nil), or *a.ParentID if it is a
// reply. Useful for grouping a flat list of Annotations into threads
// without a second repository call.
func (a *Annotation) RootID() uuid.UUID {
	if a == nil {
		return uuid.Nil
	}
	if a.ParentID != nil {
		return *a.ParentID
	}
	return a.ID
}
