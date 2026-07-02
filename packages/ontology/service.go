package ontology

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// OntologyService orchestrates the full ontology pipeline:
//
//	seed core concepts -> apply jurisdiction overlay -> register
//	  aliases/labels -> link concepts to nodes -> version -> persist
//	  -> expose query methods
//
// This mirrors packages/application's ApplicationService and
// packages/issue/fact's *Service orchestration pattern: a single entry
// point wiring together this package's otherwise independent,
// individually testable building blocks (SeedCoreConcepts, MergeOverlay,
// AliasRegistry, LinkConcept, NewInitialVersion/NextVersion).
type OntologyService struct {
	// Store persists concepts, relations, overlays, links, aliases, and
	// version history. If nil, a fresh InMemoryOntologyStore is used.
	Store OntologyStore
}

// NewOntologyService constructs an OntologyService with a fresh
// InMemoryOntologyStore.
func NewOntologyService() *OntologyService {
	return &OntologyService{Store: NewInMemoryOntologyStore()}
}

func (s *OntologyService) store() OntologyStore {
	if s.Store == nil {
		s.Store = NewInMemoryOntologyStore()
	}
	return s.Store
}

// BootstrapRequest carries the input to OntologyService.Bootstrap.
type BootstrapRequest struct {
	// Taxonomy supplies the category codes SeedCoreConcepts seeds
	// concepts for. Required.
	Taxonomy category.Taxonomy

	// Overlay optionally supplies a JurisdictionOverlay merged on top of
	// the core seed set. A zero-value overlay (empty JurisdictionCode) is
	// treated as "no overlay" and skipped.
	Overlay JurisdictionOverlay

	// CreatedAt stamps the initial OntologyVersion. If zero, time.Now()
	// is used.
	CreatedAt time.Time
}

// Bootstrap runs the seed -> overlay -> persist -> version pipeline: it
// seeds core concepts from req.Taxonomy, merges req.Overlay on top (if
// any), persists every resulting Concept, and records the initial
// OntologyVersion (or the next version, if the store already has one).
// Returns the final concept set and the OntologyVersion recorded for
// this bootstrap.
func (s *OntologyService) Bootstrap(req BootstrapRequest) ([]Concept, OntologyVersion, error) {
	if req.Taxonomy == nil {
		return nil, OntologyVersion{}, ErrEmptyInput
	}

	core := SeedCoreConcepts(req.Taxonomy)
	concepts := core
	if req.Overlay.JurisdictionCode != "" {
		concepts = MergeOverlay(core, req.Overlay)
		if err := s.store().SaveOverlay(req.Overlay); err != nil {
			return nil, OntologyVersion{}, err
		}
	}

	store := s.store()
	for _, c := range concepts {
		if err := store.SaveConcept(c); err != nil {
			return nil, OntologyVersion{}, err
		}
	}

	createdAt := req.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	version, err := s.recordNextVersion(createdAt)
	if err != nil {
		return nil, OntologyVersion{}, err
	}

	return concepts, version, nil
}

// recordNextVersion saves and returns the next OntologyVersion in the
// store's sequence: the initial version if none exists yet, or the
// successor of the current latest version.
func (s *OntologyService) recordNextVersion(createdAt time.Time) (OntologyVersion, error) {
	store := s.store()
	latest, err := store.LatestVersion()
	var next OntologyVersion
	if err != nil {
		next = NewInitialVersion(createdAt)
	} else {
		next = NextVersion(latest, createdAt)
	}
	if err := store.SaveVersion(next); err != nil {
		return OntologyVersion{}, err
	}
	return next, nil
}

// RegisterAlias registers alias as a synonym for conceptID, verifying
// conceptID refers to a concept already present in the store. Returns
// ErrConceptNotFound if conceptID is unknown, or ErrDuplicateAlias if
// alias already resolves to a different concept.
func (s *OntologyService) RegisterAlias(conceptID, alias string) error {
	if _, err := s.store().GetConcept(conceptID); err != nil {
		return err
	}
	return s.store().Aliases().AddAlias(conceptID, alias)
}

// RegisterLabel sets concept c's label for languageCode and persists the
// updated Concept.
func (s *OntologyService) RegisterLabel(conceptID, languageCode, label string) (Concept, error) {
	c, err := s.store().GetConcept(conceptID)
	if err != nil {
		return Concept{}, err
	}
	c = c.SetLabel(languageCode, label)
	if err := s.store().SaveConcept(c); err != nil {
		return Concept{}, err
	}
	return c, nil
}

// LinkConceptToNode links conceptID to node at the given confidence,
// verifying conceptID refers to a concept already present in the store,
// and persists the resulting ConceptLink.
func (s *OntologyService) LinkConceptToNode(conceptID string, node irac.NodeLike, confidence float64) (ConceptLink, error) {
	c, err := s.store().GetConcept(conceptID)
	if err != nil {
		return ConceptLink{}, err
	}
	link := LinkConcept(c, node, confidence)
	if err := s.store().SaveLink(link); err != nil {
		return ConceptLink{}, err
	}
	return link, nil
}

// ResolveConcept resolves nameOrAlias to a Concept: first by treating it
// as a Concept.ID directly, then by treating it as a registered alias.
// Returns ErrConceptNotFound if neither resolves.
func (s *OntologyService) ResolveConcept(nameOrAlias string) (Concept, error) {
	if c, err := s.store().GetConcept(nameOrAlias); err == nil {
		return c, nil
	}
	conceptID, ok := s.store().Aliases().ResolveAlias(nameOrAlias)
	if !ok {
		return Concept{}, ErrConceptNotFound
	}
	return s.store().GetConcept(conceptID)
}

// ConceptsByCategory returns every stored Concept associated with
// categoryCode, in no particular order.
func (s *OntologyService) ConceptsByCategory(categoryCode string) []Concept {
	var out []Concept
	for _, c := range s.store().ListConcepts() {
		if c.HasCategory(categoryCode) {
			out = append(out, c)
		}
	}
	return out
}

// LinksForConcept returns every stored ConceptLink whose ConceptID
// matches conceptID.
func (s *OntologyService) LinksForConcept(conceptID string) []ConceptLink {
	return s.store().LinksForConcept(conceptID)
}
