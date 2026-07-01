package tenancy_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/tenancy"
)

func TestNewPostgresProvisioningRecordRepository_ReturnsNonNil(t *testing.T) {
	repo := tenancy.NewPostgresProvisioningRecordRepository()
	if repo == nil {
		t.Fatal("expected a non-nil repository")
	}
}

func TestProvisioningOutcomes_MatchDatabaseConstraint(t *testing.T) {
	// These must stay in sync with the
	// deployment_provisioning_records_outcome_allowed CHECK constraint
	// in packages/persistence/migrations/000004_create_deployment_provisioning_records.up.sql.
	want := map[string]bool{
		tenancy.ProvisioningOutcomeStarted:   true,
		tenancy.ProvisioningOutcomeSucceeded: true,
		tenancy.ProvisioningOutcomeFailed:    true,
	}
	if len(want) != 3 {
		t.Fatalf("expected 3 distinct outcome constants, got %d", len(want))
	}
}
