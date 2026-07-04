package threatmodel

import (
	"regexp"
	"strconv"
	"strings"
)

// This file is task 3's concrete detection/mitigation logic over
// ingested text before it reaches any LLM prompt. It operates one
// stage earlier than packages/prompts.SanitizeValue (Phase 016), which
// only rejects a template variable value once it is already being
// placed into a rendered prompt (its check is narrowly "does this
// value contain Go text/template delimiters"). DetectInjectionAttempt
// instead screens arbitrary ingested text -- a document, a
// transcript, an evidence excerpt -- for the broader family of
// prompt-injection patterns attackers actually use: role-override
// phrases ("ignore previous instructions"), instruction-injection
// markers (fake system/assistant turn markers), and delimiter-breaking
// sequences (attempts to escape whatever wrapper markup a caller uses
// to delimit untrusted content within a prompt). This package does not
// duplicate packages/prompts.SanitizeValue's delimiter check; the two
// compose (see doc.go).

// FindingKind classifies the category of suspicious pattern a Finding
// represents.
type FindingKind string

const (
	// FindingRoleOverride flags text attempting to instruct a model to
	// disregard its prior instructions or assume a different role/
	// persona (e.g. "ignore previous instructions", "you are now").
	FindingRoleOverride FindingKind = "role_override"

	// FindingInstructionMarker flags text containing a fake
	// system/assistant/user turn marker, an attempt to make injected
	// text masquerade as a new conversation turn rather than ingested
	// content (e.g. "system:", "[INST]", "### Instruction").
	FindingInstructionMarker FindingKind = "instruction_marker"

	// FindingDelimiterBreak flags text containing sequences commonly
	// used to escape a prompt's own content-delimiting markup (e.g.
	// closing an XML-style tag such as "</document>" or a triple-
	// backtick code fence, then continuing with new directives).
	FindingDelimiterBreak FindingKind = "delimiter_break"

	// FindingDataExfiltration flags text attempting to instruct a
	// model to reveal its system prompt, configuration, or other
	// content it should not disclose.
	FindingDataExfiltration FindingKind = "data_exfiltration"
)

