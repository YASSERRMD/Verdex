package keymanagement

import (
	"context"

	"github.com/google/uuid"
)

// Service orchestrates key operations across a Provider (key
// material), a Repository (key metadata, used for read paths that do
// not need to touch material), an AuditRecorder (every operation
// logged and persisted), and a BreakGlassStore (breakglass.go),
// applying role-gated access control (access.go) at every entry
// point. Service is the type callers outside this package are
// expected to use — Provider/Repository/Adapter remain available
// directly for lower-level composition (e.g. Adapter wraps Provider
// without Service's permission gating, since packages/encryption's
// Encrypt/Decrypt call sites are trusted internal infrastructure, not
// end-user-facing requests — see adapter.go).
type Service struct {
	provider   Provider
	repo       Repository
	audit      *AuditRecorder
	breakGlass BreakGlassStore
}

// NewService builds a Service. provider, repo, and audit must be
// non-nil; breakGlass may be nil only if the caller never invokes
// GrantBreakGlass/UseBreakGlass (both would then panic on a nil-map
// dereference inside a nil BreakGlassStore, so production callers
// should always supply one — NewInMemoryBreakGlassStore is available
// for tests and single-node deployments).
func NewService(provider Provider, repo Repository, audit *AuditRecorder, breakGlass BreakGlassStore) (*Service, error) {
	if provider == nil {
		return nil, ErrNilProvider
	}
	if repo == nil {
		return nil, ErrNilRepository
	}
	if audit == nil {
		return nil, wrapf("NewService", ErrNilRepository)
	}
	return &Service{provider: provider, repo: repo, audit: audit, breakGlass: breakGlass}, nil
}

// recordAudit is a best-effort wrapper around s.audit.Record: audit
// recording failures are swallowed (logged is still attempted inside
// AuditRecorder.Record before the repository write) rather than
// propagated, so a transient audit-store outage never blocks the
// underlying key operation the caller actually asked for. This
// mirrors the "a stub channel failing must never roll back or block"
// posture packages/notifications documents for its own best-effort
// side channel.
func (s *Service) recordAudit(ctx context.Context, tenantID uuid.UUID, action AuditAction, keyID string, outcome AuditOutcome, justification, detail string) {
	_ = s.audit.Record(ctx, AuditEntry{
		TenantID:      tenantID,
		Actor:         currentActor(ctx),
		Action:        action,
		KeyID:         keyID,
		Outcome:       outcome,
		Justification: justification,
		Detail:        detail,
	})
}

// CurrentKey returns the tenant's current key material via the
// underlying Provider. Gated on viewPermission (reading key material
// to encrypt/decrypt is a read-like operation from an access-control
// perspective) and audited as AuditActionCurrentKey regardless of
// outcome.
func (s *Service) CurrentKey(ctx context.Context, tenantID uuid.UUID) (KeyMaterial, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionCurrentKey, "", err)
		return KeyMaterial{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionCurrentKey, "", err)
		return KeyMaterial{}, err
	}

	material, err := s.provider.CurrentKey(ctx, tenantID.String())
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionCurrentKey, "", AuditOutcomeError, "", err.Error())
		return KeyMaterial{}, wrapf("Service.CurrentKey", err)
	}
	s.recordAudit(ctx, tenantID, AuditActionCurrentKey, material.Metadata.ID, AuditOutcomeSuccess, "", "")
	return material, nil
}

// Key returns keyID's material via the underlying Provider. Gated and
// audited identically to CurrentKey.
func (s *Service) Key(ctx context.Context, tenantID uuid.UUID, keyID string) (KeyMaterial, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionKeyLookup, keyID, err)
		return KeyMaterial{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionKeyLookup, keyID, err)
		return KeyMaterial{}, err
	}

	material, err := s.provider.Key(ctx, tenantID.String(), keyID)
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionKeyLookup, keyID, AuditOutcomeError, "", err.Error())
		return KeyMaterial{}, wrapf("Service.Key", err)
	}
	s.recordAudit(ctx, tenantID, AuditActionKeyLookup, keyID, AuditOutcomeSuccess, "", "")
	return material, nil
}

// Rotate creates a new Active key version for tenantID via the
// underlying Provider (task 3: "creates a new active key version
// while marking the prior version retired-but-still-decryptable").
// Gated on managePermission (task 5) and heavily audited
// (AuditActionRotate) regardless of outcome.
func (s *Service) Rotate(ctx context.Context, tenantID uuid.UUID) (string, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionRotate, "", err)
		return "", err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionRotate, "", err)
		return "", err
	}

	meta, err := s.provider.Rotate(ctx, tenantID.String())
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionRotate, "", AuditOutcomeError, "", err.Error())
		return "", wrapf("Service.Rotate", err)
	}
	s.recordAudit(ctx, tenantID, AuditActionRotate, meta.ID, AuditOutcomeSuccess, "", "")
	return meta.ID, nil
}

// Revoke transitions keyID to KeyStateRevoked via the Repository.
// Gated on managePermission and audited as AuditActionRevoke.
// Revoking a key does not delete its metadata or material — see
// KeyStateRevoked's doc comment on why revocation does not
// retroactively break decryption of ciphertext already written under
// it.
func (s *Service) Revoke(ctx context.Context, tenantID uuid.UUID, keyID string) error {
	user, err := authorizeManage(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionRevoke, keyID, err)
		return err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionRevoke, keyID, err)
		return err
	}

	if err := s.repo.UpdateState(ctx, tenantID, keyID, KeyStateRevoked); err != nil {
		s.recordAudit(ctx, tenantID, AuditActionRevoke, keyID, AuditOutcomeError, "", err.Error())
		return wrapf("Service.Revoke", err)
	}
	s.recordAudit(ctx, tenantID, AuditActionRevoke, keyID, AuditOutcomeSuccess, "", "")
	return nil
}

// ListKeys returns every key version's metadata for tenantID, gated
// on viewPermission and audited as AuditActionViewMetadata. This
// never touches key material — only Repository is consulted.
func (s *Service) ListKeys(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]*KeyMetadata, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		s.auditDenied(ctx, tenantID, AuditActionViewMetadata, "", err)
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		s.auditDenied(ctx, tenantID, AuditActionViewMetadata, "", err)
		return nil, err
	}

	list, err := s.repo.ListForTenant(ctx, tenantID, filter)
	if err != nil {
		s.recordAudit(ctx, tenantID, AuditActionViewMetadata, "", AuditOutcomeError, "", err.Error())
		return nil, wrapf("Service.ListKeys", err)
	}
	s.recordAudit(ctx, tenantID, AuditActionViewMetadata, "", AuditOutcomeSuccess, "", "")
	return list, nil
}

// AuditHistory returns tenantID's key-access audit trail (task 7:
// "queryable"), gated on viewPermission. Unlike the other Service
// methods, this call is not itself separately audited — reading the
// audit trail is not a key-access event.
func (s *Service) AuditHistory(ctx context.Context, tenantID uuid.UUID, limit int) ([]*AuditEntry, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	return s.audit.repo.ListForTenant(ctx, tenantID, limit)
}
