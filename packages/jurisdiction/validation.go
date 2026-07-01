package jurisdiction

import (
	"fmt"
	"strings"
	"unicode"
)

// Validate checks that j satisfies all structural and business-rule
// constraints.  It returns a descriptive error (wrapping ErrInvalidJurisdiction
// via fmt.Errorf) on the first violation encountered.
func Validate(j Jurisdiction) error {
	if err := validateCountryCode(j.CountryCode); err != nil {
		return err
	}

	if strings.TrimSpace(j.CountryName) == "" {
		return fmt.Errorf("%w: country name must not be empty", ErrInvalidJurisdiction)
	}

	if strings.TrimSpace(j.CourtName) == "" {
		return fmt.Errorf("%w: court name must not be empty", ErrInvalidJurisdiction)
	}

	if err := validateCourtLevel(j.CourtLevel); err != nil {
		return err
	}

	if !j.LegalFamily.IsValid() {
		return fmt.Errorf("%w: unknown legal family %q", ErrInvalidJurisdiction, j.LegalFamily)
	}

	if err := validateLanguages(j.Languages); err != nil {
		return err
	}

	return nil
}

// validateCountryCode ensures code is exactly two uppercase ASCII letters.
func validateCountryCode(code string) error {
	if len(code) != 2 {
		return fmt.Errorf("%w: got %q (length %d)", ErrCountryCodeInvalid, code, len(code))
	}
	for _, r := range code {
		if !unicode.IsLetter(r) || !unicode.IsUpper(r) || r > 'Z' {
			return fmt.Errorf("%w: got %q (must be uppercase ASCII letters)", ErrCountryCodeInvalid, code)
		}
	}
	return nil
}

// validateLanguages ensures at least one language is provided and that each
// entry is a non-empty, lowercase 2-letter ISO 639-1 code.
func validateLanguages(langs []string) error {
	if len(langs) == 0 {
		return fmt.Errorf("%w: at least one language must be specified", ErrInvalidJurisdiction)
	}
	for _, l := range langs {
		if strings.TrimSpace(l) == "" {
			return fmt.Errorf("%w: language code must not be blank", ErrInvalidJurisdiction)
		}
		if len(l) != 2 {
			return fmt.Errorf("%w: language code %q must be a 2-letter ISO 639-1 code", ErrInvalidJurisdiction, l)
		}
		for _, r := range l {
			if !unicode.IsLetter(r) || !unicode.IsLower(r) {
				return fmt.Errorf("%w: language code %q must be lowercase ASCII letters", ErrInvalidJurisdiction, l)
			}
		}
	}
	return nil
}

// validateCourtLevel returns an error if cl is not a recognised CourtLevel.
func validateCourtLevel(cl CourtLevel) error {
	if !cl.IsValid() {
		return fmt.Errorf("%w: unknown court level %q", ErrInvalidJurisdiction, cl)
	}
	return nil
}
