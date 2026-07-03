package caseversioning

import (
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// ArtifactKind identifies which part of a case a Snapshot captures the
// state of at one point in time. See doc.go, "What this package adds".
type ArtifactKind string

const (
	// ArtifactCaseMetadata means the Snapshot captures the mutable
	// fields of a packages/caselifecycle.Case (title, reference,
	// category, state, metadata) at one point in time. Payload carries
	// a CaseMetadataPayload, and this is the only ArtifactKind Diff
	// produces a field-level diff for and Restore can revert.
	ArtifactCaseMetadata ArtifactKind = "case-metadata"

	// ArtifactTree means the Snapshot references one immutable
	// packages/irac.TreeRevision (by RevisionNumber) for the case's
	// reasoning tree, produced by packages/treeassembly. This package
	// does not copy or re-store the tree itself — see doc.go,
	// "Composition, not reimplementation".
	ArtifactTree ArtifactKind = "tree"

	// ArtifactEvidence means the Snapshot marks a point-in-time
	// evidence-set change for the case (e.g. new evidence segment
	// ingested, annotation added/edited). ArtifactRevisionRef carries an
	// upstream identifier (an annotations.Annotation ID or evidence
	// segment ID) when one is available.
	ArtifactEvidence ArtifactKind = "evidence"

	// ArtifactOpinion means the Snapshot captures a
	// packages/synthesisagent.Opinion produced for the case. No package
	// upstream of this one versions Opinion output, so Payload carries a
	// compact copy (OpinionPayload) rather than a reference — see
	// doc.go, "Opinion history is new".
	ArtifactOpinion ArtifactKind = "opinion"
)

// allArtifactKinds is the exhaustive set of recognized ArtifactKind
// values, used by IsValid.
var allArtifactKinds = map[ArtifactKind]struct{}{
	ArtifactCaseMetadata: {},
	ArtifactTree:         {},
	ArtifactEvidence:     {},
	ArtifactOpinion:      {},
}

// IsValid reports whether k is one of the recognized ArtifactKind
// constants.
func (k ArtifactKind) IsValid() bool {
	_, ok := allArtifactKinds[k]
	return ok
}

// String satisfies fmt.Stringer.
func (k ArtifactKind) String() string { return string(k) }

// CaseMetadataPayload is the Snapshot.Payload shape for
// ArtifactCaseMetadata: a compact copy of the mutable fields of a
// packages/caselifecycle.Case, sufficient for Diff to compute a
// field-level comparison and for Restore to revert the live Case to this
// state.
type CaseMetadataPayload struct {
	Title      string            `json:"title"`
	Reference  string            `json:"reference,omitempty"`
	CategoryID string            `json:"category_id,omitempty"`
	State      string            `json:"state"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// OpinionPayload is the Snapshot.Payload shape for ArtifactOpinion: a
// compact copy of a packages/synthesisagent.Opinion at the time the
// snapshot was taken. See NewOpinionSnapshot.
type OpinionPayload struct {
	// CaseID mirrors synthesisagent.Opinion.CaseID.
	CaseID string `json:"case_id"`

	// ConclusionCount is len(Opinion.Conclusions) at snapshot time —
	// kept alongside the full Conclusions slice so a lightweight summary
	// view doesn't need to walk the payload.
	ConclusionCount int `json:"conclusion_count"`

	// Conclusions is a copy of Opinion.Conclusions at snapshot time.
	Conclusions []synthesisagent.TentativeConclusion `json:"conclusions"`

	// SkippedIssueNodeIDs mirrors Opinion.SkippedIssueNodeIDs.
	SkippedIssueNodeIDs []string `json:"skipped_issue_node_ids,omitempty"`

	// GeneratedAt mirrors Opinion.GeneratedAt.
	GeneratedAt time.Time `json:"generated_at"`
}

// Snapshot is one immutable, point-in-time record of a case artifact's
// state: the case-level aggregator entity this package adds. See doc.go
// for how Snapshot composes (rather than duplicates) the tree-revision
// and case-metadata-version mechanisms that already exist elsewhere.
type Snapshot struct {
	// ID uniquely identifies this snapshot.
	ID uuid.UUID `json:"id"`

	// CaseID identifies the packages/caselifecycle.Case this snapshot
	// belongs to. Required.
	CaseID uuid.UUID `json:"case_id"`

	// TenantID is the tenant this snapshot belongs to. Every Repository
	// method is scoped to a tenantID and refuses cross-tenant access
	// (see ErrCrossTenantAccess), mirroring packages/annotations and
	// packages/caselifecycle exactly.
	TenantID uuid.UUID `json:"tenant_id"`

	// ArtifactKind selects which case artifact this snapshot captures.
	// Required, one of the ArtifactKind constants.
	ArtifactKind ArtifactKind `json:"artifact_kind"`

	// ArtifactRevisionRef is the upstream revision identifier this
	// snapshot points to, when the artifact already has its own
	// versioning mechanism:
	//   - ArtifactTree: irac.TreeRevision.RevisionNumber, formatted as a
	//     decimal string (e.g. "3").
	//   - ArtifactEvidence: an annotations.Annotation ID or evidence
	//     segment ID, when derivable; empty otherwise.
	//   - ArtifactCaseMetadata: caselifecycle.Case.MetadataVersion,
	//     formatted as a decimal string, at the time of this snapshot.
	//   - ArtifactOpinion: empty — no upstream revision ID exists for
	//     Opinion (see OpinionPayload, which carries a compact copy
	//     instead).
	ArtifactRevisionRef string `json:"artifact_revision_ref,omitempty"`

	// Payload carries a compact, artifact-kind-specific copy of the
	// artifact's state at snapshot time, when one is needed for
	// diff/restore (CaseMetadataPayload for ArtifactCaseMetadata,
	// OpinionPayload for ArtifactOpinion). Nil for ArtifactTree and
	// ArtifactEvidence, which rely solely on ArtifactRevisionRef —
	// this package does not duplicate the tree or evidence store.
	Payload any `json:"payload,omitempty"`

	// CreatedBy is the identity.User who caused this snapshot to be
	// recorded — the actor performing the edit, restore, or triggering
	// event.
	CreatedBy uuid.UUID `json:"created_by"`

	// Reason is a free-form change-attribution note: what triggered this
	// snapshot (e.g. "manual edit", "signoff re-review",
	// "restore to version from 2026-06-01"). Optional but always
	// populated by Service helpers.
	Reason string `json:"reason,omitempty"`

	// Label is an optional short human-readable name for this snapshot
	// (e.g. "Pre-hearing draft"), distinct from Reason (why it was
	// taken) — what a reviewer would call it in a picker.
	Label string `json:"label,omitempty"`

	// RestoredFromID, if non-nil, identifies the Snapshot this one
	// restored the case-metadata artifact from. Set only on the
	// forward-only snapshot Restore records for the restore operation
	// itself — see Restore's doc comment.
	RestoredFromID *uuid.UUID `json:"restored_from_id,omitempty"`

	// CreatedAt is when this snapshot was recorded.
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks that s has every field required to be persisted: a
// non-nil CaseID and TenantID, a valid ArtifactKind, and a non-nil
// CreatedBy. Returns a sentinel error identifying which check failed.
func (s *Snapshot) Validate() error {
	if s == nil {
		return ErrNilSnapshot
	}
	if s.CaseID == uuid.Nil {
		return ErrEmptyCaseID
	}
	if s.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if !s.ArtifactKind.IsValid() {
		return ErrInvalidArtifactKind
	}
	if s.CreatedBy == uuid.Nil {
		return ErrEmptyCreatedBy
	}
	return nil
}

// IsRestore reports whether s was recorded by a Restore call rather than
// an ordinary snapshot of live state.
func (s *Snapshot) IsRestore() bool {
	return s != nil && s.RestoredFromID != nil
}
