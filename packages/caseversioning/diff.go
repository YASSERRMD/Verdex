package caseversioning

import (
	"reflect"
	"sort"
)

// FieldChange is one field-level (or map-key-level) change between two
// CaseMetadataPayload snapshots.
type FieldChange struct {
	// Field names the changed field, e.g. "title", "state", or
	// "metadata[docket_number]" for a change nested inside Metadata.
	Field string `json:"field"`

	// Before is the field's value in snapshotA, as a string. Empty if
	// the field/key was absent in snapshotA.
	Before string `json:"before"`

	// After is the field's value in snapshotB, as a string. Empty if the
	// field/key was absent in snapshotB (i.e. removed).
	After string `json:"after"`
}

// Diff is the structured comparison Diff(ctx, snapshotA, snapshotB)
// produces between two Snapshots of the same case and ArtifactKind. See
// Diff's doc comment for how each ArtifactKind is compared.
type Diff struct {
	// CaseID is the case both snapshots belong to.
	CaseID string `json:"case_id"`

	// ArtifactKind is the shared ArtifactKind of snapshotA and
	// snapshotB.
	ArtifactKind ArtifactKind `json:"artifact_kind"`

	// SnapshotAID / SnapshotBID identify the two snapshots compared, in
	// the order passed to Diff.
	SnapshotAID string `json:"snapshot_a_id"`
	SnapshotBID string `json:"snapshot_b_id"`

	// FieldChanges holds a real field-by-field diff. Populated for
	// ArtifactCaseMetadata; empty for the reference-level kinds.
	FieldChanges []FieldChange `json:"field_changes,omitempty"`

	// RevisionRefChanged reports whether ArtifactRevisionRef differs
	// between snapshotA and snapshotB — the reference-level diff for
	// ArtifactTree, ArtifactEvidence, and ArtifactOpinion, per task 2:
	// "a reference-level diff for tree/opinion (which revision IDs
	// changed)".
	RevisionRefChanged bool `json:"revision_ref_changed"`

	// RevisionRefBefore / RevisionRefAfter carry the two
	// ArtifactRevisionRef values being compared for the reference-level
	// diff.
	RevisionRefBefore string `json:"revision_ref_before,omitempty"`
	RevisionRefAfter  string `json:"revision_ref_after,omitempty"`

	// Identical reports whether the two snapshots represent no change at
	// all (no field changes and no revision-ref change).
	Identical bool `json:"identical"`
}

// diffCaseMetadata produces the field-level FieldChanges between two
// CaseMetadataPayload values, comparing Title, Reference, CategoryID,
// State, and every key present in either Metadata map.
func diffCaseMetadata(a, b CaseMetadataPayload) []FieldChange {
	var changes []FieldChange

	if a.Title != b.Title {
		changes = append(changes, FieldChange{Field: "title", Before: a.Title, After: b.Title})
	}
	if a.Reference != b.Reference {
		changes = append(changes, FieldChange{Field: "reference", Before: a.Reference, After: b.Reference})
	}
	if a.CategoryID != b.CategoryID {
		changes = append(changes, FieldChange{Field: "category_id", Before: a.CategoryID, After: b.CategoryID})
	}
	if a.State != b.State {
		changes = append(changes, FieldChange{Field: "state", Before: a.State, After: b.State})
	}

	keys := make(map[string]struct{})
	for k := range a.Metadata {
		keys[k] = struct{}{}
	}
	for k := range b.Metadata {
		keys[k] = struct{}{}
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		av, aok := a.Metadata[k]
		bv, bok := b.Metadata[k]
		if aok && bok && av == bv {
			continue
		}
		if !aok && !bok {
			continue
		}
		changes = append(changes, FieldChange{
			Field:  "metadata[" + k + "]",
			Before: av,
			After:  bv,
		})
	}

	return changes
}

