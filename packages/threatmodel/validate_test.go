package threatmodel_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestValidateSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		maxBytes int
		wantErr  bool
	}{
		{"within default limit", "hello", 0, false},
		{"exactly at explicit limit", "hello", 5, false},
		{"one byte over explicit limit", "hello!", 5, true},
		{"exceeds default limit", strings.Repeat("a", threatmodel.DefaultMaxInputBytes+1), 0, true},
		{"negative maxBytes falls back to default", "hello", -1, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := threatmodel.ValidateSize(tt.input, tt.maxBytes)
			if tt.wantErr && !errors.Is(err, threatmodel.ErrInputTooLarge) {
				t.Errorf("ValidateSize() = %v, want ErrInputTooLarge", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateSize() = %v, want nil", err)
			}
		})
	}
}

func TestValidateCharset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"plain ascii", "hello world", false},
		{"unicode letters", "héllo wörld", false},
		{"tab newline cr allowed", "line1\tvalue\nline2\r\n", false},
		{"null byte rejected", "hello\x00world", true},
		{"bell character rejected", "hello\x07world", true},
		{"escape character rejected", "\x1b[31mred\x1b[0m", true},
		{"invalid utf-8", string([]byte{0xff, 0xfe, 0xfd}), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := threatmodel.ValidateCharset(tt.input)
			if tt.wantErr && !errors.Is(err, threatmodel.ErrInputInvalidCharset) {
				t.Errorf("ValidateCharset() = %v, want ErrInputInvalidCharset", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateCharset() = %v, want nil", err)
			}
		})
	}
}

func TestSanitizeControlChars(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"tab newline cr preserved", "a\tb\nc\r\n", "a\tb\nc\r\n"},
		{"null byte stripped", "hello\x00world", "helloworld"},
		{"multiple control chars stripped", "a\x01b\x02c\x1bd", "abcd"},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := threatmodel.SanitizeControlChars(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeControlChars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}

	t.Run("result is always valid utf-8 after sanitizing", func(t *testing.T) {
		t.Parallel()
		got := threatmodel.SanitizeControlChars("hello\x00\x01world")
		if !strings.Contains(got, "helloworld") {
			t.Errorf("SanitizeControlChars() = %q, want to contain %q", got, "helloworld")
		}
	})
}

func TestValidateNonBlank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-blank", "hello", false},
		{"empty", "", true},
		{"whitespace only", "   \t\n  ", true},
		{"leading/trailing whitespace with content", "  hello  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := threatmodel.ValidateNonBlank(tt.input)
			if tt.wantErr && !errors.Is(err, threatmodel.ErrInputInvalidStructure) {
				t.Errorf("ValidateNonBlank() = %v, want ErrInputInvalidStructure", err)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateNonBlank() = %v, want nil", err)
			}
		})
	}
}

func TestValidateMaxRunes(t *testing.T) {
	t.Parallel()

	t.Run("within limit", func(t *testing.T) {
		t.Parallel()
		if err := threatmodel.ValidateMaxRunes("hello", 10); err != nil {
			t.Errorf("ValidateMaxRunes() = %v, want nil", err)
		}
	})

	t.Run("exceeds limit", func(t *testing.T) {
		t.Parallel()
		if err := threatmodel.ValidateMaxRunes("hello world", 5); !errors.Is(err, threatmodel.ErrInputTooLarge) {
			t.Errorf("ValidateMaxRunes() = %v, want ErrInputTooLarge", err)
		}
	})

	t.Run("multi-byte runes counted as one rune each", func(t *testing.T) {
		t.Parallel()
		// "héllo" is 5 runes but more than 5 bytes (é is 2 bytes in UTF-8).
		if err := threatmodel.ValidateMaxRunes("héllo", 5); err != nil {
			t.Errorf("ValidateMaxRunes() = %v, want nil (rune count is 5)", err)
		}
	})

	t.Run("zero limit disables check", func(t *testing.T) {
		t.Parallel()
		if err := threatmodel.ValidateMaxRunes(strings.Repeat("a", 1000), 0); err != nil {
			t.Errorf("ValidateMaxRunes() with zero limit = %v, want nil", err)
		}
	})
}

func TestValidate_CombinedOptions(t *testing.T) {
	t.Parallel()

	t.Run("all checks pass", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{
			MaxBytes:           100,
			MaxRunes:           50,
			RequireNonBlank:    true,
			RejectControlChars: true,
		}
		if err := threatmodel.Validate("a valid case title", opt); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank input fails RequireNonBlank even if size ok", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{MaxBytes: 100, RequireNonBlank: true}
		if err := threatmodel.Validate("   ", opt); !errors.Is(err, threatmodel.ErrInputInvalidStructure) {
			t.Errorf("Validate() = %v, want ErrInputInvalidStructure", err)
		}
	})

	t.Run("control chars fail when RejectControlChars set", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{MaxBytes: 100, RejectControlChars: true}
		if err := threatmodel.Validate("bad\x00input", opt); !errors.Is(err, threatmodel.ErrInputInvalidCharset) {
			t.Errorf("Validate() = %v, want ErrInputInvalidCharset", err)
		}
	})

	t.Run("control chars allowed when RejectControlChars unset", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{MaxBytes: 100}
		if err := threatmodel.Validate("has\x00null", opt); err != nil {
			t.Errorf("Validate() = %v, want nil (control-char check disabled)", err)
		}
	})

	t.Run("negative MaxBytes disables size check", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{MaxBytes: -1}
		if err := threatmodel.Validate(strings.Repeat("a", threatmodel.DefaultMaxInputBytes+1), opt); err != nil {
			t.Errorf("Validate() = %v, want nil (size check disabled)", err)
		}
	})

	t.Run("exceeds MaxRunes", func(t *testing.T) {
		t.Parallel()
		opt := threatmodel.ValidatorOptions{MaxBytes: 1000, MaxRunes: 3}
		if err := threatmodel.Validate("hello", opt); !errors.Is(err, threatmodel.ErrInputTooLarge) {
			t.Errorf("Validate() = %v, want ErrInputTooLarge", err)
		}
	})
}
