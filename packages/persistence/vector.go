package persistence

import "github.com/pgvector/pgvector-go"

// Vector re-exports pgvector-go's Vector type so callers can depend on
// packages/persistence alone for the pgvector column type, without a
// direct dependency on github.com/pgvector/pgvector-go. Phase 041 owns
// actually storing and querying embedding vectors; this phase only
// pins the driver dependency and confirms it compiles and links
// against the rest of this module.
type Vector = pgvector.Vector

// NewVector constructs a Vector from a slice of float32 embedding
// components. See Vector for storage details.
func NewVector(embedding []float32) Vector {
	return pgvector.NewVector(embedding)
}
