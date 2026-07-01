package gateway

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
)

// FieldError describes a validation failure for a single struct field.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationError aggregates one or more field-level validation failures.
type ValidationError struct {
	Fields []FieldError
}

// Error satisfies the error interface.
func (e *ValidationError) Error() string {
	msgs := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		msgs = append(msgs, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return "validation error: " + strings.Join(msgs, "; ")
}

// Details returns a string slice suitable for use in ErrWithDetails.
func (e *ValidationError) Details() []string {
	out := make([]string, 0, len(e.Fields))
	for _, f := range e.Fields {
		out = append(out, fmt.Sprintf("%s: %s", f.Field, f.Message))
	}
	return out
}

// ValidateRequest decodes the JSON body of r into dst and performs basic
// struct validation (required fields must be non-zero).
//
// On decode failure it returns an *APIError with code BAD_REQUEST.
// On validation failure it returns a *ValidationError.
func ValidateRequest[T any](r *http.Request, dst *T) error {
	if r.Body == nil {
		return &APIError{
			Code:    ErrCodeBadRequest,
			Message: "request body is required",
		}
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return &APIError{
			Code:    ErrCodeBadRequest,
			Message: fmt.Sprintf("invalid JSON: %s", err.Error()),
			Err:     err,
		}
	}

	if err := validateStruct(dst); err != nil {
		return err
	}

	return nil
}

// validateStruct inspects fields tagged with `validate:"required"` and returns
// a *ValidationError if any required field is the zero value for its type.
func validateStruct(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}

	rt := rv.Type()
	var fieldErrors []FieldError

	for i := range rt.NumField() {
		field := rt.Field(i)
		fval := rv.Field(i)

		tag := field.Tag.Get("validate")
		if tag == "" {
			continue
		}

		rules := strings.Split(tag, ",")
		for _, rule := range rules {
			rule = strings.TrimSpace(rule)
			switch rule {
			case "required":
				if fval.IsZero() {
					name := jsonFieldName(field)
					fieldErrors = append(fieldErrors, FieldError{
						Field:   name,
						Message: "is required",
					})
				}
			}
		}
	}

	if len(fieldErrors) > 0 {
		return &ValidationError{Fields: fieldErrors}
	}
	return nil
}

// jsonFieldName returns the JSON key name for a struct field, falling back to
// the field name when no json tag is present.
func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	parts := strings.SplitN(tag, ",", 2)
	if parts[0] == "" || parts[0] == "-" {
		return f.Name
	}
	return parts[0]
}

// WriteValidationError writes a BAD_REQUEST response that includes field-level
// validation details. It accepts either a *ValidationError or a generic error.
func WriteValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		ErrWithDetails(w, r, &APIError{
			Code:    ErrCodeBadRequest,
			Message: "request validation failed",
		}, ve.Details())
		return
	}
	WriteErrorWithRequest(w, r, err)
}
