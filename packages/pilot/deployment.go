package pilot

import (
	"context"

	"github.com/google/uuid"
)

// ProvisionDeployment creates a new PilotDeployment for tenantID (task
// 1), requiring managePermission and tenant match. A freshly
// provisioned deployment always starts at DeploymentStatusProvisioning
// regardless of what the caller supplies, since a brand-new deployment
// has not onboarded a jurisdiction or corpus yet by definition. Every
// call is recorded via AuditSink regardless of outcome.
func (e *Engine) ProvisionDeployment(ctx context.Context, tenantID uuid.UUID, d PilotDeployment) (PilotDeployment, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, actorFromCtx(ctx), d.ID, "", DeploymentStatusProvisioning, err)
		}
		return PilotDeployment{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, d.ID, "", DeploymentStatusProvisioning, err)
		}
		return PilotDeployment{}, err
	}

	d.TenantID = tenantID
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	d.Status = DeploymentStatusProvisioning
	d.CreatedBy = user.ID
	now := e.now()
	if d.StartDate.IsZero() {
		d.StartDate = now
	}
	d.CreatedAt = now
	d.UpdatedAt = now

	if err := d.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, d.ID, "", d.Status, err)
		}
		return PilotDeployment{}, err
	}
	if err := e.deployments.Create(ctx, tenantID, &d); err != nil {
		wrapped := wrapf("ProvisionDeployment", err)
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, d.ID, "", d.Status, wrapped)
		}
		return PilotDeployment{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, d.ID, "", d.Status, nil)
	}
	return d, nil
}

// GetDeployment returns the PilotDeployment identified by id for
// tenantID, requiring viewPermission and tenant match.
func (e *Engine) GetDeployment(ctx context.Context, tenantID, id uuid.UUID) (PilotDeployment, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return PilotDeployment{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return PilotDeployment{}, err
	}
	d, err := e.deployments.Get(ctx, tenantID, id)
	if err != nil {
		return PilotDeployment{}, wrapf("GetDeployment", err)
	}
	return *d, nil
}

// ListDeployments returns every PilotDeployment recorded for tenantID,
// requiring viewPermission and tenant match.
func (e *Engine) ListDeployments(ctx context.Context, tenantID uuid.UUID) ([]PilotDeployment, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.deployments.ListAll(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListDeployments", err)
	}
	return list, nil
}

// TransitionDeployment moves deploymentID from its current
// DeploymentStatus to to (task 2's corpus-onboarding transition and
// every later lifecycle move), requiring managePermission and tenant
// match. The transition from the deployment's current Status to to
// must be legal per CanTransitionDeployment, or TransitionDeployment
// fails with ErrIllegalStatusTransition before any state changes -- an
// illegal attempt is still recorded via AuditSink as a denied
// transition.
func (e *Engine) TransitionDeployment(ctx context.Context, tenantID, deploymentID uuid.UUID, to DeploymentStatus) (PilotDeployment, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, actorFromCtx(ctx), deploymentID, "", to, err)
		}
		return PilotDeployment{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, "", to, err)
		}
		return PilotDeployment{}, err
	}
	if !to.IsValid() {
		wrapped := wrapf("TransitionDeployment", ErrInvalidDeployment)
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, "", to, wrapped)
		}
		return PilotDeployment{}, wrapped
	}

	d, err := e.deployments.Get(ctx, tenantID, deploymentID)
	if err != nil {
		wrapped := wrapf("TransitionDeployment", err)
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, "", to, wrapped)
		}
		return PilotDeployment{}, wrapped
	}

	from := d.Status
	if !CanTransitionDeployment(from, to) {
		wrapped := wrapf("TransitionDeployment", ErrIllegalStatusTransition)
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, from, to, wrapped)
		}
		return PilotDeployment{}, wrapped
	}

	now := e.now()
	d.Status = to
	d.UpdatedAt = now
	if to == DeploymentStatusConcluded && d.EndDate.IsZero() {
		d.EndDate = now
	}

	if err := e.deployments.Update(ctx, tenantID, d); err != nil {
		wrapped := wrapf("TransitionDeployment", err)
		if e.audit != nil {
			_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, from, to, wrapped)
		}
		return PilotDeployment{}, wrapped
	}

	if e.audit != nil {
		_, _ = e.audit.RecordDeploymentStatusChange(ctx, tenantID, user.ID, deploymentID, from, to, nil)
	}
	return *d, nil
}
