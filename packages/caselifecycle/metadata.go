package caselifecycle

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MetadataUpdateInput bundles the arguments for SetMetadata and
// MergeMetadata.
type MetadataUpdateInput struct {
	// TenantID scopes the operation.
	TenantID uuid.UUID

	// CaseID identifies the case whose metadata is being updated.
	CaseID uuid.UUID

	// Values is the metadata to write. SetMetadata replaces the
	// entire map with Values; MergeMetadata overlays Values onto the
	// existing map (adding new keys, overwriting existing ones,
	// leaving keys not present in Values untouched).
	Values map[string]string

	// ExpectedVersion, if non-zero, must match the case's current
	// MetadataVersion or the update is rejected with
	// ErrMetadataVersionConflict. Pass 0 to skip the version check
	// (last-writer-wins).
	ExpectedVersion int
}

// validateMetadataValues returns ErrInvalidMetadataKey if any key in
// values is blank after trimming.
func validateMetadataValues(values map[string]string) error {
	for k := range values {
		if strings.TrimSpace(k) == "" {
			return ErrInvalidMetadataKey
		}
	}
	return nil
}

// SetMetadata replaces a case's entire Metadata map with
// input.Values, validates every key, bumps MetadataVersion, and
// persists the result via repo.Update.
//
// Returns ErrNotFound if the case does not exist or is not visible to
// input.TenantID, ErrInvalidMetadataKey if any key in input.Values is
// blank, and ErrMetadataVersionConflict if input.ExpectedVersion is
// non-zero and does not match the case's current MetadataVersion.
func SetMetadata(ctx context.Context, repo Repository, input MetadataUpdateInput) (*Case, error) {
	return updateMetadata(ctx, repo, input, false)
}

// MergeMetadata overlays input.Values onto a case's existing Metadata
// map (new keys are added, existing keys are overwritten, keys absent
// from input.Values are left untouched), validates every key in
// input.Values, bumps MetadataVersion, and persists the result via
// repo.Update.
//
// Error conditions mirror SetMetadata.
func MergeMetadata(ctx context.Context, repo Repository, input MetadataUpdateInput) (*Case, error) {
	return updateMetadata(ctx, repo, input, true)
}

func updateMetadata(ctx context.Context, repo Repository, input MetadataUpdateInput, merge bool) (*Case, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if err := validateMetadataValues(input.Values); err != nil {
		return nil, err
	}

	c, err := repo.Get(ctx, input.TenantID, input.CaseID)
	if err != nil {
		return nil, err
	}

	if input.ExpectedVersion != 0 && input.ExpectedVersion != c.MetadataVersion {
		return nil, ErrMetadataVersionConflict
	}

	if merge {
		next := make(map[string]string, len(c.Metadata)+len(input.Values))
		for k, v := range c.Metadata {
			next[k] = v
		}
		for k, v := range input.Values {
			next[k] = v
		}
		c.Metadata = next
	} else {
		next := make(map[string]string, len(input.Values))
		for k, v := range input.Values {
			next[k] = v
		}
		c.Metadata = next
	}

	c.MetadataVersion++
	c.UpdatedAt = time.Now().UTC()

	if err := repo.Update(ctx, input.TenantID, c); err != nil {
		return nil, err
	}
	return c, nil
}

// GetMetadataValue returns the value for key in c's Metadata map, and
// whether that key is present. A nil-safe convenience so callers do
// not need to nil-check c.Metadata themselves.
func GetMetadataValue(c *Case, key string) (string, bool) {
	if c == nil || c.Metadata == nil {
		return "", false
	}
	v, ok := c.Metadata[key]
	return v, ok
}
