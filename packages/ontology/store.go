package ontology

import "sync"

// OntologyStore defines the persistence contract for the ontology layer:
// concepts, relations, jurisdiction overlays, concept-to-node links, the
// alias registry, and version history. This mirrors
// packages/evidence/store.go's ClassificationStore pattern: a small,
// storage-agnostic contract that a relational, document, or (as
// implemented here) in-memory backend can satisfy.
//
// OntologyStore deliberately does not go through packages/graph's
// GraphStore: a Concept is not one of the five fixed IRAC node types
// (Issue/Rule/Fact/Application/Conclusion, fixed in Phase 031), so this
// package owns its own store rather than extending the IRAC schema.
type OntologyStore interface {
	// SaveConcept persists concept, keyed by concept.ID, replacing any
	// existing record. Returns ErrEmptyInput if concept.ID is empty.
	SaveConcept(concept Concept) error

	// GetConcept retrieves the Concept stored for id. Returns
	// ErrConceptNotFound if no record exists.
	GetConcept(id string) (Concept, error)

	// ListConcepts returns every stored Concept, in no particular order.
	ListConcepts() []Concept

	// SaveRelation persists rel. Relations are append-only (no ID-keyed
	// replace semantics); duplicates are allowed since a Concept pair may
	// legitimately carry more than one RelationType.
	SaveRelation(rel Relation) error

	// ListRelations returns every stored Relation, in no particular
	// order.
	ListRelations() []Relation

	// SaveOverlay persists overlay, keyed by overlay.JurisdictionCode,
	// replacing any existing overlay for that jurisdiction.
	SaveOverlay(overlay JurisdictionOverlay) error

	// GetOverlay retrieves the JurisdictionOverlay stored for
	// jurisdictionCode. Returns ErrJurisdictionNotFound if none exists.
	GetOverlay(jurisdictionCode string) (JurisdictionOverlay, error)

	// SaveLink persists link. Links are append-only, mirroring
	// SaveRelation: a Concept may legitimately be linked to the same node
	// more than once (e.g. re-linked at a different confidence).
	SaveLink(link ConceptLink) error

	// ListLinks returns every stored ConceptLink, in no particular order.
	ListLinks() []ConceptLink

	// LinksForConcept returns every stored ConceptLink whose ConceptID
	// matches conceptID, in no particular order.
	LinksForConcept(conceptID string) []ConceptLink

	// Aliases exposes the store's AliasRegistry.
	Aliases() *AliasRegistry

	// SaveVersion persists version, keyed by version.VersionNumber.
	SaveVersion(version OntologyVersion) error

	// GetVersion retrieves the OntologyVersion stored for versionNumber.
	// Returns ErrVersionNotFound if none exists.
	GetVersion(versionNumber int) (OntologyVersion, error)

	// LatestVersion returns the OntologyVersion with the highest
	// VersionNumber. Returns ErrVersionNotFound if no version has been
	// saved yet.
	LatestVersion() (OntologyVersion, error)
}

// InMemoryOntologyStore is the default OntologyStore implementation: a
// mutex-guarded in-memory struct with no real database dependency,
// suitable for tests and for wiring OntologyService end-to-end before a
// durable backend is introduced in a later phase.
type InMemoryOntologyStore struct {
	mu sync.RWMutex

	concepts  map[string]Concept
	relations []Relation
	overlays  map[string]JurisdictionOverlay
	links     []ConceptLink
	aliases   *AliasRegistry
	versions  map[int]OntologyVersion
}

// NewInMemoryOntologyStore constructs an empty InMemoryOntologyStore.
func NewInMemoryOntologyStore() *InMemoryOntologyStore {
	return &InMemoryOntologyStore{
		concepts: make(map[string]Concept),
		overlays: make(map[string]JurisdictionOverlay),
		aliases:  NewAliasRegistry(),
		versions: make(map[int]OntologyVersion),
	}
}

