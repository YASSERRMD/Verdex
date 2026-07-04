package compliance

// SeedControls returns the platform's starter Control catalogue (tasks
// 2-4): a concrete set of UAE data-protection controls, a
// judicial-records-handling set, and international frameworks mapped
// in as an applicable/reference overlay. Every Description below is
// framed as a mapped requirement *category* this platform addresses,
// not a verbatim statutory quotation -- this package does not cite
// specific legal provisions it cannot verify (see doc/compliance.md).
// Callers seed a fresh ControlRepository with these via
// RegisterControl in a loop, or use SeedCatalogue directly.
//
// Every returned Control leaves ID, CreatedBy, CreatedAt, and UpdatedAt
// zero -- RegisterControl (engine.go) fills them in exactly as
// Engine.RegisterInventoryEntry does for
// packages/privacy.DataInventoryEntry.
func SeedControls() []Control {
	controls := make([]Control, 0, len(uaeDataProtectionSeeds)+len(judicialRecordsSeeds)+len(internationalOverlaySeeds))
	controls = append(controls, uaeDataProtectionSeeds...)
	controls = append(controls, judicialRecordsSeeds...)
	controls = append(controls, internationalOverlaySeeds...)
	return controls
}

// uaeDataProtectionSeeds is task 2's starter set: concrete,
// PDPL-style UAE data-protection controls mapped to the
// packages/privacy machinery that already satisfies them.
var uaeDataProtectionSeeds = []Control{
	{
		Code:        "UAE-DP-01",
		Title:       "Lawful basis recorded for every processing purpose",
		Description: "Every category of personal data processed by the platform records a lawful basis (consent, contract, legal obligation, legitimate interest, public task, or vital interest) before processing begins.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryLawfulBasis,
		MappedTo:    []string{"packages/privacy.DataInventoryEntry", "packages/privacy.LegalBasis"},
	},
	{
		Code:        "UAE-DP-02",
		Title:       "Data subject access rights honored",
		Description: "A data subject may request to know what personal data the tenant holds about them, tracked through a guarded workflow to fulfillment or rejection within a defined window.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryDataSubjectRights,
		MappedTo:    []string{"packages/privacy.SubjectAccessRequest", "packages/privacy.SARStatus"},
	},
	{
		Code:        "UAE-DP-03",
		Title:       "Right to erasure with provenance preservation",
		Description: "A data subject may request erasure of their personal data; execution scrubs the personal content while the chain-of-custody hash of any related record is structurally preserved, never silently dropped.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryDataSubjectRights,
		MappedTo:    []string{"packages/privacy.ErasureRequest", "packages/privacy.ExecuteErasure"},
	},
	{
		Code:        "UAE-DP-04",
		Title:       "Consent and withdrawal tracked over time",
		Description: "Consent grants and withdrawals are recorded per subject and purpose, and a subject's full consent history (not a single most-recent record) is evaluated to determine current validity.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryDataSubjectRights,
		MappedTo:    []string{"packages/privacy.ConsentRecord", "packages/privacy.HasValidConsent"},
	},
	{
		Code:        "UAE-DP-05",
		Title:       "Cross-border transfer restriction",
		Description: "Personal data is not transferred, stored, or processed outside an approved set of jurisdictions/regions without an explicit, logged residency decision.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryCrossBorderTransfer,
		MappedTo:    []string{"packages/dataresidency", "packages/airgapped"},
	},
	{
		Code:        "UAE-DP-06",
		Title:       "Breach detection and notification path",
		Description: "A suspected or confirmed personal-data breach is detectable from the durable audit trail and reportable through a defined notification path within a required window.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryBreachNotification,
		MappedTo:    []string{"packages/auditlog", "packages/accessgovernance"},
	},
	{
		Code:        "UAE-DP-07",
		Title:       "Data retention limits enforced",
		Description: "Each category of personal data carries a retention period; once elapsed, the platform's evaluation logic reports the prescribed action (hard delete or anonymize) rather than retaining data indefinitely by default.",
		Framework:   FrameworkUAEDataProtection,
		Category:    CategoryRecordRetention,
		MappedTo:    []string{"packages/privacy.RetentionPolicy", "packages/privacy.EnforceRetention"},
	},
}

