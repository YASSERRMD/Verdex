package application

// Legal-family weighting table.
//
// Different legal traditions treat statute and precedent as authority
// differently:
//
//   - Under "common_law" (e.g. England & Wales, most US states), judicial
//     precedent (stare decisis) is itself a primary, binding source of
//     law, often filling gaps statute leaves open or interpreting it —
//     so a precedent-origin rule is weighted higher than a
//     statute-origin rule of otherwise equal match quality.
//   - Under "civil_law" (e.g. France, Germany, most of continental
//     Europe), codified statute is the primary source of law and
//     judicial decisions are persuasive but not formally binding
//     authority — so the weighting is reversed: statute-origin rules
//     outweigh precedent-origin rules.
//   - Any other/unrecognized dominantFamily value is treated as neutral:
//     both origins weight equally (1.0), since this package has no basis
//     to prefer one over the other without a recognized legal-family
//     signal.
//
// | dominantFamily | OriginStatute | OriginPrecedent |
// |-----------------|---------------|-----------------|
// | "common_law"    | 0.8           | 1.0             |
// | "civil_law"      | 1.0           | 0.8             |
// | anything else    | 1.0           | 1.0             |
const (
	commonLawFamily = "common_law"
	civilLawFamily  = "civil_law"

	dominantOriginWeight    = 1.0
	subordinateOriginWeight = 0.8
	neutralOriginWeight     = 1.0
)

// WeightByLegalFamily returns a multiplier in (0, 1] reflecting how
// strongly rule's Origin is favored under dominantFamily, per the
// weighting table documented above. The result is intended to be
// combined with a rule's raw match Score (see confidence.go) rather than
// used standalone.
func WeightByLegalFamily(rule OriginatedRule, dominantFamily string) float64 {
	switch dominantFamily {
	case commonLawFamily:
		if rule.Origin == OriginPrecedent {
			return dominantOriginWeight
		}
		return subordinateOriginWeight
	case civilLawFamily:
		if rule.Origin == OriginStatute {
			return dominantOriginWeight
		}
		return subordinateOriginWeight
	default:
		return neutralOriginWeight
	}
}
