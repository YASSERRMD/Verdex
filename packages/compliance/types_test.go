package compliance_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func TestFramework_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		f    compliance.Framework
		want bool
	}{
		{"uae data protection", compliance.FrameworkUAEDataProtection, true},
		{"international overlay", compliance.FrameworkInternationalDataProtection, true},
		{"judicial records", compliance.FrameworkJudicialRecordsHandling, true},
		{"arbitrary non-blank framework", compliance.Framework("iso_27001"), true},
		{"blank", compliance.Framework(""), false},
		{"whitespace only", compliance.Framework("   "), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.f.IsValid(); got != tt.want {
				t.Errorf("Framework(%q).IsValid() = %v, want %v", tt.f, got, tt.want)
			}
		})
	}
}

func TestControlCategory_IsValid(t *testing.T) {
	t.Parallel()

	if !compliance.CategoryLawfulBasis.IsValid() {
		t.Error("CategoryLawfulBasis.IsValid() = false, want true")
	}
	if compliance.ControlCategory("not_a_category").IsValid() {
		t.Error("unknown ControlCategory.IsValid() = true, want false")
	}
}

func validControl() compliance.Control {
	return compliance.Control{
		Code:      "TEST-01",
		Title:     "Test control",
		Framework: compliance.FrameworkUAEDataProtection,
		Category:  compliance.CategoryLawfulBasis,
	}
}

func TestControl_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid control passes", func(t *testing.T) {
		t.Parallel()
		c := validControl()
		if err := c.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("nil control", func(t *testing.T) {
		t.Parallel()
		var c *compliance.Control
		if err := c.Validate(); !errors.Is(err, compliance.ErrInvalidControl) {
			t.Errorf("Validate() = %v, want ErrInvalidControl", err)
		}
	})

	t.Run("blank code", func(t *testing.T) {
		t.Parallel()
		c := validControl()
		c.Code = "   "
		if err := c.Validate(); !errors.Is(err, compliance.ErrInvalidControl) {
			t.Errorf("Validate() = %v, want ErrInvalidControl", err)
		}
	})

	t.Run("blank title", func(t *testing.T) {
		t.Parallel()
		c := validControl()
		c.Title = ""
		if err := c.Validate(); !errors.Is(err, compliance.ErrInvalidControl) {
			t.Errorf("Validate() = %v, want ErrInvalidControl", err)
		}
	})

	t.Run("invalid framework", func(t *testing.T) {
		t.Parallel()
		c := validControl()
		c.Framework = ""
		if err := c.Validate(); !errors.Is(err, compliance.ErrInvalidFramework) {
			t.Errorf("Validate() = %v, want ErrInvalidFramework", err)
		}
	})

	t.Run("invalid category", func(t *testing.T) {
		t.Parallel()
		c := validControl()
		c.Category = "not_a_category"
		if err := c.Validate(); !errors.Is(err, compliance.ErrInvalidControl) {
			t.Errorf("Validate() = %v, want ErrInvalidControl", err)
		}
	})
}
