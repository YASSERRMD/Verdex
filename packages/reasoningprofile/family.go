package reasoningprofile

import "github.com/YASSERRMD/verdex/packages/jurisdiction"

// Family classifies the legal tradition a reasoning-weight profile is
// keyed on. Values match packages/jurisdiction.LegalFamily's string
// values exactly, but Family is declared independently rather than as a
// type alias, mirroring packages/evidenceweighing.LegalFamily's and
// packages/lawapplication.LegalFamily's own decoupling convention: this
// package's public surface stays self-contained, and ResolveFamily is the
// one seam where a packages/jurisdiction value crosses into it.
type Family string

// Family constants. These are the exhaustive, canonical set of legal
// families this package resolves a Weights profile for — every switch
// over Family in this package covers exactly these four with no silent
// default.
const (
	// FamilyCommonLaw matches jurisdiction.LegalFamilyCommonLaw.
	FamilyCommonLaw Family = "common_law"

	// FamilyCivilLaw matches jurisdiction.LegalFamilyCivilLaw.
	FamilyCivilLaw Family = "civil_law"

	// FamilyMixed matches jurisdiction.LegalFamilyMixed.
	FamilyMixed Family = "mixed"

	// FamilyIslamicLaw matches jurisdiction.LegalFamilyIslamicLaw.
	FamilyIslamicLaw Family = "islamic_law"
)

// validFamilies is the exhaustive set of recognized Family values.
var validFamilies = map[Family]struct{}{
	FamilyCommonLaw:  {},
	FamilyCivilLaw:   {},
	FamilyMixed:      {},
	FamilyIslamicLaw: {},
}

// IsValid reports whether f is one of the four canonical Family
// constants.
func (f Family) IsValid() bool {
	_, ok := validFamilies[f]
	return ok
}

// ResolveFamily converts j's LegalFamily field into this package's
// Family, a thin wrapper with no additional logic: the two types share
// identical string values by construction (see the constants above), so
// resolution is a direct string conversion.
func ResolveFamily(j jurisdiction.Jurisdiction) Family {
	return Family(j.LegalFamily)
}
