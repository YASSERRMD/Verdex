package docsite

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyRoot is returned when CheckLinks is called with a blank
	// root path.
	ErrEmptyRoot = errors.New("docsite: root path is required")

	// ErrRootNotFound is returned when root does not exist or is not a
	// directory.
	ErrRootNotFound = errors.New("docsite: root does not exist or is not a directory")

	// ErrNoMarkdownFiles is returned when CheckLinks walks root's
	// known documentation locations (docs/ and packages/*/doc/) and
	// finds zero markdown files to check -- almost always a
	// misconfigured root rather than a legitimately empty
	// documentation tree, so this is a hard error rather than a
	// silently clean Report.
	ErrNoMarkdownFiles = errors.New("docsite: no markdown files found under root")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages, fixed to CheckLinks since it
// is this package's only exported function that can fail this way.
func wrapf(err error) error {
	return fmt.Errorf("docsite: CheckLinks: %w", err)
}
