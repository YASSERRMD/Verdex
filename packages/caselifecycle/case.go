package caselifecycle

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// NewCaseInput bundles the fields a caller supplies to create a new
// Case. ID, State, MetadataVersion, CreatedAt, and UpdatedAt are
// assigned by NewCase, not the caller.
type NewCaseInput struct {
	// TenantID is the tenant this case belongs to. Required.
	TenantID uuid.UUID

	// JurisdictionID links to the jurisdiction this case is heard
	// under. Required.
	JurisdictionID uuid.UUID

	// CategoryID optionally links to a packages/category taxonomy
	// entry. May be left blank and assigned later during intake.
	CategoryID string

	// Title is a short human-readable label for the case. Required.
	Title string

	// Reference is an optional external/docket reference number.
	Reference string

	// Metadata seeds the case's initial metadata map. May be nil.
	Metadata map[string]string

	// CreatedBy is the ID of the identity.User creating this case.
	// Required.
	CreatedBy uuid.UUID
}

// NewCase constructs a new Case in StateDraft from input, generating a
// fresh ID and stamping CreatedAt/UpdatedAt to the current time. It
// returns ErrInvalidCase if the constructed Case fails Validate, and
// ErrUnauthenticated if input.CreatedBy is unset.
//
// NewCase only builds the in-memory value; callers must still persist
// it via a Repository's Create method (see repository.go).
func NewCase(input NewCaseInput) (*Case, error) {
	if input.CreatedBy == uuid.Nil {
		return nil, ErrUnauthenticated
	}

	now := time.Now().UTC()
	metadata := make(map[string]string, len(input.Metadata))
	for k, v := range input.Metadata {
		metadata[k] = v
	}

	c := &Case{
		ID:              uuid.New(),
		TenantID:        input.TenantID,
		JurisdictionID:  input.JurisdictionID,
		CategoryID:      strings.TrimSpace(input.CategoryID),
		Title:           strings.TrimSpace(input.Title),
		Reference:       strings.TrimSpace(input.Reference),
		State:           StateDraft,
		Metadata:        metadata,
		MetadataVersion: 1,
		CreatedBy:       input.CreatedBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}
