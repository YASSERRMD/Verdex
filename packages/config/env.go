package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// EnvPrefix is the prefix applied to every environment variable
// recognized by this package. A field's env var name is built by
// joining EnvPrefix with the upper-cased, underscore-separated path of
// struct field names down to that field, e.g. Config.Server.Port becomes
// "VERDEX_SERVER_PORT".
const EnvPrefix = "VERDEX_"

// envFieldName returns the name segment used for a struct field when
// building its environment variable name. It honors an explicit `env`
// struct tag if present (a tag of "-" means "not settable via env");
// otherwise it converts the Go field name (e.g. "ReadTimeout") to
// SCREAMING_SNAKE_CASE (e.g. "READ_TIMEOUT").
func envFieldName(f reflect.StructField) (name string, skip bool) {
	if tag, ok := f.Tag.Lookup("env"); ok {
		if tag == "-" {
			return "", true
		}
		return strings.ToUpper(tag), false
	}
	return toScreamingSnakeCase(f.Name), false
}

// toScreamingSnakeCase converts a Go exported identifier such as
// "ReadTimeout" or "DSN" into SCREAMING_SNAKE_CASE such as
// "READ_TIMEOUT" or "DSN". A new word boundary is inserted before an
// upper-case letter that follows a lower-case letter or digit, and
// before the last upper-case letter of a run that is followed by a
// lower-case letter (so "DSNValue" becomes "DSN_VALUE").
func toScreamingSnakeCase(s string) string {
	runes := []rune(s)
	var b strings.Builder

	for i, r := range runes {
		isUpper := r >= 'A' && r <= 'Z'
		if isUpper && i > 0 {
			prev := runes[i-1]
			prevIsLowerOrDigit := (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9')
			nextIsLower := i+1 < len(runes) && runes[i+1] >= 'a' && runes[i+1] <= 'z'
			prevIsUpper := prev >= 'A' && prev <= 'Z'

			if prevIsLowerOrDigit || (prevIsUpper && nextIsLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(r)
	}

	return strings.ToUpper(b.String())
}

// loadEnv overlays environment variables onto cfg, mutating it in
// place. Only fields whose corresponding environment variable is
// actually set are overwritten, so unset fields retain whatever value
// a lower-precedence layer already assigned.
func loadEnv(cfg *Config) error {
	return loadEnvStruct(reflect.ValueOf(cfg).Elem(), EnvPrefix)
}

// loadEnvStruct walks v (which must be a struct) and, for each field,
// either recurses into nested structs (joining the env var prefix) or
// attempts to set the field from the corresponding environment
// variable.
func loadEnvStruct(v reflect.Value, prefix string) error {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		name, skip := envFieldName(field)
		if skip {
			continue
		}

		fieldValue := v.Field(i)
		envName := prefix + name

		if fieldValue.Kind() == reflect.Struct && fieldValue.Type() != reflect.TypeOf(time.Duration(0)) {
			if err := loadEnvStruct(fieldValue, envName+"_"); err != nil {
				return err
			}
			continue
		}

		raw, ok := os.LookupEnv(envName)
		if !ok {
			continue
		}

		if err := setFieldFromString(fieldValue, raw); err != nil {
			return fmt.Errorf("config: env var %s: %w", envName, err)
		}
	}

	return nil
}

// setFieldFromString parses raw and assigns it to field according to
// field's kind/type. Supported kinds: string, int family, bool, and
// time.Duration.
func setFieldFromString(field reflect.Value, raw string) error {
	switch {
	case field.Type() == reflect.TypeOf(time.Duration(0)):
		d, err := time.ParseDuration(raw)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", raw, err)
		}
		field.SetInt(int64(d))
		return nil

	case field.Kind() == reflect.String:
		field.SetString(raw)
		return nil

	case field.Kind() >= reflect.Int && field.Kind() <= reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q: %w", raw, err)
		}
		field.SetInt(n)
		return nil

	case field.Kind() == reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid boolean %q: %w", raw, err)
		}
		field.SetBool(b)
		return nil

	default:
		return fmt.Errorf("unsupported field kind %s for value %q", field.Kind(), raw)
	}
}
