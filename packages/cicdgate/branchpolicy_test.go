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
		name    string
		count   int
		wantErr bool
	}{
		{name: "exactly minimum", count: 10, wantErr: false},
		{name: "above minimum", count: 23, wantErr: false},
		{name: "one below minimum", count: 9, wantErr: true},
		{name: "zero commits", count: 0, wantErr: true},
		{name: "well above minimum", count: 100, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePRCommitCount(tt.count)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ValidatePRCommitCount(%d) error = nil, want error", tt.count)
				}
				if !errors.Is(err, ErrInsufficientCommits) {
					t.Errorf("ValidatePRCommitCount(%d) error = %v, want wrapping ErrInsufficientCommits", tt.count, err)
				}
				return
			}
			if err != nil {
				t.Errorf("ValidatePRCommitCount(%d) error = %v, want nil", tt.count, err)
			}
		})
	}
}
