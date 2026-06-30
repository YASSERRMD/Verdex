package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestRedacted_StringNeverLeaks(t *testing.T) {
	r := Redact("super-secret-value")
	if r.String() != redactedPlaceholder {
		t.Errorf("String() = %q, want %q", r.String(), redactedPlaceholder)
	}
}

func TestRedacted_LogValueAppliedByLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(WithLevel(LevelDebug), WithFormat(FormatJSON), WithOutput(&buf))

	logger.Info(context.Background(), "user updated", "ssn", Redact("123-45-6789"))

	out := buf.String()
	if strings.Contains(out, "123-45-6789") {
		t.Fatalf("raw sensitive value leaked into log output: %s", out)
	}

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if record["ssn"] != redactedPlaceholder {
		t.Errorf("ssn = %v, want %q", record["ssn"], redactedPlaceholder)
	}
}

type fakeProfile struct {
	UserID   string
	Email    string `redact:"true"`
	APIToken string `redact:"true"`
	Nested   fakeNested
}

type fakeNested struct {
	Secret string `redact:"true"`
	Public string
}

func TestRedactStruct_RedactsTaggedFields(t *testing.T) {
	original := fakeProfile{
		UserID:   "u-1",
		Email:    "alice@example.com",
		APIToken: "abc123",
		Nested: fakeNested{
			Secret: "shh",
			Public: "visible",
		},
	}

	redactedAny := RedactStruct(original)
	redacted, ok := redactedAny.(fakeProfile)
	if !ok {
		t.Fatalf("RedactStruct returned unexpected type %T", redactedAny)
	}

	if redacted.UserID != "u-1" {
		t.Errorf("UserID = %q, want unchanged u-1", redacted.UserID)
	}
	if redacted.Email != redactedPlaceholder {
		t.Errorf("Email = %q, want %q", redacted.Email, redactedPlaceholder)
	}
	if redacted.APIToken != redactedPlaceholder {
		t.Errorf("APIToken = %q, want %q", redacted.APIToken, redactedPlaceholder)
	}
	if redacted.Nested.Secret != redactedPlaceholder {
		t.Errorf("Nested.Secret = %q, want %q", redacted.Nested.Secret, redactedPlaceholder)
	}
	if redacted.Nested.Public != "visible" {
		t.Errorf("Nested.Public = %q, want unchanged visible", redacted.Nested.Public)
	}

	// Original must be untouched.
	if original.Email != "alice@example.com" {
		t.Error("RedactStruct must not mutate the original value")
	}
}

func TestRedactStruct_NonStructPassthrough(t *testing.T) {
	got := RedactStruct(42)
	if got != 42 {
		t.Errorf("expected non-struct input to pass through unchanged, got %v", got)
	}
}

func TestRedactString_Email(t *testing.T) {
	in := "contact me at bob.smith@example.org for details"
	got := RedactString(in)
	if strings.Contains(got, "bob.smith@example.org") {
		t.Errorf("email was not redacted: %s", got)
	}
	if !strings.Contains(got, redactedPlaceholder) {
		t.Errorf("expected placeholder in output: %s", got)
	}
}

func TestRedactString_Credential(t *testing.T) {
	tests := []string{
		"api_key=sk-abcd1234efgh",
		"password: hunter2",
		"token=eyJhbGciOiJIUzI1NiJ9.payload.sig",
	}
	for _, in := range tests {
		got := RedactString(in)
		if !strings.Contains(got, redactedPlaceholder) {
			t.Errorf("RedactString(%q) = %q, expected placeholder", in, got)
		}
	}
}

func TestRedactString_LeavesOrdinaryTextAlone(t *testing.T) {
	in := "request completed in 42ms for case 1234"
	got := RedactString(in)
	if got != in {
		t.Errorf("expected ordinary text to be left alone, got %q", got)
	}
}
