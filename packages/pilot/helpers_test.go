package pilot_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/pilot"
)

// pilotNow returns the current UTC time, a small named wrapper so test
// call sites read as intent ("a fresh timestamp for this fixture")
// rather than a bare time.Now().UTC() repeated at every call site.
func pilotNow() time.Time {
	return time.Now().UTC()
}

// floatsClose reports whether a and b differ by no more than a small
// epsilon, used throughout this test suite to compare arithmetic-mean
// aggregates without tripping over binary floating-point rounding
// (e.g. (0.8+0.9)/2 == 0.8500000000000001, not exactly 0.85).
func floatsClose(a, b float64) bool {
	const epsilon = 1e-9
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "pilot@example.test",
		Name:     "Test User",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManagePilot and PermViewPilot) scoped to tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewPilot) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// testEngine bundles a *pilot.Engine together with the fresh tenant ID
// it was built for and the *auditlog.Store its AuditSink writes to, so
// tests can exercise a full round-trip without repeating this wiring.
type testEngine struct {
	engine     *pilot.Engine
	auditStore *auditlog.Store
	tenantID   uuid.UUID
}

// newTestEngine builds a pilot.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink, returning the
// bundle and a fresh tenant ID.
func newTestEngine(t *testing.T) *testEngine {
	t.Helper()

	deployments := pilot.NewInMemoryDeploymentRepository()
	cases := pilot.NewInMemoryCaseRepository()
	feedback := pilot.NewInMemoryFeedbackRepository()
	findings := pilot.NewInMemoryFindingRepository()
	refinements := pilot.NewInMemoryRefinementRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := pilot.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("pilot.NewAuditSink: %v", err)
	}

	engine, err := pilot.NewEngine(deployments, cases, feedback, findings, refinements, sink)
	if err != nil {
		t.Fatalf("pilot.NewEngine: %v", err)
	}
	return &testEngine{engine: engine, auditStore: auditStore, tenantID: uuid.New()}
}

// provisionAndActivate provisions a PilotDeployment for te's tenant as
// adminUser and drives it straight through to DeploymentStatusActive
// (Provisioning -> CorpusOnboarding -> Active), returning the active
// deployment -- the state most PilotCase/feedback/finding tests need
// to start from.
func provisionAndActivate(t *testing.T, te *testEngine) pilot.PilotDeployment {
	t.Helper()
	admin := adminUser(te.tenantID)

	d, err := te.engine.ProvisionDeployment(ctxWithUser(admin), te.tenantID, pilot.PilotDeployment{
		Name:             "Test pilot",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ProvisionDeployment: %v", err)
	}
	d, err = te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, d.ID, pilot.DeploymentStatusCorpusOnboarding)
	if err != nil {
		t.Fatalf("TransitionDeployment (onboarding): %v", err)
	}
	d, err = te.engine.TransitionDeployment(ctxWithUser(admin), te.tenantID, d.ID, pilot.DeploymentStatusActive)
	if err != nil {
		t.Fatalf("TransitionDeployment (active): %v", err)
	}
	return d
}

// assignTestCase assigns a PilotCase under deploymentID for te's
// tenant as adminUser, returning it.
func assignTestCase(t *testing.T, te *testEngine, deploymentID uuid.UUID) pilot.PilotCase {
	t.Helper()
	admin := adminUser(te.tenantID)
	pc, err := te.engine.AssignCase(ctxWithUser(admin), te.tenantID, deploymentID, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("AssignCase: %v", err)
	}
	return pc
}

// submitTestFeedback submits a single valid FeedbackEntry against
// pilotCaseID for te's tenant as adminUser, returning it.
func submitTestFeedback(t *testing.T, te *testEngine, pilotCaseID uuid.UUID) pilot.FeedbackEntry {
	t.Helper()
	admin := adminUser(te.tenantID)
	entry, err := te.engine.SubmitFeedback(ctxWithUser(admin), te.tenantID, pilot.FeedbackEntry{
		PilotCaseID: pilotCaseID,
		Ratings: []pilot.DimensionRating{
			{Dimension: pilot.DimensionGrounding, Score: 0.8},
			{Dimension: pilot.DimensionCitation, Score: 0.9},
		},
		Trust:    pilot.TrustHigh,
		Comments: "Solid grounding, citations checked out.",
	})
	if err != nil {
		t.Fatalf("SubmitFeedback: %v", err)
	}
	return entry
}

// recordTestFinding records a single valid PilotFinding under
// deploymentID for te's tenant as adminUser, sourced from
// feedbackIDs, returning it.
func recordTestFinding(t *testing.T, te *testEngine, deploymentID uuid.UUID, feedbackIDs ...uuid.UUID) pilot.PilotFinding {
	t.Helper()
	admin := adminUser(te.tenantID)
	f, err := te.engine.RecordFinding(ctxWithUser(admin), te.tenantID, pilot.PilotFinding{
		DeploymentID:      deploymentID,
		SourceFeedbackIDs: feedbackIDs,
		Title:             "Test finding",
		Description:       "A finding recorded for test fixtures.",
		Priority:          pilot.PriorityMedium,
	})
	if err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}
	return f
}
