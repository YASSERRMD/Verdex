package jurisdiction_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/jurisdiction"
)

func TestSeedData_MinimumCount(t *testing.T) {
	t.Parallel()
	data := jurisdiction.SeedData()
	if len(data) < 10 {
		t.Errorf("SeedData() returned %d jurisdiction(s); want at least 10", len(data))
	}
}

func TestSeedData_AllValid(t *testing.T) {
	t.Parallel()
	for i, j := range jurisdiction.SeedData() {
		if err := jurisdiction.Validate(j); err != nil {
			t.Errorf("SeedData()[%d] (%q) failed Validate(): %v", i, j.CourtName, err)
		}
	}
}

func TestSeedData_UniqueIDs(t *testing.T) {
	t.Parallel()
	seen := make(map[uuid.UUID]int)
	for i, j := range jurisdiction.SeedData() {
		if prev, dup := seen[j.ID]; dup {
			t.Errorf("SeedData()[%d] has duplicate ID %v (first seen at index %d)", i, j.ID, prev)
		}
		seen[j.ID] = i
	}
}

func TestSeedData_CoversExpectedCountries(t *testing.T) {
	t.Parallel()
	required := []string{"AE", "PK", "IN", "LK", "GB", "US", "EG", "SA", "MY", "NG"}

	data := jurisdiction.SeedData()
	found := make(map[string]bool)
	for _, j := range data {
		found[j.CountryCode] = true
	}

	for _, cc := range required {
		if !found[cc] {
			t.Errorf("SeedData() missing jurisdiction for country code %q", cc)
		}
	}
}

func TestSeedData_AllHaveProceduralRules(t *testing.T) {
	t.Parallel()
	for i, j := range jurisdiction.SeedData() {
		if len(j.ProceduralRules) == 0 {
			t.Errorf("SeedData()[%d] (%q) has no procedural rules", i, j.CourtName)
		}
	}
}

func TestSeedData_LegalFamiliesRepresented(t *testing.T) {
	t.Parallel()
	families := make(map[jurisdiction.LegalFamily]bool)
	for _, j := range jurisdiction.SeedData() {
		families[j.LegalFamily] = true
	}

	required := []jurisdiction.LegalFamily{
		jurisdiction.LegalFamilyCommonLaw,
		jurisdiction.LegalFamilyMixed,
		jurisdiction.LegalFamilyIslamicLaw,
	}
	for _, lf := range required {
		if !families[lf] {
			t.Errorf("SeedData() does not include any jurisdiction with legal family %q", lf)
		}
	}
}
