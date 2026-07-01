package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver

	"github.com/YASSERRMD/verdex/packages/persistence/migrations"
)

// Migrator runs schema migrations against PostgreSQL. It wraps
// golang-migrate/migrate with a context-aware API and a source
// rooted at an fs.FS (in production, an embed.FS bundled into the
// compiled binary; in tests, any fs.FS such as os.DirFS).
type Migrator struct {
	migrate *migrate.Migrate
	sqlDB   *sql.DB
}

// NewMigrator builds a Migrator that reads migration files from dir
// within migrations (e.g. "." if migrations is already rooted at the
// migration files) and applies them against dsn using the pgx
// database/sql driver.
//
// The returned Migrator owns its own *sql.DB, independent of any
// Postgres/pgxpool.Pool the caller may also have open; call Close
// when done with it.
func NewMigrator(migrations fs.FS, dir, dsn string) (*Migrator, error) {
	if dsn == "" {
		return nil, fmt.Errorf("persistence: NewMigrator: dsn must not be empty")
	}

	sourceDriver, err := iofs.New(migrations, dir)
	if err != nil {
		return nil, fmt.Errorf("persistence: NewMigrator: source driver: %w", err)
	}

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("persistence: NewMigrator: open database/sql: %w", err)
	}

	dbDriver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("persistence: NewMigrator: database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx", dbDriver)
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("persistence: NewMigrator: %w", err)
	}

	return &Migrator{migrate: m, sqlDB: sqlDB}, nil
}

// NewEmbeddedMigrator builds a Migrator using the SQL files embedded
// in packages/persistence/migrations at compile time. This is the
// constructor production services should use; NewMigrator remains
// available for tests and tools that need to point at an alternate
// migrations source.
func NewEmbeddedMigrator(dsn string) (*Migrator, error) {
	return NewMigrator(migrations.FS, ".", dsn)
}

// Close releases the Migrator's own database connection. It does not
// affect any other pool or connection the caller holds.
func (m *Migrator) Close() error {
	if m == nil || m.sqlDB == nil {
		return nil
	}
	return m.sqlDB.Close()
}

// Up applies all available up migrations that have not yet been
// applied. It returns nil (not an error) if the schema is already at
// the latest version. ctx is checked for cancellation before the
// (synchronous, non-cancellable) underlying migration run starts.
func (m *Migrator) Up(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.migrate.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("persistence: Up: %w", err)
	}
	return nil
}

// Down reverts all applied migrations. It returns nil (not an error)
// if there is nothing to revert.
func (m *Migrator) Down(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.migrate.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("persistence: Down: %w", err)
	}
	return nil
}

// Steps applies n migrations if n is positive, or reverts |n|
// migrations if n is negative. It returns nil (not an error) if there
// is nothing left to do in the requested direction.
func (m *Migrator) Steps(ctx context.Context, n int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.migrate.Steps(n); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("persistence: Steps: %w", err)
	}
	return nil
}

// Version reports the currently applied migration version and
// whether the schema is in a "dirty" state (a previous migration
// failed partway through and needs operator attention; see Force).
// It returns migrate.ErrNilVersion if no migration has been applied
// yet.
func (m *Migrator) Version() (version uint, dirty bool, err error) {
	return m.migrate.Version()
}

// Force sets the migration version without running any migration,
// and clears the dirty flag. This is the documented recovery path
// when golang-migrate leaves the schema_migrations table marked dirty
// after a failed migration: inspect the database by hand, fix it up
// to match a known-good version, then Force that version so future
// Up/Down/Steps calls proceed normally again.
func (m *Migrator) Force(version int) error {
	if err := m.migrate.Force(version); err != nil {
		return fmt.Errorf("persistence: Force: %w", err)
	}
	return nil
}
