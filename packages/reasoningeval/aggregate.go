package reasoningeval

// JurisdictionSummary aggregates QualityScores for a single jurisdiction.
type JurisdictionSummary struct {
	// JurisdictionCode identifies the jurisdiction this summary covers.
	JurisdictionCode string

	// LegalFamily is the reasoningprofile.Family this jurisdiction
	// resolves to, if every aggregated QualityScore agreed on one.
	// Left empty when scores disagree or none carried a LegalFamily.
	LegalFamily string

	// Count is the number of QualityScores aggregated into this summary.
	Count int

	// AvgOverall is the arithmetic mean of QualityScore.Overall across
	// this jurisdiction's scores.
	AvgOverall float64

	// AvgPerDimension is the arithmetic mean of each dimension's raw
	// score across this jurisdiction's scores.
	AvgPerDimension map[DimensionName]float64
}

// LegalFamilySummary aggregates QualityScores across every jurisdiction
// that resolves to the same reasoningprofile.Family, letting a caller see
// whether reasoning quality varies by legal tradition rather than just by
// individual jurisdiction.
type LegalFamilySummary struct {
	// LegalFamily identifies the legal family this summary covers.
	LegalFamily string

	// Count is the number of QualityScores aggregated into this summary.
	Count int

	// AvgOverall is the arithmetic mean of QualityScore.Overall across
	// this family's scores.
	AvgOverall float64

	// AvgPerDimension is the arithmetic mean of each dimension's raw
	// score across this family's scores.
	AvgPerDimension map[DimensionName]float64
}

// AggregateByJurisdiction groups scores by JurisdictionCode and computes
// a JurisdictionSummary per group. Scores with an empty JurisdictionCode
// are grouped under the empty string key ("unknown jurisdiction") rather
// than dropped, so callers can still see and investigate them.
func AggregateByJurisdiction(scores []QualityScore) map[string]JurisdictionSummary {
	byJurisdiction := make(map[string][]QualityScore)
	for _, s := range scores {
		byJurisdiction[s.JurisdictionCode] = append(byJurisdiction[s.JurisdictionCode], s)
	}

	summaries := make(map[string]JurisdictionSummary, len(byJurisdiction))
	for code, group := range byJurisdiction {
		summaries[code] = JurisdictionSummary{
			JurisdictionCode: code,
			LegalFamily:      commonLegalFamily(group),
			Count:            len(group),
			AvgOverall:       averageOverall(group),
			AvgPerDimension:  averagePerDimension(group),
		}
	}
	return summaries
}

// AggregateByLegalFamily groups scores by LegalFamily and computes a
// LegalFamilySummary per group. Scores with an empty LegalFamily are
// excluded, since "unknown family" is not a meaningful tradition to
// track trends for — callers wanting those should use
// AggregateByJurisdiction instead.
func AggregateByLegalFamily(scores []QualityScore) map[string]LegalFamilySummary {
	byFamily := make(map[string][]QualityScore)
	for _, s := range scores {
		if s.LegalFamily == "" {
			continue
		}
		byFamily[s.LegalFamily] = append(byFamily[s.LegalFamily], s)
	}

	summaries := make(map[string]LegalFamilySummary, len(byFamily))
	for family, group := range byFamily {
		summaries[family] = LegalFamilySummary{
			LegalFamily:     family,
			Count:           len(group),
			AvgOverall:      averageOverall(group),
			AvgPerDimension: averagePerDimension(group),
		}
	}
	return summaries
}

// commonLegalFamily returns the LegalFamily shared by every score in
// group, or "" if the group is empty or the scores disagree.
func commonLegalFamily(group []QualityScore) string {
	if len(group) == 0 {
		return ""
	}
	family := group[0].LegalFamily
	for _, s := range group[1:] {
		if s.LegalFamily != family {
			return ""
		}
	}
	return family
}
