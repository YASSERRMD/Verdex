package prompts

import "errors"

// ErrTemplateNotFound is returned when no template matching the requested
// ID, version, locale, and legal family exists in the registry.
var ErrTemplateNotFound = errors.New("prompts: template not found")

// ErrMissingVariable is returned by Render when a required template variable
// is absent from the supplied vars map.
var ErrMissingVariable = errors.New("prompts: missing required variable")

// ErrVariableTooLong is returned by Render (and SanitizeValue) when a variable
// value exceeds the VariableSpec.MaxLen limit.
var ErrVariableTooLong = errors.New("prompts: variable value exceeds maximum length")

// ErrInjectionAttempt is returned when a variable value contains Go
// text/template syntax ({{ or }}) that could enable prompt-injection.
var ErrInjectionAttempt = errors.New("prompts: injection attempt detected in variable value")

// ErrInvalidTemplate is returned when a template fails structural validation
// (e.g. missing ID, unparseable body, duplicate VariableSpec names).
var ErrInvalidTemplate = errors.New("prompts: invalid template")

// ErrVersionConflict is returned when a template with the same
// ID+Version+Locale+LegalFamily combination is registered more than once.
var ErrVersionConflict = errors.New("prompts: version conflict — template already registered")
