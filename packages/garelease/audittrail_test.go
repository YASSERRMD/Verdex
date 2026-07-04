package garelease_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/garelease"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/observability"
)

// brokenChainStore is a minimal garelease.AuditTrailStore fake that
// always reports its hash chain as broken, used to prove
// VerifyAuditTrail is a harness that CAN fail -- not a rubber stamp --
// by exercising the exact discrimination point without needing to
// hand-corrupt a real *auditlog.Store's persisted hash chain (which
// would require reaching into that package's internals or a Postgres
// backend this unit test deliberately avoids per this phase's brief).
type brokenChainStore struct {
	queryErr error
}

func (s brokenChainStore) Query(_ context.Context, _ uuid.UUID, _ auditlog.Filter) ([]auditlog.Event, error) {
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return []auditlog.Event{{}}, nil
}

func (brokenChainStore) VerifyTenantChain(_ context.Context, _ uuid.UUID) (bool, int, error) {
	return false, 3, nil
}

var _ garelease.AuditTrailStore = brokenChainStore{}

func TestVerifyAuditTrail_NilStore(t *testing.T) {
	engine := newTestEngine(t)
	_, err := engine.VerifyAuditTrail(context.Background(), nil, uuid.New())
	if !errors.Is(err, garelease.ErrNilAuditStore) {
		t.Fatalf("VerifyAuditTrail(nil store) = %v, want ErrNilAuditStore", err)
	}
}

func TestVerifyAuditTrail_EmptyTenantID(t *testing.T) {
	engine := newTestEngine(t)
	store, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	_, err = engine.VerifyAuditTrail(context.Background(), store, uuid.Nil)
	if !errors.Is(err, garelease.ErrEmptyRepresentativeTenantID) {
		t.Fatalf("VerifyAuditTrail(nil tenant) = %v, want ErrEmptyRepresentativeTenantID", err)
	}
}

func TestVerifyAuditTrail_QueryError(t *testing.T) {
	engine := newTestEngine(t)
	store := brokenChainStore{queryErr: errors.New("boom")}

	verification, err := engine.VerifyAuditTrail(context.Background(), store, uuid.New())
	if err != nil {
		t.Fatalf("VerifyAuditTrail returned an error, want a failed AuditVerification instead: %v", err)
	}
	if verification.Passed {
		t.Fatalf("VerifyAuditTrail with a broken Query = Passed true, want false")
	}
	if len(verification.Failures()) == 0 {
		t.Fatalf("VerifyAuditTrail with a broken Query = no failures reported, want at least one")
	}
}

func TestVerifyAuditTrail_BrokenChainIsDetected(t *testing.T) {
	// This is the harness-that-can-fail proof for the audit dimension:
	// a store that reports its chain as broken must make
	// VerifyAuditTrail report Passed == false, never true.
	engine := newTestEngine(t)
	store := brokenChainStore{}

	verification, err := engine.VerifyAuditTrail(context.Background(), store, uuid.New())
	if err != nil {
		t.Fatalf("VerifyAuditTrail: %v", err)
	}
	if verification.Passed {
		t.Fatalf("VerifyAuditTrail against a deliberately broken chain = Passed true, want false")
	}

	chainResult, found := findAuditCheck(verification, "audit_chain_intact")
	if !found {
		t.Fatalf("audit_chain_intact assertion not reported")
	}
	if chainResult.Passed {
		t.Fatalf("audit_chain_intact assertion reported Passed true against a broken chain")
	}
}

func TestVerifyAuditTrail_IntactChainPasses(t *testing.T) {
	engine := newTestEngine(t)
	store, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	tenantID := uuid.New()
	if _, err := store.Append(context.Background(), auditlog.Event{
		TenantID: tenantID,
		Kind:     auditlog.KindSystem,
		AuditEvent: observability.AuditEvent{
			Actor:   "system:test",
			Action:  "test.seed",
			Target:  tenantID.String(),
			Outcome: "seeded",
		},
	}); err != nil {
		t.Fatalf("seeding audit event: %v", err)
	}

	verification, err := engine.VerifyAuditTrail(withAuditReader(tenantID), store, tenantID)
	if err != nil {
		t.Fatalf("VerifyAuditTrail: %v", err)
	}
	if !verification.Passed {
		t.Fatalf("VerifyAuditTrail against an intact, seeded chain = Passed false, want true. Failures: %+v", verification.Failures())
	}
}

// findAuditCheck locates the AuditCheckResult named name within
// verification.Results.
func findAuditCheck(verification garelease.AuditVerification, name string) (garelease.AuditCheckResult, bool) {
	for _, r := range verification.Results {
		if r.Name == name {
			return r, true
		}
	}
	return garelease.AuditCheckResult{}, false
}

// withAuditReader returns a context carrying a RoleAdmin user (holds
// PermAuditRead) scoped to tenantID, since
// packages/auditlog.Store.Query/VerifyTenantChain both require an
// authenticated actor holding PermAuditRead whose TenantID exactly
// matches the tenant being queried.
func withAuditReader(tenantID uuid.UUID) context.Context {
	return ctxWithUser(newTestUserForTenant(tenantID, identity.RoleAdmin))
}
