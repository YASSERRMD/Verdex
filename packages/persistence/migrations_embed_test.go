package persistence

import (
	"testing"

	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/YASSERRMD/verdex/packages/persistence/migrations"
)

// TestEmbeddedMigrations_SourceParses verifies the embedded migration
// files are well-formed and discoverable by golang-migrate's source
// driver, without needing a live database.
func TestEmbeddedMigrations_SourceParses(t *testing.T) {
	t.Parallel()

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("iofs.New: %v", err)
	}
	t.Cleanup(func() { _ = src.Close() })

	first, err := src.First()
	if err != nil {
		t.Fatalf("First: %v", err)
	}
	if first != 1 {
		t.Fatalf("expected first migration version 1, got %d", first)
	}

	second, err := src.Next(first)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if second != 2 {
		t.Fatalf("expected second migration version 2, got %d", second)
	}

	upReader, identifier, err := src.ReadUp(second)
	if err != nil {
		t.Fatalf("ReadUp: %v", err)
	}
	defer func() { _ = upReader.Close() }()
	if identifier == "" {
		t.Fatal("expected non-empty identifier for up migration")
	}

	downReader, _, err := src.ReadDown(second)
	if err != nil {
		t.Fatalf("ReadDown: %v", err)
	}
	_ = downReader.Close()
}

func TestNewEmbeddedMigrator_EmptyDSN(t *testing.T) {
	t.Parallel()

	_, err := NewEmbeddedMigrator("")
	if err == nil {
		t.Fatal("expected error for empty dsn, got nil")
	}
}
