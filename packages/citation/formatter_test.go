package citation_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
)

func TestCommonLawFormatterStatute(t *testing.T) {
	in := citation.FormatInput{Act: "Act 12", Section: "5", Clause: "a", Origin: citation.OriginStatute}
	got := citation.CommonLawFormatter.Format(in)
	want := "Act 12, s.5(a)"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestCommonLawFormatterPrecedent(t *testing.T) {
	in := citation.FormatInput{CaseName: "Smith v Jones", RawCitation: "[2020] UKSC 1", Origin: citation.OriginPrecedent}
	got := citation.CommonLawFormatter.Format(in)
	want := "Smith v Jones [2020] UKSC 1"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestCivilLawFormatterStatute(t *testing.T) {
	in := citation.FormatInput{Act: "Code Civil", Section: "5", Origin: citation.OriginStatute}
	got := citation.CivilLawFormatter.Format(in)
	want := "Art. 5 Code Civil"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestCivilLawFormatterPrecedent(t *testing.T) {
	in := citation.FormatInput{CaseName: "Dupont v Durand", RawCitation: "Cass. civ. 1re, 2020", Origin: citation.OriginPrecedent}
	got := citation.CivilLawFormatter.Format(in)
	want := "Dupont v Durand, Cass. civ. 1re, 2020"
	if got != want {
		t.Errorf("Format() = %q, want %q", got, want)
	}
}

func TestFormatterEmptyInputs(t *testing.T) {
	if got := citation.CommonLawFormatter.Format(citation.FormatInput{Origin: citation.OriginPrecedent}); got != "" {
		t.Errorf("CommonLawFormatter empty precedent = %q, want empty", got)
	}
	if got := citation.CivilLawFormatter.Format(citation.FormatInput{Origin: citation.OriginStatute}); got != "" {
		t.Errorf("CivilLawFormatter empty statute = %q, want empty", got)
	}
}

func TestNewDefaultRegistry(t *testing.T) {
	registry := citation.NewDefaultRegistry()

	got, err := registry.Format("common_law", citation.FormatInput{Act: "Act 1", Section: "2", Origin: citation.OriginStatute})
	if err != nil {
		t.Fatalf("Format(common_law) error = %v", err)
	}
	if got != "Act 1, s.2" {
		t.Errorf("Format(common_law) = %q, want %q", got, "Act 1, s.2")
	}

	got, err = registry.Format("civil_law", citation.FormatInput{Act: "Code Civil", Section: "2", Origin: citation.OriginStatute})
	if err != nil {
		t.Fatalf("Format(civil_law) error = %v", err)
	}
	if got != "Art. 2 Code Civil" {
		t.Errorf("Format(civil_law) = %q, want %q", got, "Art. 2 Code Civil")
	}

	if !registry.Has("common_law") || !registry.Has("civil_law") {
		t.Error("expected both common_law and civil_law to be registered")
	}
	if registry.Has("sharia_law") {
		t.Error("Has(sharia_law) = true, want false")
	}
}

func TestRegistryUnknownKeyWithoutFallback(t *testing.T) {
	registry := citation.NewRegistry()
	_, err := registry.Format("unknown", citation.FormatInput{})
	if !errors.Is(err, citation.ErrUnknownFormatter) {
		t.Errorf("Format(unknown) error = %v, want ErrUnknownFormatter", err)
	}
}

func TestRegistryFallback(t *testing.T) {
	registry := citation.NewRegistry().WithFallback(citation.CommonLawFormatter)
	got, err := registry.Format("unknown", citation.FormatInput{Act: "Act 1", Origin: citation.OriginStatute})
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got != "Act 1" {
		t.Errorf("Format() = %q, want %q", got, "Act 1")
	}
}

func TestRegistryRegisterOverwrites(t *testing.T) {
	registry := citation.NewRegistry()
	registry.Register("civil_law", citation.CommonLawFormatter)
	registry.Register("civil_law", citation.CivilLawFormatter)

	got, err := registry.Format("civil_law", citation.FormatInput{Act: "Code Civil", Section: "5", Origin: citation.OriginStatute})
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if got != "Art. 5 Code Civil" {
		t.Errorf("Format() = %q, want civil-law style after overwrite", got)
	}
}

func TestFormatterFuncAdapter(t *testing.T) {
	var f citation.Formatter = citation.FormatterFunc(func(in citation.FormatInput) string {
		return "custom:" + in.Act
	})
	if got := f.Format(citation.FormatInput{Act: "X"}); got != "custom:X" {
		t.Errorf("Format() = %q, want custom:X", got)
	}
}
