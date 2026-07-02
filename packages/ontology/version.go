package ontology

import "time"

// OntologyVersion identifies one immutable snapshot of the ontology
// (concepts, relations, overlays, aliases, and labels as they stood at
// that point). An ontology is never mutated in place: each change
// produces a new OntologyVersion that supersedes, but does not overwrite,
// its parent. This mirrors packages/irac's TreeRevision immutable-
// revision shape.
type OntologyVersion struct {
	// VersionNumber is this version's sequence number, starting at 1 and
	// incrementing by 1 for each subsequent version.
	VersionNumber int `json:"version_number"`

	// CreatedAt is the timestamp this version was created.
	CreatedAt time.Time `json:"created_at"`

	// ParentVersion is the VersionNumber of the version this one
	// supersedes, or nil if this is the first version.
	ParentVersion *int `json:"parent_version,omitempty"`
}

// IsInitial reports whether v is the first version in the sequence (i.e.
// it has no ParentVersion).
func (v OntologyVersion) IsInitial() bool {
	return v.ParentVersion == nil
}

// NewInitialVersion constructs the first OntologyVersion: VersionNumber
// 1, no ParentVersion.
func NewInitialVersion(createdAt time.Time) OntologyVersion {
	return OntologyVersion{
		VersionNumber: 1,
		CreatedAt:     createdAt,
	}
}

// NextVersion constructs the OntologyVersion that immediately follows
// prev in the sequence: VersionNumber prev.VersionNumber + 1,
// ParentVersion pointing back at prev.VersionNumber.
func NextVersion(prev OntologyVersion, createdAt time.Time) OntologyVersion {
	parent := prev.VersionNumber
	return OntologyVersion{
		VersionNumber: prev.VersionNumber + 1,
		CreatedAt:     createdAt,
		ParentVersion: &parent,
	}
}

// IsValidSuccessorOf reports whether v is a well-formed direct successor
// of prev: VersionNumber exactly one greater, and ParentVersion pointing
// at prev.VersionNumber.
func (v OntologyVersion) IsValidSuccessorOf(prev OntologyVersion) bool {
	if v.VersionNumber != prev.VersionNumber+1 {
		return false
	}
	if v.ParentVersion == nil {
		return false
	}
	return *v.ParentVersion == prev.VersionNumber
}
