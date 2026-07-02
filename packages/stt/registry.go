package stt

import (
	"fmt"
	"sort"
	"sync"
)

// Registry is a thread-safe map from provider IDs to STTProvider instances.
//
// Use DefaultRegistry for process-wide provider registration, or create an
// isolated Registry for testing. This mirrors provider.Registry in
// packages/provider.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]STTProvider
}

// NewRegistry returns an empty, initialised Registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]STTProvider),
	}
}

// Register adds p to the registry under id.
//
// It returns an error if id is empty, p is nil, or a provider with the same
// id is already registered.
func (r *Registry) Register(id string, p STTProvider) error {
	if id == "" {
		return fmt.Errorf("stt: %w: id must not be empty", ErrInvalidRequest)
	}
	if p == nil {
		return fmt.Errorf("stt: %w: provider must not be nil", ErrInvalidRequest)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[id]; exists {
		return fmt.Errorf("stt: %w: provider %q is already registered", ErrInvalidRequest, id)
	}

	r.providers[id] = p
	return nil
}

// Get retrieves the STTProvider registered under id.
//
// It returns ErrProviderNotFound (wrapped) when no provider with that id
// exists.
func (r *Registry) Get(id string) (STTProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("stt: %w: %q", ErrProviderNotFound, id)
	}
	return p, nil
}

// List returns the IDs of all registered providers in lexicographic order.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// MustGet returns the STTProvider registered under id.
//
// It panics if no provider with that id exists. Use this only during program
// initialisation where a missing provider is a programming error.
func (r *Registry) MustGet(id string) STTProvider {
	p, err := r.Get(id)
	if err != nil {
		panic(err)
	}
	return p
}

// DefaultRegistry is the process-wide STT provider registry.
//
// All packages that need an STTProvider should call DefaultRegistry.Get() or
// DefaultRegistry.MustGet() after their adapter has been registered during
// application startup.
var DefaultRegistry = NewRegistry()