// IsValid reports whether k is one of the named FindingKind constants.
func (k FindingKind) IsValid() bool {
	switch k {
	case FindingRoleOverride, FindingInstructionMarker, FindingDelimiterBreak, FindingDataExfiltration:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (k FindingKind) String() string { return string(k) }

// Finding is one suspicious pattern match DetectInjectionAttempt
// located within a scanned text.
type Finding struct {
	// Kind classifies what category of pattern matched.
	Kind FindingKind `json:"kind"`

	// Matched is the exact substring that triggered this Finding,
	// bounded to a short excerpt so a very long match cannot blow up a
	// log line or error message.
	Matched string `json:"matched"`

	// Index is the byte offset within the scanned text where Matched
	// begins, so a caller can locate and review the surrounding
	// context.
	Index int `json:"index"`
}

// maxFindingExcerpt bounds how much of a match Finding.Matched echoes
// back, mirroring packages/guardrail's truncateForError idiom for the
// same reason: a very long injected string should not blow up a log
// line or downstream error-wrapping chain.
const maxFindingExcerpt = 120

// injectionPattern pairs a compiled pattern with the FindingKind it
// signals.
type injectionPattern struct {
	kind FindingKind
	re   *regexp.Regexp
}

// injectionPatterns is the fixed, real pattern set
// DetectInjectionAttempt scans with. Patterns are deliberately
// case-insensitive ((?i)) and tolerant of common obfuscation via
// flexible whitespace (\s+), since an attacker will not conveniently
// use the exact casing/spacing a naive literal match expects. This is
// a denylist, not an exhaustive defense -- see doc/threat-model.md for
// why layered mitigations (this screen, packages/prompts.SanitizeValue,
// and packages/guardrail's output-side checks) matter more than any
// single pattern list.
var injectionPatterns = []injectionPattern{
	// Role-override phrases.
	{FindingRoleOverride, regexp.MustCompile(`(?i)ignore\s+(all\s+|the\s+)?(previous|prior|above|preceding)\s+(instructions?|prompts?|directives?)`)},
	{FindingRoleOverride, regexp.MustCompile(`(?i)disregard\s+(all\s+|the\s+)?(previous|prior|above)\s+(instructions?|prompts?)`)},
	{FindingRoleOverride, regexp.MustCompile(`(?i)you\s+are\s+now\s+[a-z]`)},
	{FindingRoleOverride, regexp.MustCompile(`(?i)forget\s+(everything|all)\s+(you\s+)?(know|were\s+told)`)},
	{FindingRoleOverride, regexp.MustCompile(`(?i)new\s+instructions?\s*:`)},
	{FindingRoleOverride, regexp.MustCompile(`(?i)act\s+as\s+(if\s+you\s+(are|were)\s+)?an?\s+[a-z]+\s+with\s+no\s+(restrictions|limitations|rules)`)},

	// Fake instruction/turn markers. The system: marker is anchored to
	// line-start ((?m)^), not just string-start: the realistic attack
	// injects a fake turn marker mid-document (e.g. as the first line
	// of an otherwise-legitimate-looking paragraph), not necessarily as
	// literally the first character of the entire ingested text.
	{FindingInstructionMarker, regexp.MustCompile(`(?im)^\s*system\s*:`)},
	{FindingInstructionMarker, regexp.MustCompile(`(?i)\[\s*(inst|system|/inst)\s*\]`)},
	{FindingInstructionMarker, regexp.MustCompile(`(?i)###\s*(instruction|system)\s*:?`)},
	{FindingInstructionMarker, regexp.MustCompile(`(?i)<\s*\|?\s*(system|assistant|im_start|im_end)\s*\|?\s*>`)},

	// Delimiter-breaking sequences.
	{FindingDelimiterBreak, regexp.MustCompile("```+\\s*(system|end|stop)")},
	{FindingDelimiterBreak, regexp.MustCompile(`</\s*(document|context|system|instructions?)\s*>`)},
	{FindingDelimiterBreak, regexp.MustCompile(`-{3,}\s*end\s+of\s+(document|context|instructions?)`)},

	// Data-exfiltration attempts.
	{FindingDataExfiltration, regexp.MustCompile(`(?i)(reveal|print|output|show|repeat)\s+(your\s+|the\s+)?(system\s+prompt|initial\s+instructions?|configuration)`)},
	{FindingDataExfiltration, regexp.MustCompile(`(?i)what\s+(is|are)\s+your\s+(system\s+prompt|instructions?|rules)`)},
}

// DetectInjectionAttempt scans text for known prompt-injection patterns
// -- role-override phrases, instruction-injection markers,
// delimiter-breaking sequences, and data-exfiltration attempts --
// before text is ever placed into an LLM prompt. It returns every
// Finding located (not just the first), so a caller can log or review
// the complete set rather than being told only "something matched".
// The bool return is true iff len(findings) > 0, provided so a caller
// that only needs a yes/no gate is not forced to check slice length
// itself.
func DetectInjectionAttempt(text string) (bool, []Finding) {
	findings := make([]Finding, 0)
	for _, p := range injectionPatterns {
		locs := p.re.FindAllStringIndex(text, -1)
		for _, loc := range locs {
			start, end := loc[0], loc[1]
			matched := text[start:end]
			findings = append(findings, Finding{
				Kind:    p.kind,
				Matched: truncateExcerpt(matched),
				Index:   start,
			})
		}
	}
	return len(findings) > 0, findings
}

// truncateExcerpt bounds s to maxFindingExcerpt runes, appending an
// ellipsis marker if truncated.
func truncateExcerpt(s string) string {
	r := []rune(s)
	if len(r) <= maxFindingExcerpt {
		return s
	}
	return string(r[:maxFindingExcerpt]) + "..."
}

// FindingsByKind groups findings by their Kind, convenience for a
// caller that wants counts or examples per category rather than a flat
// list.
func FindingsByKind(findings []Finding) map[FindingKind][]Finding {
	out := make(map[FindingKind][]Finding)
	for _, f := range findings {
		out[f.Kind] = append(out[f.Kind], f)
	}
	return out
}

// SummarizeFindings renders a short, human-readable one-line summary
// of findings, e.g. "3 findings: role_override=2, delimiter_break=1" --
// used by audit Detail strings and log lines that want a compact
// description rather than the full Finding slice.
func SummarizeFindings(findings []Finding) string {
	if len(findings) == 0 {
		return "no findings"
	}
	counts := FindingsByKind(findings)
	parts := make([]string, 0, len(counts))
	// Iterate the fixed known-kind order rather than map order, so the
	// summary is deterministic across calls with the same input.
	for _, kind := range []FindingKind{FindingRoleOverride, FindingInstructionMarker, FindingDelimiterBreak, FindingDataExfiltration} {
		if fs, ok := counts[kind]; ok {
			parts = append(parts, kind.String()+"="+strconv.Itoa(len(fs)))
		}
	}
	return strconv.Itoa(len(findings)) + " findings: " + strings.Join(parts, ", ")
}
