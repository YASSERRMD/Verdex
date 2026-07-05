package cicdgate

import (
	"errors"
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		wantErr    bool
	}{
		{name: "valid phase branch", branchName: "phase-095-cicd-hardening", wantErr: false},
		{name: "valid phase branch with multi-segment slug", branchName: "phase-007-jurisdiction-loader", wantErr: false},
		{name: "valid three digit phase branch", branchName: "phase-100-future-phase", wantErr: false},
		{name: "valid fix branch", branchName: "fix-notifications-access-check", wantErr: false},
		{name: "valid single-segment fix branch", branchName: "fix-typo", wantErr: false},
		{name: "valid dependabot npm branch", branchName: "dependabot/npm_and_yarn/tailwindcss-4.3.2", wantErr: false},
		{name: "valid dependabot github_actions branch", branchName: "dependabot/github_actions/actions/checkout-7", wantErr: false},
		{name: "missing phase number", branchName: "phase-cicd-hardening", wantErr: true},
		{name: "single digit phase number", branchName: "phase-9-cicd-hardening", wantErr: true},
		{name: "no slug", branchName: "phase-095", wantErr: true},
		{name: "uppercase slug", branchName: "phase-095-CICD-Hardening", wantErr: true},
		{name: "main branch", branchName: "main", wantErr: true},
		{name: "empty string", branchName: "", wantErr: true},
		{name: "feature prefix not recognized", branchName: "feature-cicd-hardening", wantErr: true},
		{name: "fix branch with trailing hyphen", branchName: "fix-", wantErr: true},
		{name: "phase branch with underscore", branchName: "phase-095-cicd_hardening", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branchName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidateBranchName(%q) error = nil, want error", tt.branchName)
				}
				if !errors.Is(err, ErrInvalidBranchName) {
					t.Errorf("ValidateBranchName(%q) error = %v, want wrapping ErrInvalidBranchName", tt.branchName, err)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidateBranchName(%q) error = %v, want nil", tt.branchName, err)
			}
		})
	}
}

func TestValidatePRCommitCount(t *testing.T) {
	tests := []struct {
		name       string
		branchName string
		count      int
		wantErr    bool
	}{
		{name: "phase branch exactly minimum", branchName: "phase-095-cicd-hardening", count: 10, wantErr: false},
		{name: "phase branch above minimum", branchName: "phase-095-cicd-hardening", count: 23, wantErr: false},
		{name: "phase branch one below minimum", branchName: "phase-095-cicd-hardening", count: 9, wantErr: true},
		{name: "phase branch zero commits", branchName: "phase-095-cicd-hardening", count: 0, wantErr: true},
		{name: "phase branch well above minimum", branchName: "phase-095-cicd-hardening", count: 100, wantErr: false},

		// fix-slug branches are this repository's convention for
		// small, non-phase corrective work (see ValidateBranchName)
		// and are exempt from the phase-sized commit minimum,
		// matching every fix-* PR merged before this check existed.
		{name: "fix branch single commit", branchName: "fix-notifications-access-check", count: 1, wantErr: false},
		{name: "fix branch zero commits", branchName: "fix-typo", count: 0, wantErr: false},

		// Dependabot branches are this repository's convention for
		// automated dependency-update PRs (see ValidateBranchName) and
		// are exempt from the phase-sized commit minimum -- Dependabot
		// always opens exactly one commit per bump.
		{name: "dependabot branch single commit", branchName: "dependabot/npm_and_yarn/tailwindcss-4.3.2", count: 1, wantErr: false},

		// A malformed branch name is not exempt -- ValidateBranchName
		// already rejects it separately, but ValidatePRCommitCount
		// must not silently waive the count check for it too.
		{name: "malformed branch name below minimum", branchName: "feature-cicd-hardening", count: 1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePRCommitCount(tt.branchName, tt.count)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidatePRCommitCount(%q, %d) error = nil, want error", tt.branchName, tt.count)
				}
				if !errors.Is(err, ErrInsufficientCommits) {
					t.Errorf("ValidatePRCommitCount(%q, %d) error = %v, want wrapping ErrInsufficientCommits", tt.branchName, tt.count, err)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidatePRCommitCount(%q, %d) error = %v, want nil", tt.branchName, tt.count, err)
			}
		})
	}
}
