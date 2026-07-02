package category

import (
	"context"
	"errors"
	"testing"
)

func TestCategoryService_Categorize_SuggestionOnly(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	procedural := NewProceduralRules()
	procedural.Register("IN", CodeCivil, ProceduralRuleRef{Code: "CPC", Name: "Code of Civil Procedure"})
	statutes := NewStatutePartitions()
	statutes.Register("IN", CodeCivil, StatutePartitionRef{PartitionID: "IN-CONTRACT-ACT"})
	audit := &CapturingAuditSink{}

	svc := &CategoryService{
		Suggester:  NewKeywordSuggester(),
		Procedural: procedural,
		Statutes:   statutes,
		Audit:      audit,
	}

	result, err := svc.Categorize(context.Background(), CategorizeRequest{
		CaseID:           "case-1",
		JurisdictionCode: "IN",
		Text:             "The plaintiff alleges a breach of contract and seeks damages.",
		Taxonomy:         tax,
		Actor:            "user-1",
	})
	if err != nil {
		t.Fatalf("Categorize() error = %v, want nil", err)
	}

	if result.Assignment.Category.Code != CodeCivil {
		t.Errorf("got category %q, want %q", result.Assignment.Category.Code, CodeCivil)
	}
	if result.Assignment.Override != nil {
		t.Error("got non-nil Override, want nil (no override supplied)")
	}
	if len(result.Assignment.Suggestions) == 0 {
		t.Error("got no suggestions retained on assignment")
	}
	if len(result.ProceduralRules) != 1 || result.ProceduralRules[0].Code != "CPC" {
		t.Errorf("got procedural rules %v, want [{CPC ...}]", result.ProceduralRules)
	}
	if len(result.StatutePartitions) != 1 || result.StatutePartitions[0].PartitionID != "IN-CONTRACT-ACT" {
		t.Errorf("got statute partitions %v, want [{IN-CONTRACT-ACT}]", result.StatutePartitions)
	}

	// Audit trail completeness: suggestion + validation + final change.
	var sawSuggested, sawValidated, sawChanged bool
	for _, e := range audit.Events {
		switch e.EventType {
		case AuditEventSuggested:
			sawSuggested = true
		case AuditEventValidated:
			sawValidated = true
		case AuditEventChanged:
			sawChanged = true
		}
	}
	if !sawSuggested || !sawValidated || !sawChanged {
		t.Errorf("incomplete audit trail: suggested=%v validated=%v changed=%v (events=%+v)", sawSuggested, sawValidated, sawChanged, audit.Events)
	}
}

func TestCategoryService_Categorize_WithOverride(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	audit := &CapturingAuditSink{}
	svc := &CategoryService{Audit: audit}

	consumer, _ := tax.Lookup("IN", CodeConsumer)
	result, err := svc.Categorize(context.Background(), CategorizeRequest{
		CaseID:           "case-2",
		JurisdictionCode: "IN",
		Text:             "The plaintiff alleges a breach of contract.",
		Taxonomy:         tax,
		Override: &ManualOverride{
			CaseID:     "case-2",
			Category:   consumer,
			ReviewedBy: "reviewer-1",
			Reason:     "actually a consumer complaint",
		},
		Actor: "reviewer-1",
	})
	if err != nil {
		t.Fatalf("Categorize() error = %v, want nil", err)
	}

	if result.Assignment.Category.Code != CodeConsumer {
		t.Errorf("got category %q, want %q (override should take precedence)", result.Assignment.Category.Code, CodeConsumer)
	}
	if result.Assignment.Confidence != 1.0 {
		t.Errorf("got confidence %v, want 1.0", result.Assignment.Confidence)
	}
	if result.Assignment.Override == nil {
		t.Fatal("got nil Override, want non-nil")
	}
	if result.Assignment.Override.Previous == nil {
		t.Fatal("got nil Override.Previous, want the original suggestion retained")
	}
	if result.Assignment.Override.Previous.Category.Code != CodeCivil {
		t.Errorf("Override.Previous.Category = %q, want %q (original suggestion retained alongside override)",
			result.Assignment.Override.Previous.Category.Code, CodeCivil)
	}
	if len(result.Assignment.Suggestions) == 0 {
		t.Error("got no suggestions retained on overridden assignment")
	}

	var sawOverridden bool
	for _, e := range audit.Events {
		if e.EventType == AuditEventOverridden {
			sawOverridden = true
		}
	}
	if !sawOverridden {
		t.Errorf("audit trail missing %q event: %+v", AuditEventOverridden, audit.Events)
	}
}

