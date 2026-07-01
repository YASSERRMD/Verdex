# Verdex Prompt Template Authoring Guide

This document explains how to write, register, and test prompt templates for
the Verdex judicial reasoning platform.

---

## 1. Template Anatomy

A `PromptTemplate` has these fields:

| Field             | Type             | Required | Description |
|-------------------|------------------|----------|-------------|
| `ID`              | `string`         | Yes      | Dot-separated identifier, e.g. `irac.issue.extraction` |
| `Name`            | `string`         | Yes      | Human-readable display name |
| `Version`         | `int`            | Yes      | Monotonically increasing (>= 1) |
| `Locale`          | `string`         | No       | BCP-47 tag (e.g. `en`, `ar`). Empty = wildcard |
| `LegalFamily`     | `string`         | No       | `common_law`, `civil_law`, `mixed`, `islamic_law`. Empty = wildcard |
| `Body`            | `string`         | Yes      | Go `text/template` source |
| `Variables`       | `[]VariableSpec` | No       | Declared variables |
| `NonBindingLabel` | `bool`           | No       | Append AI-output disclaimer when true |
| `CreatedAt`       | `time.Time`      | No       | Set automatically on registration |

---

## 2. Writing the Body

Template bodies use standard Go `text/template` syntax. Because variable
values are passed as a `map[string]string`, access them with the `index`
built-in:

```
{{index . "variable_name"}}
```

You may use template control structures (`if`, `range`, `with`) in the body,
but callers' variable values are sanitised so they cannot inject additional
template directives.

**Example:**

```
You are a legal analyst.

Jurisdiction: {{index . "jurisdiction_name"}}
Legal family: {{index . "legal_family"}}

Analyse the following case summary:
{{index . "case_summary"}}
```

---

## 3. Declaring Variables

Declare every variable the body references in the `Variables` slice:

```go
Variables: []prompts.VariableSpec{
    {Name: "case_summary",      Required: true,  Sanitize: true, MaxLen: 32000},
    {Name: "jurisdiction_name", Required: true,  Sanitize: true, MaxLen: 256},
    {Name: "optional_notes",    Required: false, Sanitize: true, MaxLen: 4096},
},
```

| Field      | Effect |
|------------|--------|
| `Required` | `Render` returns `ErrMissingVariable` when the value is absent or empty |
| `Sanitize` | The value is passed through `SanitizeValue` before injection (strips control chars; rejects `{{` / `}}`) |
| `MaxLen`   | Maximum UTF-8 character count; `0` = no limit |

Always set `Sanitize: true` for values that come from external sources.

---

## 4. Locale and Legal-Family Variants

Register multiple templates with the same `ID` but different `Locale` or
`LegalFamily` values to provide jurisdiction-specific or language-specific
variants.

```go
// Arabic civil-law variant
prompts.DefaultRegistry.Register(prompts.PromptTemplate{
    ID:          "irac.issue.extraction",
    Version:     1,
    Locale:      "ar",
    LegalFamily: "civil_law",
    Body:        `...Arabic / civil-law body...`,
    ...
})

// English common-law variant
prompts.DefaultRegistry.Register(prompts.PromptTemplate{
    ID:          "irac.issue.extraction",
    Version:     1,
    Locale:      "en",
    LegalFamily: "common_law",
    Body:        `...English / common-law body...`,
    ...
})

// Universal fallback (empty locale + legalFamily)
prompts.DefaultRegistry.Register(prompts.PromptTemplate{
    ID:      "irac.issue.extraction",
    Version: 1,
    Locale:  "",
    LegalFamily: "",
    Body:    `...fallback body...`,
    ...
})
```

`VariantSelector.SelectBest` resolves the best match using this fallback chain:

1. Exact locale **and** legal family
2. Exact locale, any legal family
3. Any locale, exact legal family
4. Any locale, any legal family (universal)

---

## 5. Versioning

Increment `Version` when making backwards-incompatible changes to a template
(changed variable names, changed output format). New versions coexist with
older ones in the registry; callers request a specific version with `Get` or
the latest with `Latest`/`VariantSelector.SelectBest`.

Do **not** modify an already-registered template in place — registering the
same `ID+Version+Locale+LegalFamily` key twice returns `ErrVersionConflict`.

---

## 6. Non-Binding Label

Set `NonBindingLabel: true` for templates whose output is legal analysis (not
advocacy). The renderer appends a standard disclaimer:

> DISCLAIMER: The above analysis is AI-generated legal reasoning … does NOT
> constitute legal advice … must not be presented as a legal opinion in any
> proceeding.

Mandatory for: `irac.issue.extraction`, `irac.synthesis`, and any template
that produces a reasoned opinion or summary.

Optional for: advocacy templates (`irac.argument.*`) where the output is
explicitly framed as one-sided argument.

---

## 7. Registering a Template

Place your template in `packages/prompts/templates/` and register it in an
`init()` function:

```go
package templates

import (
    "log"
    "github.com/YASSERRMD/verdex/packages/prompts"
)

func init() {
    t := prompts.PromptTemplate{ ... }
    if err := prompts.DefaultRegistry.Register(t); err != nil {
        log.Fatalf("prompts/templates: failed to register %s v%d: %v", t.ID, t.Version, err)
    }
}
```

Then import the package for side effects wherever templates are needed:

```go
import _ "github.com/YASSERRMD/verdex/packages/prompts/templates"
```

---

## 8. Testing Templates

Use `TestHarness` for lightweight template tests:

```go
func TestMyTemplate(t *testing.T) {
    h := prompts.TestHarness{}

    tmpl, _ := prompts.DefaultRegistry.Latest("my.template", "en", "common_law")

    // Positive test: check that expected substrings appear.
    err := h.RunCase(tmpl, map[string]string{
        "var_one": "value one",
        "var_two": "value two",
    }, []string{"value one", "DISCLAIMER"})
    if err != nil {
        t.Error(err)
    }

    // Negative test: confirm that injection attempts are rejected.
    failed, _ := h.FailsOn(tmpl, map[string]string{
        "var_one": "{{.Secret}}",
        "var_two": "value two",
    })
    if !failed {
        t.Error("expected injection to be blocked")
    }
}
```

---

## 9. Security Considerations

- **Never disable sanitisation** for variables sourced from user input,
  uploaded documents, or external APIs.
- `{{` and `}}` are blocked in all sanitised values to prevent prompt injection.
- Control characters (null bytes, form feeds, etc.) are silently stripped.
- `MaxLen` is enforced on rune count (not byte count) to prevent Unicode-based
  length-check bypasses.
- Template bodies may contain `if`/`range` logic written by template authors
  (trusted), but variable *values* supplied at render time are treated as
  untrusted data.

---

## 10. Naming Conventions

| Pattern | Meaning |
|---------|---------|
| `irac.*` | IRAC-method reasoning templates |
| `irac.issue.*` | Issue identification phase |
| `irac.argument.*` | Argument generation phase |
| `irac.synthesis` | Synthesis / draft opinion phase |
| `summary.*` | Case summarisation templates |
| `evidence.*` | Evidence analysis templates |

Use `kebab-case` segments joined by `.`, all lowercase. Avoid abbreviations
unless they are widely understood legal acronyms (IRAC, etc.).
