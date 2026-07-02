package reasoningprofile

// WeightsForFamily resolves the canonical Weights profile for family,
// exhaustively across the four recognized Family constants. Unlike
// packages/evidenceweighing.ProfileForFamily and
// packages/lawapplication.ProfileForFamily (which silently default any
// unrecognized family to a neutral profile), this function has no
// fallback case: an unrecognized family returns the zero Weights value
// alongside ErrUnknownFamily, so a caller cannot silently proceed with a
// meaningless profile.
func WeightsForFamily(family Family) (Weights, error) {
	switch family {
	case FamilyCommonLaw:
		return CommonLawWeights(), nil
	case FamilyCivilLaw:
		return CivilLawWeights(), nil
	case FamilyMixed:
		return MixedWeights(), nil
	case FamilyIslamicLaw:
		return IslamicLawWeights(), nil
	default:
		return Weights{}, ErrUnknownFamily
	}
}