// ComputeDiff produces a structured comparison between snapshotA and
// snapshotB, which must belong to the same CaseID (ErrMismatchedCase
// otherwise) and share the same ArtifactKind (ErrMismatchedArtifactKind
// otherwise). This is the package-level diff primitive; Service.Diff
// (service.go) is the access-controlled, context-aware entrypoint
// callers normally use — matching this package's task-list contract of
// "Diff(ctx, snapshotA, snapshotB) (Diff, error)".
//
// For ArtifactCaseMetadata, ComputeDiff decodes both snapshots' Payload
// as CaseMetadataPayload and returns a real field-by-field diff in
// FieldChanges (title, reference, category_id, state, and every
// metadata key present in either snapshot).
//
// For ArtifactTree, ArtifactEvidence, and ArtifactOpinion — artifacts
// that already have (or, for opinion, do not need) their own full
// content store — ComputeDiff returns a reference-level diff: whether
// ArtifactRevisionRef changed between the two snapshots. This
// deliberately does not attempt to diff tree structure or opinion text
// itself; callers that need that should resolve ArtifactRevisionRef
// (ArtifactTree) or decode Payload as OpinionPayload (ArtifactOpinion)
// via the upstream package directly.
func ComputeDiff(snapshotA, snapshotB *Snapshot) (Diff, error) {
	if snapshotA == nil || snapshotB == nil {
		return Diff{}, ErrNilSnapshot
	}
	if snapshotA.CaseID != snapshotB.CaseID {
		return Diff{}, ErrMismatchedCase
	}
	if snapshotA.ArtifactKind != snapshotB.ArtifactKind {
		return Diff{}, ErrMismatchedArtifactKind
	}

	out := Diff{
		CaseID:            snapshotA.CaseID.String(),
		ArtifactKind:      snapshotA.ArtifactKind,
		SnapshotAID:       snapshotA.ID.String(),
		SnapshotBID:       snapshotB.ID.String(),
		RevisionRefBefore: snapshotA.ArtifactRevisionRef,
		RevisionRefAfter:  snapshotB.ArtifactRevisionRef,
	}
	out.RevisionRefChanged = snapshotA.ArtifactRevisionRef != snapshotB.ArtifactRevisionRef

	if snapshotA.ArtifactKind == ArtifactCaseMetadata {
		pa, err := AsCaseMetadataPayload(snapshotA)
		if err != nil {
			return Diff{}, err
		}
		pb, err := AsCaseMetadataPayload(snapshotB)
		if err != nil {
			return Diff{}, err
		}
		out.FieldChanges = diffCaseMetadata(pa, pb)
	}

	switch snapshotA.ArtifactKind {
	case ArtifactCaseMetadata:
		// Field-level diff already accounts for every meaningful
		// difference in a CaseMetadataPayload.
		out.Identical = len(out.FieldChanges) == 0
	default:
		// Reference-level artifacts: identical iff both the revision
		// ref and any compact payload (e.g. OpinionPayload) match.
		out.Identical = !out.RevisionRefChanged && reflect.DeepEqual(snapshotA.Payload, snapshotB.Payload)
	}

	return out, nil
}

// AsCaseMetadataPayload decodes s.Payload as a CaseMetadataPayload.
// Returns ErrNoPayload if s.Payload is nil, and ErrInvalidArtifactKind
// (wrapped) if s.Payload is not a CaseMetadataPayload.
func AsCaseMetadataPayload(s *Snapshot) (CaseMetadataPayload, error) {
	if s == nil || s.Payload == nil {
		return CaseMetadataPayload{}, ErrNoPayload
	}
	p, ok := s.Payload.(CaseMetadataPayload)
	if !ok {
		return CaseMetadataPayload{}, wrapf("AsCaseMetadataPayload", ErrInvalidArtifactKind)
	}
	return p, nil
}

// AsOpinionPayload decodes s.Payload as an OpinionPayload. Returns
// ErrNoPayload if s.Payload is nil, and ErrInvalidArtifactKind (wrapped)
// if s.Payload is not an OpinionPayload.
func AsOpinionPayload(s *Snapshot) (OpinionPayload, error) {
	if s == nil || s.Payload == nil {
		return OpinionPayload{}, ErrNoPayload
	}
	p, ok := s.Payload.(OpinionPayload)
	if !ok {
		return OpinionPayload{}, wrapf("AsOpinionPayload", ErrInvalidArtifactKind)
	}
	return p, nil
}
