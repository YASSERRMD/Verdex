package graph

import (
	"context"
	"fmt"
)

// WithTransaction invokes fn with store, giving fn the illusion of an
// atomic sequence of GraphStore writes: if fn returns an error (or
// panics), every write fn made through store during the call is rolled
// back before WithTransaction returns.
//
// The in-memory implementation (used when store is an
// *InMemoryGraphStore) uses a snapshot-and-rollback-on-error strategy:
// it deep-copies the store's internal state before invoking fn, and
// restores that snapshot if fn fails, rather than tracking individual
// writes to undo. This is sufficient for a single-process in-memory
// store and keeps the rollback path simple and obviously correct.
//
// A Neo4j-backed GraphStore should NOT reuse this snapshot strategy;
// document (and implement, when that store lands) real Neo4j
// sessions/transactions via the session-capable driver introduced in
// migrate.go: begin an explicit transaction with
// session.BeginTransaction, run every write against the transaction
// handle, and Commit/Rollback it directly, so atomicity is enforced by
// the database itself rather than by an in-process snapshot.
func WithTransaction(ctx context.Context, store GraphStore, fn func(GraphStore) error) (err error) {
	if store == nil {
		return fmt.Errorf("graph: WithTransaction: store must not be nil")
	}
	if fn == nil {
		return fmt.Errorf("graph: WithTransaction: fn must not be nil")
	}

	mem, ok := store.(*InMemoryGraphStore)
	if !ok {
		// Non-in-memory stores are expected to implement their own
		// transactional semantics (e.g. a Neo4j-backed store wrapping
		// session.ExecuteWrite); until such a store exists in this
		// package, run fn directly against it with no rollback support
		// rather than silently pretending to snapshot state we cannot
		// see inside.
		return fn(store)
	}

	snapshot := mem.snapshot()

	defer func() {
		if p := recover(); p != nil {
			mem.restore(snapshot)
			panic(p)
		}
	}()

	if err = fn(mem); err != nil {
		mem.restore(snapshot)
		return fmt.Errorf("%w: %w", ErrTransactionFailed, err)
	}
	return nil
}
