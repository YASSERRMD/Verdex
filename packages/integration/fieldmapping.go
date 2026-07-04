package integration

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// FieldRule is one field-level translation between this platform's
// field name and an external system's field name (task 4). Source and
// Target are always expressed from Apply's point of view: Source names
// the external system's field, Target names this platform's field
// (packages/caselifecycle.Case/party/document field names, referenced
// by convention only -- see doc.go). Reverse swaps the direction.
type FieldRule struct {
	// SourceField is the external system's field name (a key into
	// InboundCase.Fields, or the key MappedFields.Reverse produces for
	// OutboundReport.Metadata).
	SourceField string `json:"source_field"`

	// TargetField is this platform's field name (e.g. "title",
	// "reference", "jurisdiction_id", "party.name",
	// "document.filename").
	TargetField string `json:"target_field"`

	// Required marks TargetField as mandatory: Apply returns
	// ErrUnmappedField if SourceField is absent or blank in the input
	// record.
	Required bool `json:"required"`

	// DefaultValue is used for TargetField when SourceField is absent
	// from the input record and Required is false. Ignored when
	// Required is true (a required field with no value is an error,
	// never silently defaulted).
	DefaultValue string `json:"default_value,omitempty"`
}

// Validate checks r for structural well-formedness.
func (r FieldRule) Validate() error {
	if strings.TrimSpace(r.SourceField) == "" || strings.TrimSpace(r.TargetField) == "" {
		return wrapf("FieldRule.Validate", ErrInvalidFieldMapping)
	}
	return nil
}

// FieldMapping is a tenant's configurable mapping between this
// platform's case/party/document fields and one external system's
// schema (task 4): an ordered set of FieldRule entries plus enough
// identity to be looked up by a ConnectorConfig.FieldMappingID.
type FieldMapping struct {
	// ID uniquely identifies this mapping.
	ID uuid.UUID `json:"id"`

	// TenantID scopes this mapping to a tenant.
	TenantID uuid.UUID `json:"tenant_id"`

	// ConnectorType names the connector this mapping was authored for
	// (e.g. "efiling-dubai-courts"), for discoverability -- a tenant
	// may register more than one FieldMapping across different
	// connector types.
	ConnectorType string `json:"connector_type"`

	// Name is a short human-readable label (e.g. "Dubai Courts case
	// import mapping v2").
	Name string `json:"name"`

	// Rules is the ordered set of field-level translations this
	// mapping applies. Order matters only for MappedFields.Values'
	// iteration determinism in tests; Apply/Reverse do not depend on
	// rule order for correctness.
	Rules []FieldRule `json:"rules"`

	// CreatedBy is the identity.User who authored this mapping.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks m for structural well-formedness: a non-nil
// TenantID, non-blank ConnectorType/Name, and every Rule must itself
// validate.
func (m *FieldMapping) Validate() error {
	if m == nil {
		return ErrInvalidFieldMapping
	}
	if m.TenantID == uuid.Nil {
		return wrapf("FieldMapping.Validate", ErrEmptyTenantID)
	}
	if strings.TrimSpace(m.ConnectorType) == "" {
		return wrapf("FieldMapping.Validate", ErrInvalidFieldMapping)
	}
	if strings.TrimSpace(m.Name) == "" {
		return wrapf("FieldMapping.Validate", ErrInvalidFieldMapping)
	}
	for _, r := range m.Rules {
		if err := r.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// MappedFields is the result of applying a FieldMapping to an external
// record: a set of values addressed by this platform's field names,
// ready for a caller to assign onto a packages/caselifecycle.Case (or
// its parties/documents) by field name.
type MappedFields struct {
	// Values holds TargetField -> resolved value pairs.
	Values map[string]string `json:"values"`

	// UnmappedSourceFields lists external-record field names present
	// in the input that no Rule's SourceField claimed -- surfaced so a
	// caller (or the reconciliation pass) can detect a stale/incomplete
	// FieldMapping rather than silently dropping data.
	UnmappedSourceFields []string `json:"unmapped_source_fields,omitempty"`
}

// Apply translates an external system's raw field record (typically
// InboundCase.Fields) into MappedFields addressed by this platform's
// field names, per m's Rules -- a real translation, not a passthrough.
// Returns ErrUnmappedField if any Required rule's SourceField is
// absent or blank in record.
func (m *FieldMapping) Apply(record map[string]string) (MappedFields, error) {
	if m == nil {
		return MappedFields{}, ErrInvalidFieldMapping
	}
	if err := m.Validate(); err != nil {
		return MappedFields{}, err
	}

	claimed := make(map[string]bool, len(m.Rules))
	values := make(map[string]string, len(m.Rules))
	for _, rule := range m.Rules {
		claimed[rule.SourceField] = true
		val, ok := record[rule.SourceField]
		val = strings.TrimSpace(val)
		if !ok || val == "" {
			if rule.Required {
				return MappedFields{}, wrapf("FieldMapping.Apply", ErrUnmappedField)
			}
			val = rule.DefaultValue
		}
		values[rule.TargetField] = val
	}

	unmapped := make([]string, 0)
	for src := range record {
		if !claimed[src] {
			unmapped = append(unmapped, src)
		}
	}

	return MappedFields{Values: values, UnmappedSourceFields: unmapped}, nil
}

// Reverse translates this platform's field values (keyed by
// TargetField, e.g. drawn from a packages/caselifecycle.Case /
// packages/reportexport.Report by field name) back into the external
// system's own field names, for use in OutboundReport.Metadata or a
// re-export back to the source system. Reverse is the mirror image of
// Apply: it walks the same Rules, but reads TargetField and writes
// SourceField.
func (m *FieldMapping) Reverse(values map[string]string) (map[string]string, error) {
	if m == nil {
		return nil, ErrInvalidFieldMapping
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(m.Rules))
	for _, rule := range m.Rules {
		val, ok := values[rule.TargetField]
		val = strings.TrimSpace(val)
		if !ok || val == "" {
			if rule.Required {
				return nil, wrapf("FieldMapping.Reverse", ErrUnmappedField)
			}
			val = rule.DefaultValue
		}
		out[rule.SourceField] = val
	}
	return out, nil
}
