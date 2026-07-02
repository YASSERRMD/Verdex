package ontology

import "sync"

// AliasRegistry maps concept synonyms/aliases (e.g. "carelessness" as a
// synonym for "negligence") to the canonical Concept.ID they resolve to.
// This mirrors packages/evidence/store.go's mutex-guarded in-memory map
// pattern.
type AliasRegistry struct {
	mu      sync.RWMutex
	aliases map[string]string // alias -> conceptID
}

// NewAliasRegistry constructs an empty AliasRegistry.
func NewAliasRegistry() *AliasRegistry {
	return &AliasRegistry{aliases: make(map[string]string)}
}

// AddAlias registers alias as a synonym resolving to conceptID. Returns
// ErrEmptyInput if conceptID or alias is empty. Returns ErrDuplicateAlias
// if alias is already registered to a different conceptID (re-registering
// the same alias to the same conceptID is a no-op and does not error).
func (r *AliasRegistry) AddAlias(conceptID, alias string) error {
	if conceptID == "" || alias == "" {
		return ErrEmptyInput
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.aliases == nil {
		r.aliases = make(map[string]string)
	}
	if existing, ok := r.aliases[alias]; ok && existing != conceptID {
		return ErrDuplicateAlias
	}
	r.aliases[alias] = conceptID
	return nil
}

// ResolveAlias returns the Concept.ID that alias resolves to. ok is false
// if alias has not been registered via AddAlias.
func (r *AliasRegistry) ResolveAlias(alias string) (conceptID string, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	conceptID, ok = r.aliases[alias]
	return conceptID, ok
}

// Aliases returns every alias registered for conceptID, in no particular
// order.
func (r *AliasRegistry) Aliases(conceptID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for alias, id := range r.aliases {
		if id == conceptID {
			out = append(out, alias)
		}
	}
	return out
}

// RemoveAlias removes alias from the registry, if present. It is a no-op
// if alias was never registered.
func (r *AliasRegistry) RemoveAlias(alias string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.aliases, alias)
}
