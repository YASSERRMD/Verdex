package config

import (
	"encoding/json"
	"reflect"
)

// redactedPlaceholder replaces the value of any field tagged
// `redact:"true"` when producing a redacted copy of a Config.
const redactedPlaceholder = "[REDACTED]"

// Redacted returns a copy of c with every field tagged `redact:"true"`
// replaced by a fixed placeholder. Use the returned value (not c
// itself) anywhere a Config might be logged, printed, or otherwise
// surfaced outside the process, so secrets never reach logs.
//
// Only string fields are supported by the redact tag today, since
// that covers every sensitive field in Config (e.g. database.dsn,
// which may hold a literal credential or an already-resolved secret).
func (c Config) Redacted() Config {
	redactStruct(reflect.ValueOf(&c).Elem())
	return c
}

// String implements fmt.Stringer by rendering a redacted, JSON-encoded
// view of c. This makes it safe to pass a Config directly to a logger
// or Printf("%s", cfg) without leaking secrets.
func (c Config) String() string {
	redacted := c.Redacted()
	data, err := json.Marshal(redacted)
	if err != nil {
		// json.Marshal on this struct tree cannot fail in practice
		// (no channels/funcs/cyclic types), but never panic from
		// String().
		return "config.Config{<marshal error: " + err.Error() + ">}"
	}
	return string(data)
}

// redactStruct walks v (a struct, addressable) in place and blanks out
// any string field tagged `redact:"true"`.
func redactStruct(v reflect.Value) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldValue := v.Field(i)

		if fieldValue.Kind() == reflect.Struct {
			redactStruct(fieldValue)
			continue
		}

		if field.Tag.Get("redact") == "true" && fieldValue.Kind() == reflect.String && fieldValue.String() != "" {
			fieldValue.SetString(redactedPlaceholder)
		}
	}
}
