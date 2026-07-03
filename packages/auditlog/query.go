package auditlog

import (
	"context"

	"github.com/google/uuid"
)

// Query returns events for tenantID matching filter, requiring the
// authenticated actor on ctx to hold identity.PermAuditRead (task 8)
// and to belong to tenantID (task 9's tenant-isolation requirement —
// an auditor from tenant A can never query tenant B's trail, even
// though PermAuditRead is a role-level, not tenant-level, grant).
func (s *Store) Query(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error) {
	user, err := authorizeAuditRead(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, wrapf("Query", ErrEmptyTenantID)
	}

	events, err := s.repo.Query(ctx, tenantID, filter)
	if err != nil {
		return nil, wrapf("Query", err)
	}
	return events, nil
}
