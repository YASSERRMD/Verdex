package persistence

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Executor is the subset of pgxpool.Pool and pgx.Tx that repositories
// depend on. Repository methods accept an Executor rather than a
// concrete *pgxpool.Pool so the same method body can run directly
// against the pool or inside a transaction started by WithTx, without
// any repository-level branching.
type Executor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
