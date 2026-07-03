package analytics_test

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser mirrors packages/reasoningeval/helpers_test.go's fixture
// convention.
func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// advocateContext carries a user who can view cases but has no audit
// (cost/usage) permission.
func advocateContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleAdvocate))
}

// judgeContext carries a user who can view cases and holds
// identity.PermAuditRead, so cost/usage views are visible.
func judgeContext() context.Context {
	return identity.WithUser(context.Background(), newTestUser(identity.RoleJudge))
}

func unauthedContext() context.Context {
	return context.Background()
}

// seedCase creates and persists a case with the given state, category,
// and jurisdiction, backdating CreatedAt by ageDays so CreatedTrend
// tests can assert on distinct days.
func seedCase(
	t interface {
		Helper()
		Fatalf(format string, args ...any)
	},
	repo *caselifecycle.InMemoryRepository,
	tenantID, jurisdictionID uuid.UUID,
	category string,
	state caselifecycle.State,
	createdAt time.Time,
) *caselifecycle.Case {
	t.Helper()
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: jurisdictionID,
		CategoryID:     category,
		Title:          "Test Case",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase() error = %v", err)
	}
	c.State = state
	c.CreatedAt = createdAt
	c.UpdatedAt = createdAt
	if err := repo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("repo.Create() error = %v", err)
	}
	return c
}
