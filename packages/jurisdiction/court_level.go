package jurisdiction

import "fmt"

// CourtLevel represents the hierarchical position of a court within a
// national judicial system.
type CourtLevel string

const (
	// CourtLevelSupreme is the apex court of a legal system (e.g. Supreme Court,
	// Federal Court, House of Lords / UK Supreme Court).
	CourtLevelSupreme CourtLevel = "supreme"

	// CourtLevelAppellate is an intermediate appellate court that hears appeals
	// from lower courts but sits below the supreme court.
	CourtLevelAppellate CourtLevel = "appellate"

	// CourtLevelHigh is a superior court of first instance or record, often with
	// unlimited civil/criminal jurisdiction (e.g. High Court).
	CourtLevelHigh CourtLevel = "high"

	// CourtLevelDistrict represents district, sessions, or county-level courts.
	CourtLevelDistrict CourtLevel = "district"

	// CourtLevelMagistrate represents magistrates', summary, or minor courts.
	CourtLevelMagistrate CourtLevel = "magistrate"

	// CourtLevelSpecial represents tribunals, specialist courts (family, labour,
	// commercial, administrative, sharia personal-status), or other courts that
	// fall outside the main hierarchy.
	CourtLevelSpecial CourtLevel = "special"
)

// validCourtLevels is the exhaustive set of recognised court levels.
var validCourtLevels = map[CourtLevel]struct{}{
	CourtLevelSupreme:    {},
	CourtLevelAppellate:  {},
	CourtLevelHigh:       {},
	CourtLevelDistrict:   {},
	CourtLevelMagistrate: {},
	CourtLevelSpecial:    {},
}

// IsValid reports whether cl is one of the recognised CourtLevel constants.
func (cl CourtLevel) IsValid() bool {
	_, ok := validCourtLevels[cl]
	return ok
}

// String returns a human-readable label for the court level.
func (cl CourtLevel) String() string {
	switch cl {
	case CourtLevelSupreme:
		return "Supreme Court"
	case CourtLevelAppellate:
		return "Appellate Court"
	case CourtLevelHigh:
		return "High Court"
	case CourtLevelDistrict:
		return "District Court"
	case CourtLevelMagistrate:
		return "Magistrate Court"
	case CourtLevelSpecial:
		return "Special / Tribunal"
	default:
		return fmt.Sprintf("unknown(%s)", string(cl))
	}
}
