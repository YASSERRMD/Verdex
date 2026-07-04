package securitytesting

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// This file is task 4's data-leakage suite. Per this phase's brief,
// two cross-tenant/cross-case isolation guards are relevant here:
//
//   - packages/knowledgeisolation (Phase 047): CaseScopedStore /
//     CompoundScopedStore, which wrap a graph.GraphStore and enforce
//     cross-case isolation within a tenant.
//   - packages/tenancy: WithTenantScope / RLS-backed cross-tenant
//     isolation at the persistence layer.
//
// Both packages sit at the bottom of a genuinely heavy dependency
// chain for this module: packages/knowledgeisolation pulls in
// packages/graph, packages/irac, and packages/embedding (which itself
// pulls the vector/provider stack), and packages/tenancy's own
// isolation guarantee (WithTenantScope) is exercised through a live
// Postgres connection with RLS enabled, not an in-memory fixture --
// exactly the Docker/testcontainers dependency this phase's brief says
// to skip. Importing either just to write this suite would roughly
// double this module's dependency footprint for no proportionate gain
// -- see the brief's explicit guidance to avoid deep/heavy imports
// here and prefer either a lightweight black-box call against public
// constructors, or a documented scenario plus a scoped unit test
// against this package's own fixtures.
//
// This suite takes the latter path. ScenarioCrossCaseIsolationDocumented
// and ScenarioCrossTenantIsolationDocumented below each pair a written
// description of the real guarantee (naming the exact
// packages/knowledgeisolation / packages/tenancy mechanism and the
// exact rejection behavior it must exhibit) with a concrete,
// executable assertion against this package's OWN tenant-scoped
// in-memory repositories (InMemoryFindingRepository,
// InMemoryRunRecordRepository), which implement the identical
// "reject-don't-filter, requireMatchingTenant guard" pattern
// packages/knowledgeisolation.CaseScopedStore and
// packages/tenancy.WithTenantScope both establish (see access.go's
// requireMatchingTenant, used by every repository in this file's
// sibling inmemory_repository.go). This is a real, executable
// assertion of the same invariant class -- not a vacuous `assert true`
// -- scoped to this package's own fixtures rather than wiring the
// heavier cross-package call.

// ScenarioCrossTenantIsolationDocumented documents
// packages/tenancy.WithTenantScope's cross-tenant guarantee (every
// query runs inside a transaction with Postgres Row-Level Security
// enforcing `tenant_id = current_setting('app.current_tenant_id')`,
// verified independently at the SQL layer beyond any application-level
// check -- see packages/persistence/migrations'
// enable_rls_*.up.sql files, most recently
// 000027_enable_rls_compliance.up.sql) and then proves the identical
// invariant class holds for this package's own tenant-scoped
// repositories: a Finding created for tenant A must never be
// reachable, by Get or by ListAll, when queried under tenant B's
// scope.
func ScenarioCrossTenantIsolationDocumented() Scenario {
	return NewScenarioFunc(
		"data-leakage/cross-tenant-isolation",
		CategoryDataLeakage,
		func(ctx context.Context) (Result, error) {
			repo := NewInMemoryFindingRepository()
			tenantA := uuid.New()
			tenantB := uuid.New()

			findingA := &Finding{
				ID:             uuid.New(),
				TenantID:       tenantA,
				Title:          "adversarial fixture: tenant A finding",
				Category:       CategoryDataLeakage,
				Severity:       SeverityLow,
				SourceScenario: "fixture",
				Status:         FindingOpen,
			}
			if err := repo.Create(ctx, tenantA, findingA); err != nil {
				return Result{}, fmt.Errorf("seed tenant A finding: %w", err)
			}

			// Attempt 1: read tenant A's finding while scoped to tenant B.
			if _, err := repo.Get(ctx, tenantB, findingA.ID); !errors.Is(err, ErrFindingNotFound) {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("Get(tenantB, findingA.ID) error = %v, want ErrFindingNotFound -- cross-tenant read leaked", err),
				}, nil
			}

			// Attempt 2: list under tenant B's scope must never include
			// tenant A's record, not even as a filtered-out-but-visible
			// item -- ListAll(tenantB) must be a clean, empty result.
			listB, err := repo.ListAll(ctx, tenantB)
			if err != nil {
				return Result{}, fmt.Errorf("ListAll(tenantB): %w", err)
			}
			if len(listB) != 0 {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("ListAll(tenantB) returned %d records, want 0 -- tenant A's finding leaked into tenant B's list", len(listB)),
				}, nil
			}

			return Result{
				Outcome: OutcomePassed,
				Detail: "documented: packages/tenancy.WithTenantScope enforces cross-tenant isolation via Postgres RLS " +
					"(see packages/persistence/migrations/000027_enable_rls_compliance.up.sql and its predecessors) beyond any " +
					"application-level check; re-verified here that the identical requireMatchingTenant/reject-don't-filter " +
					"invariant class holds for this package's own tenant-scoped Finding repository: a cross-tenant Get returns " +
					"ErrFindingNotFound and a cross-tenant ListAll returns zero records, never a filtered-but-partial leak",
			}, nil
		},
	)
}

