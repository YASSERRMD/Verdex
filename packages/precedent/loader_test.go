package precedent

import (
	"context"
	"strings"
	"testing"
)

const syntheticTextCorpus = `CASE [1932] AC 562: Donoghue v Stevenson
COURT: House of Lords
DECIDED: 1932-05-26
FACTS: The pursuer drank ginger beer containing the decomposed remains of
a snail, purchased for her by a friend from a retailer.
HELD: A manufacturer of products owes a duty of care to the ultimate
consumer of those products, regardless of any contractual relationship.
RATIO: The neighbour principle requires reasonable care to avoid acts or
omissions which one can reasonably foresee would injure persons closely
and directly affected.

CASE [1970] AC 1004: Home Office v Dorset Yacht Co
COURT: House of Lords
DECIDED: 1970-05-06
HELD: The Home Office may owe a duty of care for damage caused by
escaping Borstal trainees under its control.
`

func syntheticJSONCorpus() string {
	return `[
		{"case_name": "R v Brown", "citation": "[1994] 1 AC 212", "court": "House of Lords", "decided_date": "1993-03-11T00:00:00Z", "full_text": "HELD: Consent is not a defence to assault causing actual bodily harm in these circumstances. RATIO: Public policy limits the scope of consent as a defence."}
	]`
}

func TestDefaultLoader_Load_TextCorpus(t *testing.T) {
	loader := NewDefaultLoader()
	precedents, err := loader.Load(context.Background(), strings.NewReader(syntheticTextCorpus))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(precedents) != 2 {
		t.Fatalf("len(precedents) = %d, want 2", len(precedents))
	}
	if precedents[0].Citation != "[1932] AC 562" || precedents[0].CaseName != "Donoghue v Stevenson" {
		t.Errorf("precedents[0] = %+v, want Citation=[1932] AC 562 CaseName=Donoghue v Stevenson", precedents[0])
	}
	if precedents[0].Court != "House of Lords" {
		t.Errorf("precedents[0].Court = %q, want House of Lords", precedents[0].Court)
	}
	if precedents[0].DecidedDate.IsZero() {
		t.Error("precedents[0].DecidedDate should not be zero")
	}
	if precedents[1].CaseName != "Home Office v Dorset Yacht Co" {
		t.Errorf("precedents[1].CaseName = %q, want Home Office v Dorset Yacht Co", precedents[1].CaseName)
	}
	if !strings.Contains(precedents[0].FullText, "HELD:") {
		t.Errorf("precedents[0].FullText missing expected content: %q", precedents[0].FullText)
	}
}

func TestDefaultLoader_Load_JSONArray(t *testing.T) {
	loader := NewDefaultLoader()
	precedents, err := loader.Load(context.Background(), strings.NewReader(syntheticJSONCorpus()))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(precedents) != 1 {
		t.Fatalf("len(precedents) = %d, want 1", len(precedents))
	}
	if precedents[0].CaseName != "R v Brown" {
		t.Errorf("CaseName = %q, want R v Brown", precedents[0].CaseName)
	}
}

func TestDefaultLoader_Load_JSONEnvelope(t *testing.T) {
	loader := NewDefaultLoader()
	input := `{"precedents": [{"case_name": "Test Case", "citation": "[2000] 1 X 1", "full_text": "HELD: test holding."}]}`
	precedents, err := loader.Load(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(precedents) != 1 || precedents[0].CaseName != "Test Case" {
		t.Fatalf("precedents = %+v, want one entry with CaseName=Test Case", precedents)
	}
}

func TestDefaultLoader_Load_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty input", ""},
		{"whitespace only", "   \n\t  "},
		{"invalid json array", "[not valid json"},
		{"invalid json envelope", "{not valid json"},
		{"empty json array", "[]"},
		{"no case headers", "just some prose with no CASE header at all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewDefaultLoader()
			_, err := loader.Load(context.Background(), strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("Load(%q) error = nil, want error", tt.input)
			}
		})
	}
}

func TestDefaultLoader_Load_NilSource(t *testing.T) {
	loader := NewDefaultLoader()
	_, err := loader.Load(context.Background(), nil)
	if err == nil {
		t.Fatal("Load(nil) error = nil, want error")
	}
}

func TestDefaultLoader_Load_CanceledContext(t *testing.T) {
	loader := NewDefaultLoader()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := loader.Load(ctx, strings.NewReader(syntheticTextCorpus))
	if err == nil {
		t.Fatal("Load() with canceled context, error = nil, want error")
	}
}
