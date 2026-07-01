// Package prompts provides a versioned prompt template registry for the Verdex
// judicial reasoning platform.
//
// It supports jurisdiction-specific and language-specific variants of prompt
// templates used to drive LLM-based legal reasoning (IRAC analysis, argument
// generation, synthesis). Templates are registered with an ID, version number,
// locale tag, and legal-family tag; a VariantSelector resolves the best match
// using a configurable fallback chain.
//
// Safe variable injection is enforced: values are sanitised to strip control
// characters and block Go text/template syntax that could enable prompt-injection
// attacks. Templates marked NonBindingLabel automatically receive a disclaimer
// footer informing readers that the output is not legal advice.
//
// Typical usage:
//
//	import _ "github.com/YASSERRMD/verdex/packages/prompts/templates"
//
//	tmpl, err := prompts.DefaultRegistry.Latest("irac.issue.extraction", "en", "common_law")
//	rendered, err := prompts.Render(tmpl, map[string]string{
//	    "case_summary":      "...",
//	    "jurisdiction_name": "Dubai International Financial Centre",
//	    "legal_family":      "common_law",
//	})
package prompts
