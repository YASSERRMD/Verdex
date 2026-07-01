package prompts

import "time"

// VariableSpec describes a single template variable: whether it is required,
// whether its value should be sanitised before injection, and the maximum
// permitted length (0 means no limit).
type VariableSpec struct {
	// Name is the variable name referenced in the template body as {{.Name}}.
	Name string

	// Required causes Render to return ErrMissingVariable when the caller
	// does not supply a value for this variable.
	Required bool

	// Sanitize causes Render to pass the value through SanitizeValue before
	// substitution, stripping control characters and blocking template syntax.
	Sanitize bool

	// MaxLen is the maximum number of UTF-8 characters allowed in the value.
	// A value of 0 disables the length check.
	MaxLen int
}

// PromptTemplate is an immutable, versioned LLM prompt template keyed by ID,
// version, locale, and legal family.
//
// The Body is a valid Go text/template string. Variable placeholders use the
// standard {{.VarName}} syntax. Template logic (range, if, etc.) is allowed
// in the body; however, values supplied by callers are sanitised to prevent
// injection of additional template directives.
type PromptTemplate struct {
	// ID is a dot-separated, human-readable identifier (e.g. "irac.issue.extraction").
	ID string

	// Name is a short, human-readable display name for the template.
	Name string

	// Version is a monotonically increasing integer. Higher numbers supersede
	// lower ones for the same ID+Locale+LegalFamily combination.
	Version int

	// Locale is a BCP-47 language tag (e.g. "en", "ar", "fr"). An empty string
	// is treated as a wildcard that matches any locale.
	Locale string

	// LegalFamily classifies the legal tradition targeted by this template
	// (e.g. "common_law", "civil_law"). An empty string matches any family.
	LegalFamily string

	// Body is the raw Go text/template source. Use {{.VarName}} to reference
	// variables declared in Variables.
	Body string

	// Variables declares the variables that Render accepts.
	Variables []VariableSpec

	// NonBindingLabel, when true, causes Render to append a standard disclaimer
	// stating that the output is AI-generated legal analysis, not legal advice.
	NonBindingLabel bool

	// CreatedAt records when this template version was registered.
	CreatedAt time.Time
}
