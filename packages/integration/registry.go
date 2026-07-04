package integration

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is a thread-safe map from connector IDs to Connector
// instances, mirroring packages/provider.Registry's exact shape
// applied to court case-management system connectors instead of LLM
// providers.
//
// Use DefaultRegistry for process-wide connector registration, or
// create an isolated Registry for testing.
type Registry struct {
	mu         sync.RWMutex
	connectors map[string]Connector
}

// NewRegistry returns an empty, initialised Registry.
func NewRegistry() *Registry {
	return &Registry{
		connectors: make(map[string]Connector),
	}
}

// Register adds c to the registry under id.
//
// It returns an error if id is empty, c is nil, or a connector with
// the same id is already registered.
func (r *Registry) Register(id string, c Connector) error {
	if id == "" {
		return wrapf("Registry.Register", ErrInvalidConnectorConfig)
	}
	if c == nil {
		return wrapf("Registry.Register", ErrNilConnector)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.connectors[id]; exists {
		return fmt.Errorf("integration: Registry.Register: %w: %q", ErrDuplicateConnector, id)
	}

	r.connectors[id] = c
	return nil
}

// Get retrieves the Connector registered under id.
//
// It returns ErrConnectorNotFound (wrapped) when no connector with
// that id exists.
func (r *Registry) Get(id string) (Connector, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.connectors[id]
	if !ok {
		return nil, fmt.Errorf("integration: Registry.Get: %w: %q", ErrConnectorNotFound, id)
	}
	return c, nil
}

// List returns the IDs of all registered connectors in lexicographic
// order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.connectors))
	for id := range r.connectors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Unregister removes the connector registered under id, if any. It is
// a no-op (returns nil) if id was not registered -- useful for test
// teardown and for replacing a connector via Unregister then Register.
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.connectors, id)
}

// DefaultRegistry is the process-wide connector registry.
var DefaultRegistry = NewRegistry()
