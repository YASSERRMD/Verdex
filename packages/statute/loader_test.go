package statute

import (
	"context"
	"strings"
	"testing"
)

const syntheticTextCorpus = `ACT 12: Contracts Act
Section 1. Definitions
(a) "party" means a natural or legal person entering into a contract.
(b) "contract" means an agreement enforceable by law.
Section 2. Formation
A contract is formed when an offer is accepted. See Section 1 for
definitions.
Section 3. Breach
(a) A breach occurs when a party fails to perform. See Section 12(a)
for exceptions.

ACT 7: Sale of Goods Act
Section 1. Scope
This Act applies to contracts for the sale of goods, subject to Section 2 of the Contracts Act.
`

func syntheticJSONCorpus() string {
	return `[
		{"act_number": "12", "act_title": "Contracts Act", "body": "Section 1. Definitions\n(a) party means a person.\nSection 2. Formation\nA contract is formed by offer and acceptance."}
	]`
}

func TestDefaultLoader_Load_TextCorpus(t *testing.T) {
	loader := NewDefaultLoader()
	statutes, err := loader.Load(context.Background(), strings.NewReader(syntheticTextCorpus))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(statutes) != 2 {
		t.Fatalf("len(statutes) = %d, want 2", len(statutes))
	}
	if statutes[0].ActNumber != "12" || statutes[0].ActTitle != "Contracts Act" {
		t.Errorf("statutes[0] = %+v, want ActNumber=12 ActTitle=Contracts Act", statutes[0])
	}
	if statutes[1].ActNumber != "7" || statutes[1].ActTitle != "Sale of Goods Act" {
		t.Errorf("statutes[1] = %+v, want ActNumber=7 ActTitle=Sale of Goods Act", statutes[1])
	}
	if !strings.Contains(statutes[0].Body, "Section 1. Definitions") {
		t.Errorf("statutes[0].Body missing expected content: %q", statutes[0].Body)
	}
}

func TestDefaultLoader_Load_JSONArray(t *testing.T) {
	loader := NewDefaultLoader()
	statutes, err := loader.Load(context.Background(), strings.NewReader(syntheticJSONCorpus()))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(statutes) != 1 {
		t.Fatalf("len(statutes) = %d, want 1", len(statutes))
	}
	if statutes[0].ActNumber != "12" {
		t.Errorf("ActNumber = %q, want 12", statutes[0].ActNumber)
	}
}

func TestDefaultLoader_Load_JSONEnvelope(t *testing.T) {
	loader := NewDefaultLoader()
	input := `{"statutes": [{"act_number": "1", "act_title": "Test Act", "body": "Section 1. X\nSome text."}]}`
	statutes, err := loader.Load(context.Background(), strings.NewReader(input))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(statutes) != 1 || statutes[0].ActNumber != "1" {
		t.Fatalf("statutes = %+v, want one entry with ActNumber=1", statutes)
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
		{"no act headers", "just some prose with no ACT header at all"},
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
