package category

import (
	"context"
	"time"
)

// CategoryService orchestrates the full case-categorization pipeline:
//
//	suggest -> validate against jurisdiction -> apply any override
//	  -> map to procedural rules + statute partitions -> audit
//	  -> return final Category assignment
//
// This mirrors packages/evidence's EvidenceService orchestration pattern: a
// single entry point wires together this package's otherwise independent,
// individually testable building blocks (Suggester, ValidateCategory,
// ApplyOverride, ProceduralRules, StatutePartitions, AuditSink).
type CategoryService struct {
	// Suggester proposes candidate categories from case text. If nil,
	// NewKeywordSuggester() is used.
	Suggester Suggester

	// Procedural maps a category to procedural rule references. If nil, an
	// empty ProceduralRules is used (Lookup returns nil for every pair).
	Procedural *ProceduralRules

	// Statutes maps a category to statute partition references. If nil, an
	// empty StatutePartitions is used (Lookup returns nil for every pair).
	Statutes *StatutePartitions

	// Audit receives a CategoryAuditEvent for every suggestion, validation,
	// override, and final assignment. If nil, NoOpAuditSink{} is used.
	Audit AuditSink
}

// NewCategoryService constructs a CategoryService with sensible defaults
// for every pluggable dependency left nil: KeywordSuggester, empty
// ProceduralRules/StatutePartitions tables, and NoOpAuditSink.
func NewCategoryService() *CategoryService {
	return &CategoryService{
		Suggester:  NewKeywordSuggester(),
		Procedural: NewProceduralRules(),
		Statutes:   NewStatutePartitions(),
		Audit:      NoOpAuditSink{},
	}
}

// CategorizeRequest carries the input to CategoryService.Categorize.
type CategorizeRequest struct {
	// CaseID identifies the case being categorized. Required.
	CaseID string

	// JurisdictionCode identifies the jurisdiction to validate the
	// suggested/overridden category against. Required.
	JurisdictionCode string

	// Text is the case content to derive suggestions from via Suggester.
	Text string

	// Taxonomy is the set of valid categories for JurisdictionCode (and any
	// other jurisdictions the caller wants suggestions scoped to).
	Taxonomy Taxonomy

	// Override, when non-nil, is applied after suggestion and takes
	// precedence over the top-ranked suggestion. It must target CaseID.
	Override *ManualOverride

	// Actor identifies who or what initiated this categorization, recorded
	// on every emitted CategoryAuditEvent (e.g. a user ID, or a system
	// identifier such as "system:intake-pipeline").
	Actor string
}

// CategoryResult is the outcome of CategoryService.Categorize: the final
// CategoryAssignment plus the procedural rules and statute partitions
// mapped from its Category.
type CategoryResult struct {
	// Assignment is the final CategoryAssignment (post-override, if any).
	Assignment CategoryAssignment

	// ProceduralRules lists the procedural rule references mapped from
	// Assignment.Category for the request's jurisdiction.
	ProceduralRules []ProceduralRuleRef

	// StatutePartitions lists the statute partition references mapped from
	// Assignment.Category for the request's jurisdiction.
	StatutePartitions []StatutePartitionRef
}

