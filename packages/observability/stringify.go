package observability

import "fmt"

// stringify renders an arbitrary value as a string using its default
// fmt verb. Used as a fallback when an Attribute or log field's value
// is not one of the small set of types handled natively.
func stringify(v any) string {
	return fmt.Sprintf("%v", v)
}
