package integration_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func testMapping(tenantID uuid.UUID) *integration.FieldMapping {
	return &integration.FieldMapping{
		ID:            uuid.New(),
		TenantID:      tenantID,
		ConnectorType: "efiling-dubai-courts",
		Name:          "test mapping",
		Rules: []integration.FieldRule{
			{SourceField: "docket_no", TargetField: "reference", Required: true},
			{SourceField: "case_title", TargetField: "title", Required: true},
			{SourceField: "filing_category", TargetField: "category_id", Required: false, DefaultValue: "uncategorized"},
		},
	}
}

func TestFieldMappingApply(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	record := map[string]string{
		"docket_no":     "DXB-2026-001",
		"case_title":    "Doe v. Acme",
		"extra_ignored": "value",
	}

	mapped, err := m.Apply(record)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if mapped.Values["reference"] != "DXB-2026-001" {
		t.Errorf("reference = %q, want DXB-2026-001", mapped.Values["reference"])
	}
	if mapped.Values["title"] != "Doe v. Acme" {
		t.Errorf("title = %q, want Doe v. Acme", mapped.Values["title"])
	}
	if mapped.Values["category_id"] != "uncategorized" {
		t.Errorf("category_id = %q, want default uncategorized", mapped.Values["category_id"])
	}
	if len(mapped.UnmappedSourceFields) != 1 || mapped.UnmappedSourceFields[0] != "extra_ignored" {
		t.Errorf("UnmappedSourceFields = %v, want [extra_ignored]", mapped.UnmappedSourceFields)
	}
}

func TestFieldMappingApplyMissingRequired(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	record := map[string]string{
		"case_title": "Doe v. Acme",
	}

	_, err := m.Apply(record)
	if !errors.Is(err, integration.ErrUnmappedField) {
		t.Fatalf("Apply() error = %v, want ErrUnmappedField", err)
	}
}

func TestFieldMappingApplyDefaultsWhenOptionalMissing(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	record := map[string]string{
		"docket_no":  "DXB-2026-002",
		"case_title": "Roe v. Widget Co",
	}

	mapped, err := m.Apply(record)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if mapped.Values["category_id"] != "uncategorized" {
		t.Errorf("category_id = %q, want default", mapped.Values["category_id"])
	}
}

func TestFieldMappingReverse(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	values := map[string]string{
		"reference":   "DXB-2026-003",
		"title":       "State v. Example",
		"category_id": "criminal",
	}

	out, err := m.Reverse(values)
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if out["docket_no"] != "DXB-2026-003" {
		t.Errorf("docket_no = %q, want DXB-2026-003", out["docket_no"])
	}
	if out["case_title"] != "State v. Example" {
		t.Errorf("case_title = %q, want State v. Example", out["case_title"])
	}
	if out["filing_category"] != "criminal" {
		t.Errorf("filing_category = %q, want criminal", out["filing_category"])
	}
}

func TestFieldMappingReverseMissingRequired(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	values := map[string]string{
		"title": "State v. Example",
	}

	_, err := m.Reverse(values)
	if !errors.Is(err, integration.ErrUnmappedField) {
		t.Fatalf("Reverse() error = %v, want ErrUnmappedField", err)
	}
}

func TestFieldMappingRoundTrip(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	m := testMapping(tenantID)

	original := map[string]string{
		"docket_no":  "DXB-2026-004",
		"case_title": "Round Trip Case",
	}

	mapped, err := m.Apply(original)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	reversed, err := m.Reverse(mapped.Values)
	if err != nil {
		t.Fatalf("Reverse() error = %v", err)
	}
	if reversed["docket_no"] != original["docket_no"] {
		t.Errorf("round-trip docket_no = %q, want %q", reversed["docket_no"], original["docket_no"])
	}
	if reversed["case_title"] != original["case_title"] {
		t.Errorf("round-trip case_title = %q, want %q", reversed["case_title"], original["case_title"])
	}
}

func TestFieldMappingValidate(t *testing.T) {
	t.Parallel()

	t.Run("nil mapping", func(t *testing.T) {
		t.Parallel()
		var m *integration.FieldMapping
		if err := m.Validate(); !errors.Is(err, integration.ErrInvalidFieldMapping) {
			t.Fatalf("Validate() = %v, want ErrInvalidFieldMapping", err)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		t.Parallel()
		m := &integration.FieldMapping{ConnectorType: "x", Name: "y"}
		if err := m.Validate(); !errors.Is(err, integration.ErrEmptyTenantID) {
			t.Fatalf("Validate() = %v, want ErrEmptyTenantID", err)
		}
	})

	t.Run("blank rule fields", func(t *testing.T) {
		t.Parallel()
		m := &integration.FieldMapping{
			TenantID:      uuid.New(),
			ConnectorType: "x",
			Name:          "y",
			Rules:         []integration.FieldRule{{SourceField: "", TargetField: "title"}},
		}
		if err := m.Validate(); !errors.Is(err, integration.ErrInvalidFieldMapping) {
			t.Fatalf("Validate() = %v, want ErrInvalidFieldMapping", err)
		}
	})
}
