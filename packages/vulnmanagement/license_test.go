package vulnmanagement_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestEvaluateLicense(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		license string
		want    vulnmanagement.LicenseClassification
	}{
		{"MIT allowed", "MIT", vulnmanagement.LicenseAllowed},
		{"Apache-2.0 allowed", "Apache-2.0", vulnmanagement.LicenseAllowed},
		{"BSD-3-Clause allowed", "BSD-3-Clause", vulnmanagement.LicenseAllowed},
		{"GPL-3.0 denied", "GPL-3.0", vulnmanagement.LicenseDenied},
		{"AGPL-3.0 denied", "AGPL-3.0", vulnmanagement.LicenseDenied},
		{"blank needs review", "", vulnmanagement.LicenseNeedsReview},
		{"unknown needs review", "Some-Custom-EULA", vulnmanagement.LicenseNeedsReview},
		{"whitespace-only needs review", "   ", vulnmanagement.LicenseNeedsReview},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := vulnmanagement.EvaluateLicense("example-pkg", c.license)
			if got.Classification != c.want {
				t.Errorf("EvaluateLicense(%q).Classification = %s, want %s", c.license, got.Classification, c.want)
			}
			if got.Package != "example-pkg" {
				t.Errorf("got.Package = %s, want example-pkg", got.Package)
			}
		})
	}
}

func TestEvaluateLicense_CaseSensitive(t *testing.T) {
	t.Parallel()
	// SPDX identifiers are canonically cased; lowercase "mit" must not
	// silently match the "MIT" allow-list entry.
	got := vulnmanagement.EvaluateLicense("example-pkg", "mit")
	if got.Classification != vulnmanagement.LicenseNeedsReview {
		t.Errorf(`EvaluateLicense("mit").Classification = %s, want LicenseNeedsReview`, got.Classification)
	}
}

func TestEvaluateLicenses(t *testing.T) {
	t.Parallel()
	deps := map[string]string{
		"pkg-a": "MIT",
		"pkg-b": "GPL-3.0",
		"pkg-c": "Unknown-License",
	}
	checks := vulnmanagement.EvaluateLicenses(deps)
	if len(checks) != 3 {
		t.Fatalf("len(checks) = %d, want 3", len(checks))
	}

	byPkg := make(map[string]vulnmanagement.LicenseCheck, len(checks))
	for _, c := range checks {
		byPkg[c.Package] = c
	}
	if byPkg["pkg-a"].Classification != vulnmanagement.LicenseAllowed {
		t.Errorf("pkg-a classification = %s, want LicenseAllowed", byPkg["pkg-a"].Classification)
	}
	if byPkg["pkg-b"].Classification != vulnmanagement.LicenseDenied {
		t.Errorf("pkg-b classification = %s, want LicenseDenied", byPkg["pkg-b"].Classification)
	}
	if byPkg["pkg-c"].Classification != vulnmanagement.LicenseNeedsReview {
		t.Errorf("pkg-c classification = %s, want LicenseNeedsReview", byPkg["pkg-c"].Classification)
	}
}

func TestNeedsReview(t *testing.T) {
	t.Parallel()
	checks := []vulnmanagement.LicenseCheck{
		vulnmanagement.EvaluateLicense("allowed-pkg", "MIT"),
		vulnmanagement.EvaluateLicense("denied-pkg", "GPL-2.0"),
		vulnmanagement.EvaluateLicense("review-pkg", "Weird-License"),
	}
	review := vulnmanagement.NeedsReview(checks)
	if len(review) != 2 {
		t.Fatalf("len(review) = %d, want 2", len(review))
	}
	for _, c := range review {
		if c.Package == "allowed-pkg" {
			t.Errorf("NeedsReview unexpectedly included an allowed package: %v", c)
		}
	}
}

func TestLicenseCheck_Validate(t *testing.T) {
	t.Parallel()

	valid := vulnmanagement.LicenseCheck{Package: "pkg", License: "MIT", Classification: vulnmanagement.LicenseAllowed}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid LicenseCheck.Validate() = %v, want nil", err)
	}

	blankPkg := valid
	blankPkg.Package = ""
	if err := blankPkg.Validate(); err == nil {
		t.Error("blank-package LicenseCheck.Validate() = nil, want error")
	}

	badClass := valid
	badClass.Classification = "bogus"
	if err := badClass.Validate(); err == nil {
		t.Error("invalid-classification LicenseCheck.Validate() = nil, want error")
	}
}