// Categorize runs the full pipeline for req: suggest candidate categories
// from req.Text (skipped if req.Text is empty and req.Override is set),
// validate the chosen category against req.JurisdictionCode's taxonomy,
// apply req.Override if supplied, map the final category to procedural
// rules and statute partitions, emit audit events for each step, and
// return the CategoryResult.
//
// Returns ErrCaseIDRequired if req.CaseID is empty. Returns
// ErrUnknownJurisdiction if req.JurisdictionCode has no entry in
// req.Taxonomy. Returns ErrEmptyInput if req.Text is empty and no Override
// is supplied (there is nothing to categorize). Any Suggester, validation,
// or override error aborts and is returned immediately.
func (s *CategoryService) Categorize(ctx context.Context, req CategorizeRequest) (CategoryResult, error) {
	if req.CaseID == "" {
		return CategoryResult{}, ErrCaseIDRequired
	}
	if !req.Taxonomy.HasJurisdiction(req.JurisdictionCode) {
		return CategoryResult{}, ErrUnknownJurisdiction
	}
	if req.Text == "" && req.Override == nil {
		return CategoryResult{}, ErrEmptyInput
	}

	suggester := s.Suggester
	if suggester == nil {
		suggester = NewKeywordSuggester()
	}
	procedural := s.Procedural
	if procedural == nil {
		procedural = NewProceduralRules()
	}
	statutes := s.Statutes
	if statutes == nil {
		statutes = NewStatutePartitions()
	}
	audit := s.Audit
	if audit == nil {
		audit = NoOpAuditSink{}
	}

	var suggestions []Suggestion
	if req.Text != "" {
		var err error
		suggestions, err = suggester.Suggest(ctx, req.Text, req.Taxonomy)
		if err != nil {
			return CategoryResult{}, err
		}
		for _, sg := range suggestions {
			_ = audit.Emit(ctx, CategoryAuditEvent{
				EventType:        AuditEventSuggested,
				CaseID:           req.CaseID,
				JurisdictionCode: req.JurisdictionCode,
				CategoryCode:     sg.Category.Code,
				Actor:            "system:" + suggesterName(suggester),
				Confidence:       sg.Confidence,
				Timestamp:        time.Now(),
			})
		}
	}

	assignment := CategoryAssignment{
		CaseID:      req.CaseID,
		Suggestions: suggestions,
	}
	if len(suggestions) > 0 {
		assignment.Category = suggestions[0].Category
		assignment.Confidence = suggestions[0].Confidence
	}

	// Validate the pre-override (suggestion-derived) category, when one was
	// produced, against the jurisdiction's taxonomy.
	if assignment.Category.Code != "" {
		if err := ValidateCategory(req.JurisdictionCode, assignment.Category, req.Taxonomy); err != nil {
			return CategoryResult{}, err
		}
		_ = audit.Emit(ctx, CategoryAuditEvent{
			EventType:        AuditEventValidated,
			CaseID:           req.CaseID,
			JurisdictionCode: req.JurisdictionCode,
			CategoryCode:     assignment.Category.Code,
			Actor:            req.Actor,
			Confidence:       assignment.Confidence,
			Timestamp:        time.Now(),
		})
	}

	if req.Override != nil {
		if err := ValidateCategory(req.JurisdictionCode, req.Override.Category, req.Taxonomy); err != nil {
			return CategoryResult{}, err
		}
		applied, err := ApplyOverride(assignment, *req.Override)
		if err != nil {
			return CategoryResult{}, err
		}
		assignment = applied
		_ = audit.Emit(ctx, CategoryAuditEvent{
			EventType:        AuditEventOverridden,
			CaseID:           req.CaseID,
			JurisdictionCode: req.JurisdictionCode,
			CategoryCode:     assignment.Category.Code,
			Actor:            req.Actor,
			Confidence:       assignment.Confidence,
			Timestamp:        time.Now(),
		})
	}

	if assignment.Category.Code == "" {
		return CategoryResult{}, ErrEmptyInput
	}

	_ = audit.Emit(ctx, CategoryAuditEvent{
		EventType:        AuditEventChanged,
		CaseID:           req.CaseID,
		JurisdictionCode: req.JurisdictionCode,
		CategoryCode:     assignment.Category.Code,
		Actor:            req.Actor,
		Confidence:       assignment.Confidence,
		Timestamp:        time.Now(),
	})

	return CategoryResult{
		Assignment:        assignment,
		ProceduralRules:   procedural.LookupCategory(req.JurisdictionCode, assignment.Category),
		StatutePartitions: statutes.LookupCategory(req.JurisdictionCode, assignment.Category),
	}, nil
}

// suggesterName returns a short identifier for suggester's concrete type,
// used to populate CategoryAuditEvent.Actor for suggestion events.
func suggesterName(suggester Suggester) string {
	if _, ok := suggester.(*KeywordSuggester); ok {
		return "keyword-suggester"
	}
	return "suggester"
}
