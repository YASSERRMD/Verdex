package guardrail_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

func TestNoSignoffRecordedGateAlwaysPending(t *testing.T) {
	gate := guardrail.NoSignoffRecordedGate{}

	status, err := gate.Status(context.Background(), "case-1")
	if err != nil {
		t.Fatalf("Status(case-1) error = %v, want nil", err)
	}
	if status != guardrail.SignoffPending {
		t.Fatalf("Status(case-1) = %v, want SignoffPending (fail-closed default)", status)
	}
}

func TestNoSignoffRecordedGateEmptyCaseID(t *testing.T) {
	gate := guardrail.NoSignoffRecordedGate{}
	_, err := gate.Status(context.Background(), "")
	if !errors.Is(err, guardrail.ErrEmptyCaseID) {
		t.Fatalf("Status(\"\") error = %v, want errors.Is ErrEmptyCaseID", err)
	}
}

// stubSignoffGate lets tests control the reported SignoffStatus/error,
// simulating a future Phase 068 implementation.
type stubSignoffGate struct {
	status SignoffStatusOrErr
}

// SignoffStatusOrErr bundles a canned Status()/err pair for stubSignoffGate.
type SignoffStatusOrErr struct {
	Status guardrail.SignoffStatus
	Err    error
}

func (g stubSignoffGate) Status(_ context.Context, _ string) (guardrail.SignoffStatus, error) {
	return g.status.Status, g.status.Err
}

func TestCanFinalizeRequiresApproval(t *testing.T) {
	tests := []struct {
		name    string
		gate    guardrail.SignoffGate
		caseID  string
		wantOK  bool
		wantErr error
	}{
		{
			name:    "no signoff recorded gate blocks",
			gate:    guardrail.NoSignoffRecordedGate{},
			caseID:  "case-1",
			wantOK:  false,
			wantErr: guardrail.ErrSignoffNotApproved,
		},
		{
			name:    "explicitly pending blocks",
			gate:    stubSignoffGate{status: SignoffStatusOrErr{Status: guardrail.SignoffPending}},
			caseID:  "case-1",
			wantOK:  false,
			wantErr: guardrail.ErrSignoffNotApproved,
		},
		{
			name:    "rejected blocks",
			gate:    stubSignoffGate{status: SignoffStatusOrErr{Status: guardrail.SignoffRejected}},
			caseID:  "case-1",
			wantOK:  false,
			wantErr: guardrail.ErrSignoffNotApproved,
		},
		{
			name:   "approved passes",
			gate:   stubSignoffGate{status: SignoffStatusOrErr{Status: guardrail.SignoffApproved}},
			caseID: "case-1",
			wantOK: true,
		},
		{
			name:    "nil gate blocks",
			gate:    nil,
			caseID:  "case-1",
			wantOK:  false,
			wantErr: guardrail.ErrNilSignoffGate,
		},
		{
			name:    "empty case id blocks",
			gate:    guardrail.NoSignoffRecordedGate{},
			caseID:  "",
			wantOK:  false,
			wantErr: guardrail.ErrEmptyCaseID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := guardrail.CanFinalize(context.Background(), tt.caseID, tt.gate)
			if ok != tt.wantOK {
				t.Fatalf("CanFinalize() ok = %v, want %v (err=%v)", ok, tt.wantOK, err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("CanFinalize() error = %v, want errors.Is %v", err, tt.wantErr)
			}
			if tt.wantOK && err != nil {
				t.Fatalf("CanFinalize() = true but err = %v, want nil", err)
			}
		})
	}
}

func TestCanFinalizeGateError(t *testing.T) {
	sentinel := errors.New("workflow backend unavailable")
	gate := stubSignoffGate{status: SignoffStatusOrErr{Err: sentinel}}

	ok, err := guardrail.CanFinalize(context.Background(), "case-1", gate)
	if ok {
		t.Fatal("CanFinalize() ok = true, want false when gate errors")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("CanFinalize() error = %v, want errors.Is underlying gate error", err)
	}
}

func TestSignoffStatusString(t *testing.T) {
	tests := map[guardrail.SignoffStatus]string{
		guardrail.SignoffPending:  "pending",
		guardrail.SignoffApproved: "approved",
		guardrail.SignoffRejected: "rejected",
	}
	for status, want := range tests {
		if got := status.String(); got != want {
			t.Errorf("SignoffStatus(%d).String() = %q, want %q", int(status), got, want)
		}
	}
}