func TestCategoryService_Categorize_Errors(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")

	tests := []struct {
		name    string
		req     CategorizeRequest
		wantErr error
	}{
		{
			name:    "missing case id",
			req:     CategorizeRequest{JurisdictionCode: "IN", Text: "some text", Taxonomy: tax},
			wantErr: ErrCaseIDRequired,
		},
		{
			name:    "unknown jurisdiction",
			req:     CategorizeRequest{CaseID: "case-1", JurisdictionCode: "ZZ", Text: "some text", Taxonomy: tax},
			wantErr: ErrUnknownJurisdiction,
		},
		{
			name:    "empty text and no override",
			req:     CategorizeRequest{CaseID: "case-1", JurisdictionCode: "IN", Taxonomy: tax},
			wantErr: ErrEmptyInput,
		},
	}

	svc := NewCategoryService()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Categorize(context.Background(), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Categorize() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCategoryService_Categorize_OverrideCategoryNotInJurisdiction(t *testing.T) {
	tax := NewDefaultTaxonomy("IN")
	svc := NewCategoryService()

	_, err := svc.Categorize(context.Background(), CategorizeRequest{
		CaseID:           "case-1",
		JurisdictionCode: "IN",
		Text:             "some text",
		Taxonomy:         tax,
		Override: &ManualOverride{
			CaseID:   "case-1",
			Category: Category{Code: "not-real", Name: "Not Real"},
		},
	})
	if !errors.Is(err, ErrCategoryNotInJurisdiction) {
		t.Errorf("Categorize() error = %v, want %v", err, ErrCategoryNotInJurisdiction)
	}
}

func TestCategoryService_Categorize_OverrideWithoutSuggestion(t *testing.T) {
	// An override supplied with no Text (nothing to suggest from) must
	// still succeed: the override alone determines the final category.
	tax := NewDefaultTaxonomy("IN")
	svc := NewCategoryService()

	civil, _ := tax.Lookup("IN", CodeCivil)
	result, err := svc.Categorize(context.Background(), CategorizeRequest{
		CaseID:           "case-1",
		JurisdictionCode: "IN",
		Taxonomy:         tax,
		Override: &ManualOverride{
			CaseID:   "case-1",
			Category: civil,
		},
	})
	if err != nil {
		t.Fatalf("Categorize() error = %v, want nil", err)
	}
	if result.Assignment.Category.Code != CodeCivil {
		t.Errorf("got category %q, want %q", result.Assignment.Category.Code, CodeCivil)
	}
}

func TestNewCategoryService_Defaults(t *testing.T) {
	svc := NewCategoryService()
	if svc.Suggester == nil {
		t.Error("Suggester is nil, want a default KeywordSuggester")
	}
	if svc.Procedural == nil {
		t.Error("Procedural is nil, want a default ProceduralRules")
	}
	if svc.Statutes == nil {
		t.Error("Statutes is nil, want a default StatutePartitions")
	}
	if svc.Audit == nil {
		t.Error("Audit is nil, want a default NoOpAuditSink")
	}
}

func TestCategoryService_Categorize_NilDependencies(t *testing.T) {
	// A zero-value CategoryService (all fields nil) must still work,
	// falling back to defaults inside Categorize.
	svc := &CategoryService{}
	tax := NewDefaultTaxonomy("IN")

	result, err := svc.Categorize(context.Background(), CategorizeRequest{
		CaseID:           "case-1",
		JurisdictionCode: "IN",
		Text:             "The prosecution charged the accused with a felony.",
		Taxonomy:         tax,
	})
	if err != nil {
		t.Fatalf("Categorize() error = %v, want nil", err)
	}
	if result.Assignment.Category.Code != CodeCriminal {
		t.Errorf("got category %q, want %q", result.Assignment.Category.Code, CodeCriminal)
	}
}
