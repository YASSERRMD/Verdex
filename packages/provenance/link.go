package provenance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ExtractionLink records that a ProvenanceRecord's content has been extracted
// and the resulting text is stored at ExtractedTextRef.
type ExtractionLink struct {
	// ProvenanceID is the ID of the provenance record that was extracted.
	ProvenanceID uuid.UUID `json:"provenance_id"`

	// ExtractedTextRef is an opaque reference to the location of the extracted
	// text (e.g., an object-store key, a database row ID, or a file path).
	ExtractedTextRef string `json:"extracted_text_ref"`

	// LinkedAt is the UTC time at which the link was recorded.
	LinkedAt time.Time `json:"linked_at"`
}

// ExtractionLinkStore is an in-memory registry of ExtractionLink values.
// It is safe for concurrent use.
type ExtractionLinkStore struct {
	mu    sync.RWMutex
	links []ExtractionLink
}

// NewExtractionLinkStore creates an empty store.
func NewExtractionLinkStore() *ExtractionLinkStore {
	return &ExtractionLinkStore{}
}

// LinkToExtraction records that the artifact identified by provenanceID has
// been extracted and the resulting text lives at extractedTextRef.
func (ls *ExtractionLinkStore) LinkToExtraction(_ context.Context, provenanceID uuid.UUID, extractedTextRef string) error {
	if provenanceID == uuid.Nil {
		return fmt.Errorf("provenance: provenanceID must not be nil")
	}
	if extractedTextRef == "" {
		return fmt.Errorf("provenance: extractedTextRef must not be empty")
	}

	link := ExtractionLink{
		ProvenanceID:     provenanceID,
		ExtractedTextRef: extractedTextRef,
		LinkedAt:         time.Now().UTC(),
	}

	ls.mu.Lock()
	defer ls.mu.Unlock()
	ls.links = append(ls.links, link)
	return nil
}

// GetLinksForProvenance returns all ExtractionLink values for the given
// provenanceID.
func (ls *ExtractionLinkStore) GetLinksForProvenance(_ context.Context, provenanceID uuid.UUID) ([]ExtractionLink, error) {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	var out []ExtractionLink
	for _, l := range ls.links {
		if l.ProvenanceID == provenanceID {
			out = append(out, l)
		}
	}
	return out, nil
}