// judicialRecordsSeeds is task 4's starter set: controls specific to
// handling judicial/court records.
var judicialRecordsSeeds = []Control{
	{
		Code:        "JRH-01",
		Title:       "Court record retention schedule enforced",
		Description: "Case records, filings, and hearing artifacts are retained according to a defined schedule before deletion or archival, distinct from and typically longer than ordinary personal-data retention windows.",
		Framework:   FrameworkJudicialRecordsHandling,
		Category:    CategoryRecordRetention,
		MappedTo:    []string{"packages/caselifecycle", "packages/caseversioning"},
	},
	{
		Code:        "JRH-02",
		Title:       "Chain of custody preserved for case content",
		Description: "Every substantive piece of case content carries a tamper-evident, hash-chained provenance record proving what it was and when it was captured, independent of any later erasure of personal data within it.",
		Framework:   FrameworkJudicialRecordsHandling,
		Category:    CategoryChainOfCustody,
		MappedTo:    []string{"packages/provenance"},
	},
	{
		Code:        "JRH-03",
		Title:       "Non-binding disclaimer enforced on reasoning output",
		Description: "Every AI-generated reasoning output is labeled as a non-binding draft analysis and rejected by the output pipeline if it contains verdict or directive language, so no generated content can be mistaken for an authoritative judicial determination.",
		Framework:   FrameworkJudicialRecordsHandling,
		Category:    CategoryNonBindingDisclaimer,
		MappedTo:    []string{"packages/guardrail.RequireDisclaimer", "packages/guardrail.CheckText"},
	},
	{
		Code:        "JRH-04",
		Title:       "Sign-off and audit trail for case disposition",
		Description: "A case's authoritative disposition (decision, order, or ruling) is recorded through an explicit sign-off workflow, and every disposition-affecting action is captured in the durable audit trail.",
		Framework:   FrameworkJudicialRecordsHandling,
		Category:    CategoryAuditability,
		MappedTo:    []string{"packages/signoff", "packages/auditlog"},
	},
	{
		Code:        "JRH-05",
		Title:       "Case access restricted by role and grant",
		Description: "Access to a case's content is gated by the actor's role and, where narrower or broader access is warranted, an explicit time-bound per-case grant -- never by tenant membership alone.",
		Framework:   FrameworkJudicialRecordsHandling,
		Category:    CategoryAccessControl,
		MappedTo:    []string{"packages/accessgovernance.CaseGrant", "packages/identity"},
	},
}

// internationalOverlaySeeds is task 3's starter set: international
// frameworks mapped in as an applicable/reference overlay layered on
// top of the same underlying controls that satisfy UAE data
// protection, rather than a wholly separate implementation. A
// deployment opts into this overlay via ComplianceProfile when its
// customer base or regulatory exposure calls for it.
var internationalOverlaySeeds = []Control{
	{
		Code:        "INTL-DP-01",
		Title:       "Lawful basis recorded (international reference)",
		Description: "Mirrors UAE-DP-01's lawful-basis requirement category as commonly found in international data-protection frameworks (e.g. GDPR-shaped regimes), mapped as a reference overlay -- not a verbatim citation of any single statute.",
		Framework:   FrameworkInternationalDataProtection,
		Category:    CategoryLawfulBasis,
		MappedTo:    []string{"packages/privacy.DataInventoryEntry", "packages/privacy.LegalBasis"},
	},
	{
		Code:        "INTL-DP-02",
		Title:       "Data subject rights (international reference)",
		Description: "Mirrors UAE-DP-02/UAE-DP-03/UAE-DP-04's data-subject-rights requirement categories (access, erasure, consent) as commonly found in international frameworks, mapped as a reference overlay.",
		Framework:   FrameworkInternationalDataProtection,
		Category:    CategoryDataSubjectRights,
		MappedTo:    []string{"packages/privacy.SubjectAccessRequest", "packages/privacy.ErasureRequest", "packages/privacy.ConsentRecord"},
	},
	{
		Code:        "INTL-DP-03",
		Title:       "Breach notification (international reference)",
		Description: "Mirrors UAE-DP-06's breach-notification requirement category as commonly found in international frameworks, mapped as a reference overlay.",
		Framework:   FrameworkInternationalDataProtection,
		Category:    CategoryBreachNotification,
		MappedTo:    []string{"packages/auditlog"},
	},
	{
		Code:        "INTL-DP-04",
		Title:       "Data minimization and anonymization (international reference)",
		Description: "Mirrors the data-minimization requirement category commonly found in international frameworks: analytics use of personal data is projected through anonymization/pseudonymization rather than operating on raw personal data directly.",
		Framework:   FrameworkInternationalDataProtection,
		Category:    CategoryRecordRetention,
		MappedTo:    []string{"packages/privacy.AnonymizeForAnalytics", "packages/pii"},
	},
}
