package treevalidation

import "testing"

func TestReportHasCritical(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     bool
	}{
		{"no findings", nil, false},
		{"only warnings", []Finding{{Severity: SeverityWarning}, {Severity: SeverityInfo}}, false},
		{"one critical", []Finding{{Severity: SeverityWarning}, {Severity: SeverityCritical}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Report{Findings: tt.findings}
			if got := r.HasCritical(); got != tt.want {
				t.Errorf("HasCritical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReportCountBySeverity(t *testing.T) {
	r := Report{Findings: []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityCritical},
		{Severity: SeverityWarning},
		{Severity: SeverityInfo},
		{Severity: SeverityInfo},
		{Severity: SeverityInfo},
	}}

	if got := r.CountBySeverity(SeverityCritical); got != 2 {
		t.Errorf("CountBySeverity(critical) = %d, want 2", got)
	}
	if got := r.CountBySeverity(SeverityWarning); got != 1 {
		t.Errorf("CountBySeverity(warning) = %d, want 1", got)
	}
	if got := r.CountBySeverity(SeverityInfo); got != 3 {
		t.Errorf("CountBySeverity(info) = %d, want 3", got)
	}
}

func TestReportSummary(t *testing.T) {
	r := Report{Findings: []Finding{
		{Severity: SeverityCritical},
		{Severity: SeverityWarning},
		{Severity: SeverityWarning},
	}}
	want := "1 critical, 2 warning, 0 info (3 total)"
	if got := r.Summary(); got != want {
		t.Errorf("Summary() = %q, want %q", got, want)
	}
}

func TestReportString(t *testing.T) {
	r := Report{CaseID: "case-1", Findings: []Finding{{Severity: SeverityCritical}}}
	got := r.String()
	if got == "" {
		t.Fatal("String() returned empty string")
	}
}
