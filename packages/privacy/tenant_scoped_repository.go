package privacy

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// TenantScopedInventoryRepository composes
// packages/tenancy.WithTenantScope with PostgresInventoryRepository,
// exactly as packages/accessgovernance.TenantScopedPolicyRepository
// does: every method opens its own RLS-scoped transaction, so
// Row-Level Security enforces tenant isolation at the database layer
// in addition to PostgresInventoryRepository's own application-level
// requireMatchingTenant guard.
type TenantScopedInventoryRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedInventoryRepository builds a
// TenantScopedInventoryRepository backed by pool.
func NewTenantScopedInventoryRepository(pool *pgxpool.Pool) *TenantScopedInventoryRepository {
	return &TenantScopedInventoryRepository{pool: pool}
}

func (r *TenantScopedInventoryRepository) Create(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresInventoryRepository(exec).Create(ctx, tenantID, e)
	})
}

func (r *TenantScopedInventoryRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*DataInventoryEntry, error) {
	var out *DataInventoryEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		e, err := NewPostgresInventoryRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedInventoryRepository) List(ctx context.Context, tenantID uuid.UUID) ([]DataInventoryEntry, error) {
	var out []DataInventoryEntry
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresInventoryRepository(exec).List(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedInventoryRepository) Update(ctx context.Context, tenantID uuid.UUID, e *DataInventoryEntry) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresInventoryRepository(exec).Update(ctx, tenantID, e)
	})
}

var _ InventoryRepository = (*TenantScopedInventoryRepository)(nil)

// TenantScopedConsentRepository composes
// packages/tenancy.WithTenantScope with PostgresConsentRepository.
type TenantScopedConsentRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedConsentRepository builds a
// TenantScopedConsentRepository backed by pool.
func NewTenantScopedConsentRepository(pool *pgxpool.Pool) *TenantScopedConsentRepository {
	return &TenantScopedConsentRepository{pool: pool}
}

func (r *TenantScopedConsentRepository) Create(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresConsentRepository(exec).Create(ctx, tenantID, c)
	})
}

func (r *TenantScopedConsentRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ConsentRecord, error) {
	var out *ConsentRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		c, err := NewPostgresConsentRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = c
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedConsentRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ConsentRecord, error) {
	var out []ConsentRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresConsentRepository(exec).ListForSubject(ctx, tenantID, subjectID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedConsentRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ConsentRecord, error) {
	var out []ConsentRecord
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresConsentRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedConsentRepository) Update(ctx context.Context, tenantID uuid.UUID, c *ConsentRecord) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresConsentRepository(exec).Update(ctx, tenantID, c)
	})
}

var _ ConsentRepository = (*TenantScopedConsentRepository)(nil)

// TenantScopedSARRepository composes
// packages/tenancy.WithTenantScope with PostgresSARRepository.
type TenantScopedSARRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedSARRepository builds a TenantScopedSARRepository
// backed by pool.
func NewTenantScopedSARRepository(pool *pgxpool.Pool) *TenantScopedSARRepository {
	return &TenantScopedSARRepository{pool: pool}
}

func (r *TenantScopedSARRepository) Create(ctx context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresSARRepository(exec).Create(ctx, tenantID, s)
	})
}

func (r *TenantScopedSARRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*SubjectAccessRequest, error) {
	var out *SubjectAccessRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		s, err := NewPostgresSARRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = s
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedSARRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]SubjectAccessRequest, error) {
	var out []SubjectAccessRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresSARRepository(exec).ListForSubject(ctx, tenantID, subjectID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedSARRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]SubjectAccessRequest, error) {
	var out []SubjectAccessRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresSARRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedSARRepository) Update(ctx context.Context, tenantID uuid.UUID, s *SubjectAccessRequest) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresSARRepository(exec).Update(ctx, tenantID, s)
	})
}

var _ SARRepository = (*TenantScopedSARRepository)(nil)

// TenantScopedErasureRepository composes
// packages/tenancy.WithTenantScope with PostgresErasureRepository.
type TenantScopedErasureRepository struct {
	pool *pgxpool.Pool
}

// NewTenantScopedErasureRepository builds a
// TenantScopedErasureRepository backed by pool.
func NewTenantScopedErasureRepository(pool *pgxpool.Pool) *TenantScopedErasureRepository {
	return &TenantScopedErasureRepository{pool: pool}
}

func (r *TenantScopedErasureRepository) Create(ctx context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresErasureRepository(exec).Create(ctx, tenantID, req)
	})
}

func (r *TenantScopedErasureRepository) Get(ctx context.Context, tenantID, id uuid.UUID) (*ErasureRequest, error) {
	var out *ErasureRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		req, err := NewPostgresErasureRepository(exec).Get(ctx, tenantID, id)
		if err != nil {
			return err
		}
		out = req
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedErasureRepository) ListForSubject(ctx context.Context, tenantID uuid.UUID, subjectID string) ([]ErasureRequest, error) {
	var out []ErasureRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresErasureRepository(exec).ListForSubject(ctx, tenantID, subjectID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedErasureRepository) ListAll(ctx context.Context, tenantID uuid.UUID) ([]ErasureRequest, error) {
	var out []ErasureRequest
	err := tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		list, err := NewPostgresErasureRepository(exec).ListAll(ctx, tenantID)
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *TenantScopedErasureRepository) Update(ctx context.Context, tenantID uuid.UUID, req *ErasureRequest) error {
	return tenancy.WithTenantScope(ctx, r.pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
		return NewPostgresErasureRepository(exec).Update(ctx, tenantID, req)
	})
}

var _ ErasureRepository = (*TenantScopedErasureRepository)(nil)
