package jurisdiction

import "fmt"

// LegalFamily classifies the primary legal tradition of a jurisdiction.
type LegalFamily string

const (
	// LegalFamilyCommonLaw represents jurisdictions that follow English common-law tradition.
	LegalFamilyCommonLaw LegalFamily = "common_law"

	// LegalFamilyCivilLaw represents jurisdictions based on codified civil-law systems
	// (Napoleonic, Roman-Germanic).
	LegalFamilyCivilLaw LegalFamily = "civil_law"

	// LegalFamilyMixed represents jurisdictions that blend two or more legal traditions
	// (e.g. common law + Islamic law, or civil law + customary law).
	LegalFamilyMixed LegalFamily = "mixed"

	// LegalFamilyIslamicLaw represents jurisdictions whose primary source of law is
	// Shari'a / Islamic jurisprudence.
	LegalFamilyIslamicLaw LegalFamily = "islamic_law"
)

// validLegalFamilies is the exhaustive set of recognised legal families.
var validLegalFamilies = map[LegalFamily]struct{}{
	LegalFamilyCommonLaw:  {},
	LegalFamilyCivilLaw:   {},
	LegalFamilyMixed:      {},
	LegalFamilyIslamicLaw: {},
}

// IsValid reports whether lf is one of the recognised LegalFamily constants.
func (lf LegalFamily) IsValid() bool {
	_, ok := validLegalFamilies[lf]
	return ok
}

// String returns a human-readable representation of the legal family.
func (lf LegalFamily) String() string {
	switch lf {
	case LegalFamilyCommonLaw:
		return "Common Law"
	case LegalFamilyCivilLaw:
		return "Civil Law"
	case LegalFamilyMixed:
		return "Mixed Legal System"
	case LegalFamilyIslamicLaw:
		return "Islamic Law (Shari'a)"
	default:
		return fmt.Sprintf("unknown(%s)", string(lf))
	}
}
