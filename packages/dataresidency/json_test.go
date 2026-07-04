package dataresidency_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/dataresidency"
)

// TestReport_JSONRoundTrip proves Report and its nested CheckResult
// values serialize and deserialize losslessly -- a Report is expected
// to cross an API boundary (e.g. a compliance dashboard or a periodic
// verification job's output), so its JSON shape is part of this
// package's contract, not an implementation detail.
func TestReport_JSONRoundTrip(t *testing.T) {
	original := dataresidency.Report{
		DeploymentID: uuid.New(),
		GeneratedAt:  time.Now().UTC().Truncate(time.Second),
		Checks: []dataresidency.CheckResult{
			{Kind: dataresidency.CheckStorageRegion, Passed: true, Region: "eu"},
			{Kind: dataresidency.CheckProviderRegions, Passed: false, Region: "cn", Detail: "not allowed"},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var round dataresidency.Report
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if round.DeploymentID != original.DeploymentID {
		t.Fatalf("DeploymentID mismatch: got %v want %v", round.DeploymentID, original.DeploymentID)
	}
	if !round.GeneratedAt.Equal(original.GeneratedAt) {
		t.Fatalf("GeneratedAt mismatch: got %v want %v", round.GeneratedAt, original.GeneratedAt)
	}
	if len(round.Checks) != 2 {
		t.Fatalf("expected 2 checks after round-trip, got %d", len(round.Checks))
	}
	if round.Passed() {
		t.Fatal("expected round-tripped report to still report as failed")
	}
}

// TestViolationEvent_JSONRoundTrip proves ViolationEvent survives a
// JSON round-trip, since AlertSink implementations (e.g. a webhook
// delivery sink an operator configures) will typically marshal it to
// send externally.
func TestViolationEvent_JSONRoundTrip(t *testing.T) {
	original := dataresidency.ViolationEvent{
		DeploymentID:  uuid.New(),
		TenantID:      uuid.New(),
		ViolationType: dataresidency.ViolationTransferBlocked,
		SourceRegion:  "eu",
		DestRegion:    "cn",
		Reason:        "destination region is not in the allowed list",
		CreatedAt:     time.Now().UTC().Truncate(time.Second),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var round dataresidency.ViolationEvent
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if round != original {
		t.Fatalf("round-trip mismatch: got %+v want %+v", round, original)
	}
}
