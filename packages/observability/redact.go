// Redaction is a defense-in-depth safety net, not a substitute for
// simply not logging PII in the first place. Treat every mechanism in
// this file as a best-effort backstop: it catches obvious mistakes
// (an email address or credential-shaped string landing in a log
// field) but cannot guarantee it catches everything, and callers
// remain responsible for not passing sensitive values to a Logger.
package observability

import (
	"log/slog"
	"reflect"
	"regexp"
)

// redactedPlaceholder replaces the value of any field or pattern match
// flagged for redaction.
const redactedPlaceholder = "[REDACTED]"

// Redact marks value as sensitive so it is replaced by a fixed
// placeholder when passed through RedactFields or RedactStruct, rather
// than being logged in the clear. Use this for an individual
// structured log field whose value is sensitive but whose key name
// alone isn't a reliable enough signal to redact automatically, e.g.:
//
//	logger.Info(ctx, "user updated", "user_id", id, "ssn", observability.Redact(ssn))
type Redacted struct {
	value string
}

// Redact wraps value, marking it for redaction wherever this package's
// redaction helpers process it.
func Redact(value string) Redacted {
	return Redacted{value: value}
}

// String implements fmt.Stringer by returning the fixed placeholder,
// never the wrapped value, so an accidental %v/%s format of a Redacted
// (bypassing the field-aware helpers below) still never leaks the
// underlying string.
func (r Redacted) String() string {
	return redactedPlaceholder
}

// LogValue implements log/slog's slog.LogValuer, so a Redacted value
// passed directly as a slog field argument is automatically rendered
// as the placeholder by any slog handler, including this package's
// Logger.
func (r Redacted) LogValue() slog.Value {
	return slog.StringValue(redactedPlaceholder)
}

// redactTag is the struct-tag name used to mark a field as sensitive,
// consistent in spirit with packages/config's `redact:"true"`
// convention.
const redactTag = "redact"

// RedactStruct returns a redaction-safe shallow copy of v (which must
// be a struct or a pointer to one): every exported string field tagged
// `redact:"true"` is replaced with the fixed placeholder. Nested
// structs are walked recursively. Use this to safely log a struct that
// carries some sensitive fields (e.g. a request DTO with a password
// field) without hand-writing a redacted copy.
func RedactStruct(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return v
	}

	copied := reflect.New(rv.Type()).Elem()
	copied.Set(rv)
	redactStructValue(copied)
	return copied.Interface()
}

func redactStructValue(v reflect.Value) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := v.Field(i)

		switch fv.Kind() {
		case reflect.Struct:
			redactStructValue(fv)
		case reflect.String:
			if field.Tag.Get(redactTag) == "true" && fv.String() != "" {
				fv.SetString(redactedPlaceholder)
			}
		default:
			// Other kinds never carry the redact tag meaningfully in
			// this package's convention; left untouched.
		}
	}
}

// Built-in pattern-based redactors. These are a best-effort,
// defense-in-depth layer applied by RedactString to catch obviously
// sensitive-looking values (an email address, a credential-shaped
// token) that were passed to a log field without an explicit Redact()
// wrapper or struct tag. They are intentionally conservative pattern
// matches, not a PII detection system.
var (
	emailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)

	// credentialPattern matches common "key=value"-shaped credential
	// hints, e.g. "api_key=abcd1234", "password: hunter2",
	// "token=eyJhbGciOi...". It is intentionally narrow to avoid
	// false-positiving on ordinary prose.
	credentialPattern = regexp.MustCompile(`(?i)\b(api[_-]?key|password|passwd|secret|token|access[_-]?key)\b\s*[:=]\s*\S+`)
)

// RedactString applies the built-in pattern-based redactors to s,
// replacing any email-looking or credential-looking substrings with
// the fixed placeholder. It is a last-resort safety net for free-text
// log messages and field values; prefer not logging sensitive data,
// or using Redact()/struct tags for fields known in advance to be
// sensitive.
func RedactString(s string) string {
	s = emailPattern.ReplaceAllString(s, redactedPlaceholder)
	s = credentialPattern.ReplaceAllString(s, redactedPlaceholder)
	return s
}
