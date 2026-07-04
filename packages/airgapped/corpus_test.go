package airgapped_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/YASSERRMD/verdex/packages/airgapped"
)

const testStatuteBody = `ACT 12: Contract Act
Section 1. Formation.
A contract requires offer and acceptance.
`

const testPrecedentBody = `CASE [1932] AC 562: Donoghue v Stevenson
COURT: House of Lords
DECIDED: 1932-05-26
HELD: A manufacturer owes a duty of care to the ultimate consumer.
`

func writeBundleFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", name, err)
	}
}

func TestProvisionCorpus_LoadsStatutesAndPrecedents(t *testing.T) {
	dir := t.TempDir()
	writeBundleFile(t, dir, "contracts.statute.txt", testStatuteBody)
	writeBundleFile(t, dir, "donoghue.precedent.txt", testPrecedentBody)

	result, err := airgapped.ProvisionCorpus(context.Background(), dir)
	if err != nil {
		t.Fatalf("ProvisionCorpus: %v", err)
	}
	if len(result.Statutes) != 1 {
		t.Fatalf("len(Statutes) = %d, want 1", len(result.Statutes))
	}
	if result.Statutes[0].ActNumber != "12" {
		t.Errorf("ActNumber = %q, want %q", result.Statutes[0].ActNumber, "12")
	}
	if len(result.Precedents) != 1 {
		t.Fatalf("len(Precedents) = %d, want 1", len(result.Precedents))
	}
	if result.Precedents[0].CaseName != "Donoghue v Stevenson" {
		t.Errorf("CaseName = %q, want %q", result.Precedents[0].CaseName, "Donoghue v Stevenson")
	}
	if len(result.StatuteFiles) != 1 || len(result.PrecedentFiles) != 1 {
		t.Errorf("file lists = %v / %v, want 1 entry each", result.StatuteFiles, result.PrecedentFiles)
	}
}

func TestProvisionCorpus_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "contracts.statute.txt")
	writeBundleFile(t, dir, "contracts.statute.txt", testStatuteBody)

	result, err := airgapped.ProvisionCorpus(context.Background(), path)
	if err != nil {
		t.Fatalf("ProvisionCorpus: %v", err)
	}
	if len(result.Statutes) != 1 {
		t.Fatalf("len(Statutes) = %d, want 1", len(result.Statutes))
	}
}

func TestProvisionCorpus_EmptyPath(t *testing.T) {
	_, err := airgapped.ProvisionCorpus(context.Background(), "")
	if !errors.Is(err, airgapped.ErrEmptyBundlePath) {
		t.Fatalf("ProvisionCorpus(\"\") error = %v, want ErrEmptyBundlePath", err)
	}
}

func TestProvisionCorpus_NonexistentPath(t *testing.T) {
	_, err := airgapped.ProvisionCorpus(context.Background(), "/no/such/bundle/path")
	if !errors.Is(err, airgapped.ErrBundleNotFound) {
		t.Fatalf("ProvisionCorpus(missing) error = %v, want ErrBundleNotFound", err)
	}
}

func TestProvisionCorpus_EmptyBundle(t *testing.T) {
	dir := t.TempDir()
	writeBundleFile(t, dir, "readme.txt", "not a recognized corpus file")

	_, err := airgapped.ProvisionCorpus(context.Background(), dir)
	if !errors.Is(err, airgapped.ErrEmptyCorpusBundle) {
		t.Fatalf("ProvisionCorpus(empty) error = %v, want ErrEmptyCorpusBundle", err)
	}
}

func TestFileBundleStatuteLoader_SatisfiesLoaderInterface(t *testing.T) {
	loader := &airgapped.FileBundleStatuteLoader{}
	result, err := loader.Load(context.Background(), stringsReader(testStatuteBody))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
}

func TestFileBundlePrecedentLoader_SatisfiesLoaderInterface(t *testing.T) {
	loader := &airgapped.FileBundlePrecedentLoader{}
	result, err := loader.Load(context.Background(), stringsReader(testPrecedentBody))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
}
