package jurisdiction_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

func seedService(t *testing.T) *jurisdiction.InMemoryLookupService {
	t.Helper()
	return jurisdiction.NewInMemoryLookupService(jurisdiction.SeedData())
}

func TestInMemoryLookupService_GetByID_Found(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	// The first seed entry has a well-known UUID.
	id := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	got, err := svc.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("GetByID() unexpected error: %v", err)
	}
	if got.ID != id {
		t.Errorf("GetByID() ID = %v; want %v", got.ID, id)
	}
	if got.CountryCode != "AE" {
		t.Errorf("GetByID() CountryCode = %q; want %q", got.CountryCode, "AE")
	}
}

func TestInMemoryLookupService_GetByID_NotFound(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	_, err := svc.GetByID(context.Background(), uuid.New())
	if err == nil {
		t.Fatal("expected ErrJurisdictionNotFound, got nil")
	}
	if !errors.Is(err, jurisdiction.ErrJurisdictionNotFound) {
		t.Errorf("expected ErrJurisdictionNotFound; got %v", err)
	}
}

func TestInMemoryLookupService_GetByCountry(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	results, err := svc.GetByCountry(context.Background(), "AE")
	if err != nil {
		t.Fatalf("GetByCountry() unexpected error: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("GetByCountry(AE) returned %d result(s); want at least 2", len(results))
	}
	for _, r := range results {
		if r.CountryCode != "AE" {
			t.Errorf("GetByCountry(AE) returned entry with CountryCode %q", r.CountryCode)
		}
	}
}

func TestInMemoryLookupService_GetByCountry_Empty(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	results, err := svc.GetByCountry(context.Background(), "ZZ")
	if err != nil {
		t.Fatalf("GetByCountry() unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("GetByCountry(ZZ) returned %d result(s); want 0", len(results))
	}
}

func TestInMemoryLookupService_ListAll(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	all, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() unexpected error: %v", err)
	}
	if len(all) < 10 {
		t.Errorf("ListAll() returned %d entries; want at least 10", len(all))
	}
}

func TestInMemoryLookupService_Search_ByCourtName(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	results, err := svc.Search(context.Background(), "supreme")
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) == 0 {
		t.Error("Search('supreme') returned no results; expected at least one")
	}
}

func TestInMemoryLookupService_Search_ByCountryName(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	results, err := svc.Search(context.Background(), "Pakistan")
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) < 2 {
		t.Errorf("Search('Pakistan') returned %d result(s); want at least 2", len(results))
	}
}

func TestInMemoryLookupService_Search_EmptyQuery(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	all, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() unexpected error: %v", err)
	}

	results, err := svc.Search(context.Background(), "")
	if err != nil {
		t.Fatalf("Search('') unexpected error: %v", err)
	}
	if len(results) != len(all) {
		t.Errorf("Search('') returned %d result(s); want %d (same as ListAll)", len(results), len(all))
	}
}

func TestInMemoryLookupService_Search_NoMatch(t *testing.T) {
	t.Parallel()
	svc := seedService(t)

	results, err := svc.Search(context.Background(), "xyzzy_no_match")
	if err != nil {
		t.Fatalf("Search() unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Search('xyzzy_no_match') returned %d result(s); want 0", len(results))
	}
}

func TestNewInMemoryLookupService_SkipsInvalid(t *testing.T) {
	t.Parallel()

	// One valid, one invalid (missing country code).
	valid := validJurisdiction()
	invalid := validJurisdiction()
	invalid.CountryCode = ""

	svc := jurisdiction.NewInMemoryLookupService([]jurisdiction.Jurisdiction{valid, invalid})

	all, err := svc.ListAll(context.Background())
	if err != nil {
		t.Fatalf("ListAll() unexpected error: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 valid entry; got %d", len(all))
	}
}
