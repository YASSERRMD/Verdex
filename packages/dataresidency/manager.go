package dataresidency

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// PolicyStore persists ResidencyPolicy and RegionPin values keyed by
// deployment ID. Manager accepts it as an interface so callers can
// back it with an in-memory map (tests, small deployments) or a
// repository composed over packages/persistence, mirroring how
// packages/keymanagement separates its Repository interface from any
// specific backing store.
type PolicyStore interface {
	PolicySource

	// SetPolicy persists policy, keyed by policy.DeploymentID.
	SetPolicy(ctx context.Context, policy ResidencyPolicy) error

	// SetRegionPin persists pin, keyed by pin.DeploymentID.
	SetRegionPin(ctx context.Context, pin RegionPin) error
}

// Manager is the access-controlled entry point for defining a
// deployment's ResidencyPolicy and RegionPin (task 1: "residency
// policy per deployment", task 2: "pin storage region/locality").
// Every mutating method requires the authenticated actor (via
// identity.UserFromContext) to hold managePermission, matching this
// repository's access-control convention (see access.go).
type Manager struct {
	store PolicyStore
}

// NewManager builds a Manager backed by store. Returns ErrNilStore if
// store is nil.
func NewManager(store PolicyStore) (*Manager, error) {
	if store == nil {
		return nil, ErrNilStore
	}
	return &Manager{store: store}, nil
}

// SetPolicy validates and persists policy after checking the
// authenticated actor holds managePermission.
func (m *Manager) SetPolicy(ctx context.Context, policy ResidencyPolicy) error {
	if err := authorizeManage(ctx); err != nil {
		return err
	}
	if err := policy.Validate(); err != nil {
		return wrapf("Manager.SetPolicy", err)
	}
	return m.store.SetPolicy(ctx, policy)
}

// SetRegionPin validates and persists pin after checking the
// authenticated actor holds managePermission.
func (m *Manager) SetRegionPin(ctx context.Context, pin RegionPin) error {
	if err := authorizeManage(ctx); err != nil {
		return err
	}
	if err := pin.Validate(); err != nil {
		return wrapf("Manager.SetRegionPin", err)
	}
	return m.store.SetRegionPin(ctx, pin)
}

// Policy returns the ResidencyPolicy for deploymentID. Reading a
// policy is not gated on managePermission -- callers throughout this
// package (CheckTransfer, Verify, ...) need to consult it without
// requiring settings-management rights, matching how
// packages/keymanagement's viewPermission is separate from
// managePermission for a comparable read/write split.
func (m *Manager) Policy(ctx context.Context, deploymentID uuid.UUID) (*ResidencyPolicy, error) {
	return m.store.Policy(ctx, deploymentID)
}

// RegionPin returns the RegionPin for deploymentID.
func (m *Manager) RegionPin(ctx context.Context, deploymentID uuid.UUID) (*RegionPin, error) {
	return m.store.RegionPin(ctx, deploymentID)
}

// InMemoryPolicyStore is a simple, thread-safe, in-process PolicyStore
// implementation, suitable for tests and small/single-node
// deployments. A production deployment is expected to back Manager
// with a repository composed over packages/persistence instead,
// mirroring packages/keymanagement's PostgresRepository.
type InMemoryPolicyStore struct {
	mu       sync.RWMutex
	policies map[uuid.UUID]ResidencyPolicy
	pins     map[uuid.UUID]RegionPin
}

// NewInMemoryPolicyStore builds an empty InMemoryPolicyStore.
func NewInMemoryPolicyStore() *InMemoryPolicyStore {
	return &InMemoryPolicyStore{
		policies: make(map[uuid.UUID]ResidencyPolicy),
		pins:     make(map[uuid.UUID]RegionPin),
	}
}

// SetPolicy implements PolicyStore.
func (s *InMemoryPolicyStore) SetPolicy(_ context.Context, policy ResidencyPolicy) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[policy.DeploymentID] = policy
	return nil
}

// SetRegionPin implements PolicyStore.
func (s *InMemoryPolicyStore) SetRegionPin(_ context.Context, pin RegionPin) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pins[pin.DeploymentID] = pin
	return nil
}

// Policy implements PolicySource.
func (s *InMemoryPolicyStore) Policy(_ context.Context, deploymentID uuid.UUID) (*ResidencyPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.policies[deploymentID]
	if !ok {
		return nil, wrapf("Policy", ErrEmptyDeploymentID)
	}
	return &p, nil
}

// RegionPin implements PolicySource.
func (s *InMemoryPolicyStore) RegionPin(_ context.Context, deploymentID uuid.UUID) (*RegionPin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.pins[deploymentID]
	if !ok {
		return nil, wrapf("RegionPin", ErrEmptyDeploymentID)
	}
	return &p, nil
}
