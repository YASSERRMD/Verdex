package prompts

import (
	"fmt"
	"sync"
	"text/template"
	"time"
)

// registryKey uniquely identifies a template slot in the registry.
type registryKey struct {
	id          string
	version     int
	locale      string
	legalFamily string
}

// Registry stores versioned prompt templates and provides lookup operations.
// All methods are safe for concurrent use.
type Registry struct {
	mu        sync.RWMutex
	templates map[registryKey]PromptTemplate
}

// NewRegistry creates and returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		templates: make(map[registryKey]PromptTemplate),
	}
}

// DefaultRegistry is the package-level registry. Template packages under
// packages/prompts/templates register into this registry via init().
var DefaultRegistry = NewRegistry()

// Register validates and stores a PromptTemplate. It returns:
//   - ErrInvalidTemplate if the template fails structural validation.
//   - ErrVersionConflict if the same ID+Version+Locale+LegalFamily is already stored.
func (r *Registry) Register(t PromptTemplate) error {
	if err := validateTemplate(&t); err != nil {
		return err
	}

	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}

	key := registryKey{
		id:          t.ID,
		version:     t.Version,
		locale:      t.Locale,
		legalFamily: t.LegalFamily,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.templates[key]; exists {
		return fmt.Errorf("%w: id=%s version=%d locale=%q legalFamily=%q",
			ErrVersionConflict, t.ID, t.Version, t.Locale, t.LegalFamily)
	}

	r.templates[key] = t
	return nil
}

// Get returns the template with the exact ID, version, locale, and legalFamily
// combination, or ErrTemplateNotFound.
func (r *Registry) Get(id string, version int, locale, legalFamily string) (*PromptTemplate, error) {
	key := registryKey{
		id:          id,
		version:     version,
		locale:      locale,
		legalFamily: legalFamily,
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.templates[key]
	if !ok {
		return nil, fmt.Errorf("%w: id=%s version=%d locale=%q legalFamily=%q",
			ErrTemplateNotFound, id, version, locale, legalFamily)
	}
	return &t, nil
}

// Latest returns the highest-version template for the given ID, locale, and
// legalFamily, or ErrTemplateNotFound.
func (r *Registry) Latest(id, locale, legalFamily string) (*PromptTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *PromptTemplate
	for _, t := range r.templates {
		t := t // capture
		if t.ID != id || t.Locale != locale || t.LegalFamily != legalFamily {
			continue
		}
		if best == nil || t.Version > best.Version {
			best = &t
		}
	}

	if best == nil {
		return nil, fmt.Errorf("%w: id=%s locale=%q legalFamily=%q",
			ErrTemplateNotFound, id, locale, legalFamily)
	}
	return best, nil
}

// List returns a snapshot of all registered templates in undefined order.
func (r *Registry) List() []PromptTemplate {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]PromptTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out
}

// validateTemplate checks structural invariants.
func validateTemplate(t *PromptTemplate) error {
	if t.ID == "" {
		return fmt.Errorf("%w: ID must not be empty", ErrInvalidTemplate)
	}
	if t.Version < 1 {
		return fmt.Errorf("%w: Version must be >= 1 (got %d)", ErrInvalidTemplate, t.Version)
	}
	if t.Body == "" {
		return fmt.Errorf("%w: Body must not be empty", ErrInvalidTemplate)
	}

	// Ensure the body is parseable as a Go text/template.
	if _, err := template.New(t.ID).Parse(t.Body); err != nil {
		return fmt.Errorf("%w: unparseable body: %v", ErrInvalidTemplate, err)
	}

	// Ensure variable names are unique.
	seen := make(map[string]struct{}, len(t.Variables))
	for _, v := range t.Variables {
		if v.Name == "" {
			return fmt.Errorf("%w: VariableSpec has empty Name", ErrInvalidTemplate)
		}
		if _, dup := seen[v.Name]; dup {
			return fmt.Errorf("%w: duplicate VariableSpec name %q", ErrInvalidTemplate, v.Name)
		}
		seen[v.Name] = struct{}{}
	}
	return nil
}