// SaveConcept implements OntologyStore.
func (s *InMemoryOntologyStore) SaveConcept(concept Concept) error {
	if concept.ID == "" {
		return ErrEmptyInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.concepts == nil {
		s.concepts = make(map[string]Concept)
	}
	s.concepts[concept.ID] = concept
	return nil
}

// GetConcept implements OntologyStore.
func (s *InMemoryOntologyStore) GetConcept(id string) (Concept, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.concepts[id]
	if !ok {
		return Concept{}, ErrConceptNotFound
	}
	return c, nil
}

// ListConcepts implements OntologyStore.
func (s *InMemoryOntologyStore) ListConcepts() []Concept {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Concept, 0, len(s.concepts))
	for _, c := range s.concepts {
		out = append(out, c)
	}
	return out
}

// SaveRelation implements OntologyStore.
func (s *InMemoryOntologyStore) SaveRelation(rel Relation) error {
	if rel.FromConceptID == "" || rel.ToConceptID == "" {
		return ErrEmptyInput
	}
	if !rel.Type.IsValid() {
		return ErrInvalidRelationType
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.relations = append(s.relations, rel)
	return nil
}

// ListRelations implements OntologyStore.
func (s *InMemoryOntologyStore) ListRelations() []Relation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Relation, len(s.relations))
	copy(out, s.relations)
	return out
}

// SaveOverlay implements OntologyStore.
func (s *InMemoryOntologyStore) SaveOverlay(overlay JurisdictionOverlay) error {
	if overlay.JurisdictionCode == "" {
		return ErrEmptyInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.overlays == nil {
		s.overlays = make(map[string]JurisdictionOverlay)
	}
	s.overlays[overlay.JurisdictionCode] = overlay
	return nil
}

// GetOverlay implements OntologyStore.
func (s *InMemoryOntologyStore) GetOverlay(jurisdictionCode string) (JurisdictionOverlay, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	o, ok := s.overlays[jurisdictionCode]
	if !ok {
		return JurisdictionOverlay{}, ErrJurisdictionNotFound
	}
	return o, nil
}

// SaveLink implements OntologyStore.
func (s *InMemoryOntologyStore) SaveLink(link ConceptLink) error {
	if link.ConceptID == "" || link.NodeID == "" {
		return ErrEmptyInput
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.links = append(s.links, link)
	return nil
}

// ListLinks implements OntologyStore.
func (s *InMemoryOntologyStore) ListLinks() []ConceptLink {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ConceptLink, len(s.links))
	copy(out, s.links)
	return out
}

// LinksForConcept implements OntologyStore.
func (s *InMemoryOntologyStore) LinksForConcept(conceptID string) []ConceptLink {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ConceptLink
	for _, l := range s.links {
		if l.ConceptID == conceptID {
			out = append(out, l)
		}
	}
	return out
}

// Aliases implements OntologyStore.
func (s *InMemoryOntologyStore) Aliases() *AliasRegistry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.aliases == nil {
		s.aliases = NewAliasRegistry()
	}
	return s.aliases
}

// SaveVersion implements OntologyStore.
func (s *InMemoryOntologyStore) SaveVersion(version OntologyVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.versions == nil {
		s.versions = make(map[int]OntologyVersion)
	}
	s.versions[version.VersionNumber] = version
	return nil
}

// GetVersion implements OntologyStore.
func (s *InMemoryOntologyStore) GetVersion(versionNumber int) (OntologyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.versions[versionNumber]
	if !ok {
		return OntologyVersion{}, ErrVersionNotFound
	}
	return v, nil
}

// LatestVersion implements OntologyStore.
func (s *InMemoryOntologyStore) LatestVersion() (OntologyVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var latest OntologyVersion
	found := false
	for _, v := range s.versions {
		if !found || v.VersionNumber > latest.VersionNumber {
			latest = v
			found = true
		}
	}
	if !found {
		return OntologyVersion{}, ErrVersionNotFound
	}
	return latest, nil
}
