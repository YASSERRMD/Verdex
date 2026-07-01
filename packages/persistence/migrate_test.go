package persistence

import (
	"os"
	"testing"
)

func TestNewMigrator_EmptyDSN(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := NewMigrator(os.DirFS(dir), ".", "")
	if err == nil {
		t.Fatal("expected error for empty dsn, got nil")
	}
}

func TestNewMigrator_InvalidSourceDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := NewMigrator(os.DirFS(dir), "does-not-exist", "postgres://user:pass@localhost:5432/verdex?sslmode=disable")
	if err == nil {
		t.Fatal("expected error for missing migrations directory, got nil")
	}
}

// TestNewMigrator_UnreachableHostFailsFast verifies that pointing a
// Migrator at an unreachable host fails fast at construction time
// (postgres.WithInstance connects eagerly, unlike a bare
// database/sql.Open) bounded by a short connect_timeout, rather than
// hanging - the same fail-fast property RecoverDirty and friends
// depend on when a caller mistakenly points them at a bad DSN.
//
// RecoverDirty's core safety property - refusing to Force when
// Version reports the schema is not dirty - is exercised end-to-end
// against a real dirty schema in the Docker-backed integration suite
// (integration_test.go), since producing a genuinely dirty
// schema_migrations row requires a live database.
func TestNewMigrator_UnreachableHostFailsFast(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeMigrationFile(t, dir, "000001_init.up.sql", "SELECT 1;")
	writeMigrationFile(t, dir, "000001_init.down.sql", "SELECT 1;")

	_, err := NewMigrator(os.DirFS(dir), ".", "postgres://user:pass@127.0.0.1:1/verdex?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error constructing a Migrator against an unreachable host, got nil")
	}
}

func writeMigrationFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(contents), 0o600); err != nil {
		t.Fatalf("write migration fixture %s: %v", name, err)
	}
}