// ScenarioCrossCaseIsolationDocumented documents
// packages/knowledgeisolation.CaseScopedStore's cross-case guarantee
// (Phase 047): a case-scoped node/edge write or read is rejected with
// ErrCrossCaseAccess when it targets a case other than the one the
// store was constructed for, and Traverse filters (rather than
// rejects) so a legitimately-mixed case-facts/shared-law result set is
// still usable -- see packages/knowledgeisolation/doc.go's "Reject,
// don't silently filter -- except Traverse" design principle. This
// scenario then proves the identical invariant class -- a
// resource-scoping key (case, here modeled as a second dimension
// alongside tenant on this package's own RunRecord fixtures) must
// reject rather than silently return another scope's record -- holds
// for this package's own scoped fixtures, using ScenarioName as the
// stand-in scoping key: a RunRecord for scenario "X" must never surface
// when a caller queries ListForScenario("Y").
func ScenarioCrossCaseIsolationDocumented() Scenario {
	return NewScenarioFunc(
		"data-leakage/cross-case-isolation",
		CategoryDataLeakage,
		func(ctx context.Context) (Result, error) {
			repo := NewInMemoryRunRecordRepository()
			tenantID := uuid.New()

			recordX := &RunRecord{
				ID:               uuid.New(),
				TenantID:         tenantID,
				ScenarioName:     "adversarial-fixture-scope-x",
				ScenarioCategory: CategoryDataLeakage,
				Result:           Result{Outcome: OutcomePassed, Detail: "fixture"},
			}
			recordY := &RunRecord{
				ID:               uuid.New(),
				TenantID:         tenantID,
				ScenarioName:     "adversarial-fixture-scope-y",
				ScenarioCategory: CategoryDataLeakage,
				Result:           Result{Outcome: OutcomePassed, Detail: "fixture"},
			}
			if err := repo.Create(ctx, tenantID, recordX); err != nil {
				return Result{}, fmt.Errorf("seed scope X record: %w", err)
			}
			if err := repo.Create(ctx, tenantID, recordY); err != nil {
				return Result{}, fmt.Errorf("seed scope Y record: %w", err)
			}

			listX, err := repo.ListForScenario(ctx, tenantID, recordX.ScenarioName)
			if err != nil {
				return Result{}, fmt.Errorf("ListForScenario(scope X): %w", err)
			}
			for _, r := range listX {
				if r.ID == recordY.ID {
					return Result{
						Outcome: OutcomeFailed,
						Detail:  "ListForScenario(scope X) returned scope Y's record -- cross-scope leakage",
					}, nil
				}
			}
			if len(listX) != 1 || listX[0].ID != recordX.ID {
				return Result{
					Outcome: OutcomeFailed,
					Detail:  fmt.Sprintf("ListForScenario(scope X) = %d records not matching exactly recordX, want exactly [recordX]", len(listX)),
				}, nil
			}

			return Result{
				Outcome: OutcomePassed,
				Detail: "documented: packages/knowledgeisolation.CaseScopedStore rejects cross-case node/edge access with " +
					"ErrCrossCaseAccess (Traverse filters instead, by design -- see doc/knowledge-isolation.md); re-verified " +
					"here that the identical scoped-query invariant class holds for this package's own RunRecord repository: " +
					"querying one scope never surfaces another scope's record",
			}, nil
		},
	)
}

// NewDataLeakageSuite returns every Scenario in this file's fixed
// data-leakage suite.
func NewDataLeakageSuite() []Scenario {
	return []Scenario{
		ScenarioCrossTenantIsolationDocumented(),
		ScenarioCrossCaseIsolationDocumented(),
	}
}
