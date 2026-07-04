package pilot

import (
	"context"

	"github.com/google/uuid"
)

// AssignCase creates a new PilotCase under deploymentID, supervising
// caseID (a real packages/caselifecycle.Case referenced by ID only --
// see PilotCase's doc comment) via supervisorUserID (task 3),
// requiring managePermission and tenant match. The referenced
// deployment must already exist for tenantID and currently be
// DeploymentStatusActive: supervised pilot cases only run once a
// pilot's jurisdiction/corpus onboarding has completed.
func (e *Engine) AssignCase(ctx context.Context, tenantID uuid.UUID, deploymentID, caseID, supervisorUserID uuid.UUID) (PilotCase, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return PilotCase{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotCase{}, err
	}

	deployment, err := e.deployments.Get(ctx, tenantID, deploymentID)
	if err != nil {
		return PilotCase{}, wrapf("AssignCase", err)
	}
	if deployment.Status != DeploymentStatusActive {
		return PilotCase{}, wrapf("AssignCase", ErrIllegalStatusTransition)
	}

	now := e.now()
	pc := PilotCase{
		ID:               uuid.New(),
		TenantID:         tenantID,
		DeploymentID:     deploymentID,
		CaseID:           caseID,
		SupervisorUserID: supervisorUserID,
		AssignedAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := pc.Validate(); err != nil {
		return PilotCase{}, err
	}
	if err := e.cases.Create(ctx, tenantID, &pc); err != nil {
		return PilotCase{}, wrapf("AssignCase", err)
	}
	return pc, nil
}

// GetCase returns the PilotCase identified by id for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) GetCase(ctx context.Context, tenantID, id uuid.UUID) (PilotCase, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return PilotCase{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotCase{}, err
	}
	c, err := e.cases.Get(ctx, tenantID, id)
	if err != nil {
		return PilotCase{}, wrapf("GetCase", err)
	}
	return *c, nil
}

// ListCasesForDeployment returns every PilotCase assigned under
// deploymentID for tenantID, requiring viewPermission and tenant
// match.
func (e *Engine) ListCasesForDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID) ([]PilotCase, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.cases.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		return nil, wrapf("ListCasesForDeployment", err)
	}
	return list, nil
}

// MarkOutcomeObserved records that pilotCaseID's supervised outcome
// has been observed (task 3's supervision metadata), requiring
// managePermission and tenant match. Idempotent: calling it again on
// an already-observed PilotCase leaves ObservedAt at its original
// value rather than overwriting it.
func (e *Engine) MarkOutcomeObserved(ctx context.Context, tenantID, pilotCaseID uuid.UUID) (PilotCase, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return PilotCase{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotCase{}, err
	}

	c, err := e.cases.Get(ctx, tenantID, pilotCaseID)
	if err != nil {
		return PilotCase{}, wrapf("MarkOutcomeObserved", err)
	}

	now := e.now()
	c.OutcomeObserved = true
	if c.ObservedAt == nil {
		c.ObservedAt = &now
	}
	c.UpdatedAt = now

	if err := e.cases.Update(ctx, tenantID, c); err != nil {
		return PilotCase{}, wrapf("MarkOutcomeObserved", err)
	}
	return *c, nil
}
