package jurisdiction

import (
	"time"

	"github.com/google/uuid"
)

// seedTime is a fixed, deterministic timestamp used for all seeded records so
// that the seed output is reproducible across runs.
var seedTime = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

// SeedData returns a catalogue of real-world jurisdictions covering the key
// legal systems relevant to Verdex's target markets.  Every returned
// Jurisdiction passes Validate().
func SeedData() []Jurisdiction {
	return []Jurisdiction{
		// ------------------------------------------------------------------ UAE
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000001"),
			CountryCode: "AE",
			CountryName: "United Arab Emirates",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Federal Supreme Court of the UAE",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ar"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "UAE-CPC",
					Name:        "UAE Civil Procedure Code (Federal Law No. 11 of 1992)",
					Description: "Governs civil and commercial proceedings before federal courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000002"),
			CountryCode: "AE",
			CountryName: "United Arab Emirates",
			CourtLevel:  CourtLevelHigh,
			CourtName:   "Dubai Courts (High Civil Court)",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ar"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "UAE-CPC",
					Name:        "UAE Civil Procedure Code",
					Description: "Applied in Dubai local court system for civil and commercial matters.",
				},
				{
					Code:        "DUBAI-LAW-3-2003",
					Name:        "Dubai Law No. 3 of 2003 (Establishment of Dubai Courts)",
					Description: "Establishes the organisational structure of Dubai Courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000003"),
			CountryCode: "AE",
			CountryName: "United Arab Emirates",
			CourtLevel:  CourtLevelSpecial,
			CourtName:   "Abu Dhabi Global Market Courts (ADGM)",
			LegalFamily: LegalFamilyCommonLaw,
			Languages:   []string{"en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "ADGM-CPR",
					Name:        "ADGM Court Procedure Rules 2016",
					Description: "Common-law rules modelled on English CPR, governing ADGM Courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Pakistan
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000004"),
			CountryCode: "PK",
			CountryName: "Pakistan",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of Pakistan",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ur", "en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "CPC-1908",
					Name:        "Code of Civil Procedure 1908",
					Description: "Governs civil procedure in all subordinate courts and the Supreme Court.",
				},
				{
					Code:        "SCR-1980",
					Name:        "Supreme Court Rules 1980",
					Description: "Procedural rules specific to the Supreme Court of Pakistan.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000005"),
			CountryCode: "PK",
			CountryName: "Pakistan",
			CourtLevel:  CourtLevelHigh,
			CourtName:   "Lahore High Court",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ur", "en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "CPC-1908",
					Name:        "Code of Civil Procedure 1908",
					Description: "Primary procedural code for civil cases in the Lahore High Court.",
				},
				{
					Code:        "LHC-RULES",
					Name:        "Lahore High Court Rules and Orders",
					Description: "Supplementary procedural rules issued by the Lahore High Court.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ India
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000006"),
			CountryCode: "IN",
			CountryName: "India",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of India",
			LegalFamily: LegalFamilyCommonLaw,
			Languages:   []string{"en", "hi"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "CPC-1908-IN",
					Name:        "Code of Civil Procedure 1908 (India)",
					Description: "Governs civil procedure across all Indian courts.",
				},
				{
					Code:        "SCI-RULES-2013",
					Name:        "Supreme Court Rules 2013",
					Description: "Rules governing practice and procedure before the Supreme Court of India.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Sri Lanka
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000007"),
			CountryCode: "LK",
			CountryName: "Sri Lanka",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of Sri Lanka",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"si", "ta", "en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "CPC-LK",
					Name:        "Civil Procedure Code (Ordinance No. 2 of 1889)",
					Description: "Foundational civil procedure statute applicable in Sri Lankan courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ United Kingdom
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000008"),
			CountryCode: "GB",
			CountryName: "United Kingdom",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "UK Supreme Court",
			LegalFamily: LegalFamilyCommonLaw,
			Languages:   []string{"en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "UKSC-RULES-2009",
					Name:        "UK Supreme Court Rules 2009",
					Description: "Procedural rules governing appeals to the UK Supreme Court.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ United States
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-000000000009"),
			CountryCode: "US",
			CountryName: "United States of America",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of the United States",
			LegalFamily: LegalFamilyCommonLaw,
			Languages:   []string{"en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "SCOTUS-RULES",
					Name:        "Rules of the Supreme Court of the United States",
					Description: "Procedural rules governing practice before SCOTUS.",
				},
				{
					Code:        "FRCP",
					Name:        "Federal Rules of Civil Procedure",
					Description: "Governs civil procedure in all US federal district courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Egypt
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-00000000000a"),
			CountryCode: "EG",
			CountryName: "Egypt",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Constitutional Court of Egypt",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ar"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "EG-CPC",
					Name:        "Egyptian Civil and Commercial Procedure Code (Law No. 13 of 1968)",
					Description: "Governs civil and commercial litigation in Egyptian courts.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Saudi Arabia
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-00000000000b"),
			CountryCode: "SA",
			CountryName: "Saudi Arabia",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of Saudi Arabia",
			LegalFamily: LegalFamilyIslamicLaw,
			Languages:   []string{"ar"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "SA-CPC-2013",
					Name:        "Saudi Civil Procedure Law (Royal Decree M/1 of 2013)",
					Description: "Governs civil procedure in all Saudi courts; grounded in Islamic jurisprudence.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Malaysia
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-00000000000c"),
			CountryCode: "MY",
			CountryName: "Malaysia",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Federal Court of Malaysia",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"ms", "en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "MY-ROC-2012",
					Name:        "Rules of Court 2012 (Malaysia)",
					Description: "Primary procedural rules for the civil courts of Malaysia.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
		// ------------------------------------------------------------------ Nigeria
		{
			ID:          uuid.MustParse("00000000-0000-0000-0000-00000000000d"),
			CountryCode: "NG",
			CountryName: "Nigeria",
			CourtLevel:  CourtLevelSupreme,
			CourtName:   "Supreme Court of Nigeria",
			LegalFamily: LegalFamilyMixed,
			Languages:   []string{"en"},
			ProceduralRules: []ProceduralRule{
				{
					Code:        "SCN-RULES-1985",
					Name:        "Supreme Court Rules 1985 (as amended)",
					Description: "Rules governing appeals and practice before the Supreme Court of Nigeria.",
				},
			},
			CreatedAt: seedTime,
			UpdatedAt: seedTime,
		},
	}
}
