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

func TestNewMigrator_ValidSourceInvalidHost(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeMigrationFile(t, dir, "000001_init.up.sql", "SELECT 1;")
	writeMigrationFile(t, dir, "000001_init.down.sql", "SELECT 1;")

	m, err := NewMigrator(os.DirFS(dir), ".", "postgres://user:pass@127.0.0.1:1/verdex?sslmode=disable&connect_timeout=1")
	if err != nil {
		// database/sql.Open with pgx is lazy and typically does not
		// dial until first use, but if the driver validates eagerly
		// that's an acceptable outcome for this constructor-only test.
		return
	}
	t.Cleanup(func() { _ = m.Close() })
}

func writeMigrationFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(dir+"/"+name, []byte(contents), 0o600); err != nil {
		t.Fatalf("write migration fixture %s: %v", name, err)
	}
}
