package encryption

import (
	"fmt"
	"reflect"
)

// encryptedTag is the struct-tag name a caller uses to mark a field as
// sensitive and expected to hold Envelope-encrypted ciphertext at
// rest, consistent in spirit with packages/config's `redact:"true"`
// and packages/observability's `redact:"true"` conventions -- this is
// the `//encrypted` convention referenced throughout this package's
// documentation: any exported []byte or string field tagged
// `encrypted:"true"` is expected, once persisted, to contain
// Envelope-wrapped ciphertext rather than plaintext.
const encryptedTag = "encrypted"

// PlaintextFinding describes a single field that ScanForPlaintext
// determined still holds an unencrypted (non-Envelope) value despite
// being tagged `encrypted:"true"`.
type PlaintextFinding struct {
	// FieldPath is a dotted path to the offending field, e.g.
	// "Party.NationalID" for a nested struct.
	FieldPath string

	// Err explains why the field's value was judged not to be a valid
	// Envelope (see LooksLikeEnvelope).
	Err error
}

// ScanForPlaintext walks v (which must be a struct or a pointer to
// one) looking for exported string or []byte fields tagged
// `encrypted:"true"`, and reports every one whose current value does
// not look like a valid Envelope (per LooksLikeEnvelope) -- i.e.
// appears to still be plaintext. Nested structs are walked
// recursively so a field like Party.NationalID is found even though
// NationalID is not a direct field of the top-level type.
//
// This is a lint-style audit, not a guarantee: a value that happens to
// coincidentally match the Envelope magic/version framing would be
// (wrongly) treated as "encrypted," and a caller that stores an empty
// string in a sensitive field is not flagged (an empty value carries
// nothing to leak). Like packages/observability's redaction, treat
// this as defense-in-depth verification that a round-trip through
// Encrypt actually happened, not a substitute for encrypting sensitive
// fields correctly in the first place.
func ScanForPlaintext(v any) ([]PlaintextFinding, error) {
	if v == nil {
		return nil, fmt.Errorf("encryption: ScanForPlaintext: value is required")
	}
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("encryption: ScanForPlaintext: value must be a struct or pointer to struct, got %s", rv.Kind())
	}

	var findings []PlaintextFinding
	scanStruct(rv, "", &findings)
	return findings, nil
}

func scanStruct(v reflect.Value, prefix string, findings *[]PlaintextFinding) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := v.Field(i)
		path := field.Name
		if prefix != "" {
			path = prefix + "." + field.Name
		}

		tagged := field.Tag.Get(encryptedTag) == "true"

		switch fv.Kind() {
		case reflect.Struct:
			scanStruct(fv, path, findings)
			continue
		case reflect.Pointer:
			if !fv.IsNil() && fv.Elem().Kind() == reflect.Struct {
				scanStruct(fv.Elem(), path, findings)
				continue
			}
		default:
			// fall through to leaf handling below
		}

		if !tagged {
			continue
		}

		var raw []byte
		switch fv.Kind() {
		case reflect.String:
			s := fv.String()
			if s == "" {
				continue
			}
			raw = []byte(s)
		case reflect.Slice:
			if fv.Type().Elem().Kind() != reflect.Uint8 {
				continue
			}
			if fv.Len() == 0 {
				continue
			}
			raw = fv.Bytes()
		default:
			continue
		}

		if !LooksLikeEnvelope(raw) {
			*findings = append(*findings, PlaintextFinding{
				FieldPath: path,
				Err:       fmt.Errorf("%w: field %q", ErrPlaintextLeak, path),
			})
		}
	}
}
