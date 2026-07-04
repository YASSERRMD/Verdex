package vulnmanagement

import "strings"

// LicenseClassification is the outcome of evaluating a dependency's
// license against this package's allow/deny lists (task 7). A closed
// enum: there are exactly three outcomes a license-compliance check
// can reach.
type LicenseClassification string

const (
	// LicenseAllowed means the license is on the allow list --
	// permissively usable with no further review (e.g. MIT,
	// Apache-2.0, BSD).
	LicenseAllowed LicenseClassification = "allowed"

	// LicenseDenied means the license is on the deny list -- its
	// copyleft/redistribution terms are incompatible with how this
	// platform ships (e.g. GPL, AGPL) and the dependency must not be
	// used without an exception.
	LicenseDenied LicenseClassification = "denied"

	// LicenseNeedsReview means the license is recognized but requires
	// a human legal/compliance review before use (e.g. a
	// source-available or field-of-use-restricted license), or is not
	// recognized at all -- an unknown license is deliberately treated
	// as "needs review", never silently allowed.
	LicenseNeedsReview LicenseClassification = "needs_review"
)

// String satisfies fmt.Stringer.
func (c LicenseClassification) String() string { return string(c) }

// allowedLicenses is a starter allow list of common permissive
// open-source licenses (by SPDX identifier), seeded rather than backed
// by a real SPDX database, per this phase's design guidance.
var allowedLicenses = map[string]struct{}{
	"MIT":          {},
	"Apache-2.0":   {},
	"BSD-2-Clause": {},
	"BSD-3-Clause": {},
	"ISC":          {},
	"MPL-2.0":      {},
	"Unlicense":    {},
	"0BSD":         {},
}

// deniedLicenses is a starter deny list of copyleft licenses whose
// redistribution terms are, by default, incompatible with how this
// platform ships closed-source deployments to customers.
var deniedLicenses = map[string]struct{}{
	"GPL-2.0":      {},
	"GPL-3.0":      {},
	"AGPL-3.0":     {},
	"LGPL-2.1":     {},
	"LGPL-3.0":     {},
	"SSPL-1.0":     {},
	"CC-BY-NC-4.0": {},
}

// LicenseCheck is the result of evaluating a single dependency's
// declared license against this package's allow/deny lists.
type LicenseCheck struct {
	// Package is the dependency name being checked.
	Package string `json:"package"`

	// License is the SPDX license identifier declared for Package
	// (e.g. "MIT", "GPL-3.0"). May be blank if the dependency declares
	// no discoverable license, which classifies as LicenseNeedsReview.
	License string `json:"license"`

	// Classification is the real evaluation outcome (see
	// EvaluateLicense).
	Classification LicenseClassification `json:"classification"`
}

// Validate checks c for structural well-formedness.
func (c LicenseCheck) Validate() error {
	if strings.TrimSpace(c.Package) == "" {
		return wrapf("LicenseCheck.Validate", ErrInvalidLicenseCheck)
	}
	switch c.Classification {
	case LicenseAllowed, LicenseDenied, LicenseNeedsReview:
	default:
		return wrapf("LicenseCheck.Validate", ErrInvalidLicenseCheck)
	}
	return nil
}

// normalizeLicenseID trims whitespace so lookups are not sensitive to
// incidental leading/trailing space in scanner output. Matching stays
// case-sensitive to SPDX identifiers on purpose: SPDX license IDs are
// a fixed, canonical casing (e.g. "MIT", not "mit"), and silently
// case-folding risks conflating genuinely distinct identifiers.
func normalizeLicenseID(license string) string {
	return strings.TrimSpace(license)
}

// EvaluateLicense classifies pkg's declared license against the
// allow/deny lists above (task 7's real evaluation function, not a
// stub): an allow-listed SPDX ID is LicenseAllowed, a deny-listed one
// is LicenseDenied, and anything else -- including a blank or
// unrecognized license string -- is LicenseNeedsReview, since an
// unrecognized license is a compliance question for a human, never a
// silent pass.
func EvaluateLicense(pkg, license string) LicenseCheck {
	id := normalizeLicenseID(license)
	classification := LicenseNeedsReview
	switch {
	case id == "":
		classification = LicenseNeedsReview
	case isAllowedLicense(id):
		classification = LicenseAllowed
	case isDeniedLicense(id):
		classification = LicenseDenied
	}
	return LicenseCheck{
		Package:        pkg,
		License:        license,
		Classification: classification,
	}
}

func isAllowedLicense(id string) bool {
	_, ok := allowedLicenses[id]
	return ok
}

func isDeniedLicense(id string) bool {
	_, ok := deniedLicenses[id]
	return ok
}

// EvaluateLicenses runs EvaluateLicense over every (package, license)
// pair in deps, a convenience for batch license-compliance sweeps (as
// opposed to Engine's per-tenant Finding operations, license
// evaluation is stateless, tenant-agnostic reference-data logic --
// exactly like packages/compliance.EvaluateControl, which also takes
// no Engine receiver).
func EvaluateLicenses(deps map[string]string) []LicenseCheck {
	out := make([]LicenseCheck, 0, len(deps))
	for pkg, license := range deps {
		out = append(out, EvaluateLicense(pkg, license))
	}
	return out
}

// NeedsReview filters checks down to those classified LicenseDenied or
// LicenseNeedsReview -- the actionable subset a compliance reviewer
// cares about, since LicenseAllowed entries require no follow-up.
func NeedsReview(checks []LicenseCheck) []LicenseCheck {
	out := make([]LicenseCheck, 0)
	for _, c := range checks {
		if c.Classification == LicenseDenied || c.Classification == LicenseNeedsReview {
			out = append(out, c)
		}
	}
	return out
}
