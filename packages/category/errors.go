package category

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrCategoryNotInJurisdiction is returned when a Category is validated
	// against a jurisdiction whose Taxonomy does not contain that
	// Category.Code (see validate.go).
	ErrCategoryNotInJurisdiction = errors.New("category: category not valid for jurisdiction")

	// ErrUnknownParent is returned when a Category's ParentCode does not
	// resolve to another Category.Code already registered in the same
	// jurisdiction's Taxonomy (see taxonomy.go, subcategory.go).
	ErrUnknownParent = errors.New("category: unknown parent category")

	// ErrUnknownJurisdiction is returned when an operation references a
	// jurisdiction code that has no entry in the Taxonomy at all.
	ErrUnknownJurisdiction = errors.New("category: unknown jurisdiction")

	// ErrInvalidOverride is returned when a ManualOverride fails basic
	// validation before being applied (see override.go).
	ErrInvalidOverride = errors.New("category: invalid manual override")

	// ErrEmptyInput is returned when a suggestion or categorization
	// operation is given empty (or whitespace-only) text.
	ErrEmptyInput = errors.New("category: input text is empty")

	// ErrCaseIDRequired is returned when an operation that produces an
	// audit trail is given an empty case identifier.
	ErrCaseIDRequired = errors.New("category: case id is required")
)
