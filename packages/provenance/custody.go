package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EventType constants for CustodyEvent.
const (
	EventUploaded  = "uploaded"
	EventHashed    = "hashed"
	EventScanned   = "scanned"
	EventDiscarded = "discarded"
	EventExtracted = "extracted"
	EventVerified  = "verified"
)

// CustodyEvent is a single step in the chain of custody for an artifact.
// Events are linked by PrevEventHash so the entire history is tamper-evident.
type CustodyEvent struct {
	// ID is the unique identifier of this event.
	ID uuid.UUID `json:"id"`

	// ProvenanceID is the ID of the ProvenanceRecord this event belongs to.
	ProvenanceID uuid.UUID `json:"provenance_id"`

	// EventType describes the action taken (see Event* constants).
	EventType string `json:"event_type"`

	// Actor is the identity (user ID, service name, etc.) that performed the action.
	Actor string `json:"actor"`

	// Details carries optional free-text context about the event.
	Details string `json:"details"`

	// Timestamp is the UTC time the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// PrevEventHash is the EventHash of the immediately preceding event, or the
	// empty string for the first event.
	PrevEventHash string `json:"prev_event_hash"`

	// EventHash is SHA-256(prevEventHash + eventType + actor + timestamp) in hex.
	EventHash string `json:"event_hash"`
}

// computeEventHash derives the event hash from its inputs.
func computeEventHash(prevHash, eventType, actor string, ts time.Time) string {
	input := prevHash + eventType + actor + ts.UTC().Format("2006-01-02T15:04:05.999999999Z")
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}

// CustodyChain is an ordered, append-only sequence of CustodyEvents for a
// single ProvenanceRecord. It links each event to the previous one so that any
// gap or modification is detectable.
type CustodyChain struct {
	mu     sync.RWMutex
	events []CustodyEvent
}

// NewCustodyChain creates an empty chain.
func NewCustodyChain() *CustodyChain {
	return &CustodyChain{}
}

// AddEvent appends e to the chain. It sets e.PrevEventHash to the hash of the
// last event (or "" for the first) and computes e.EventHash before storing.
// Returns ErrChainBroken if the event's ID is zero.
func (c *CustodyChain) AddEvent(e CustodyEvent) error {
	if e.ID == uuid.Nil {
		return fmt.Errorf("provenance: custody event has no ID")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	prevHash := ""
	if len(c.events) > 0 {
		prevHash = c.events[len(c.events)-1].EventHash
	}

	e.PrevEventHash = prevHash
	e.EventHash = computeEventHash(prevHash, e.EventType, e.Actor, e.Timestamp)
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
		e.EventHash = computeEventHash(prevHash, e.EventType, e.Actor, e.Timestamp)
	}

	c.events = append(c.events, e)
	return nil
}

// GetChain returns a copy of the event slice in insertion order.
func (c *CustodyChain) GetChain() []CustodyEvent {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]CustodyEvent, len(c.events))
	copy(out, c.events)
	return out
}

// VerifyChain recomputes each event hash and compares it to the stored value.
// Returns (true, -1, nil) when the chain is intact, or (false, i, err) at the
// first broken link at index i.
func (c *CustodyChain) VerifyChain() (valid bool, brokenAt int, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.events) == 0 {
		return true, -1, nil
	}

	prevHash := ""
	for i, e := range c.events {
		expected := computeEventHash(prevHash, e.EventType, e.Actor, e.Timestamp)
		if e.EventHash != expected {
			return false, i, fmt.Errorf("%w: custody event at index %d (id=%s)", ErrChainBroken, i, e.ID)
		}
		prevHash = e.EventHash
	}
	return true, -1, nil
}
