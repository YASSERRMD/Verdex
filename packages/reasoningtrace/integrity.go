package reasoningtrace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// IntegrityHash computes a SHA-256 hash, hex-encoded, over trace's
// canonical JSON representation (encoding/json.Marshal's deterministic
// field order for a struct, so the same Trace value always hashes
// identically regardless of caller). This is a tamper-EVIDENCE
// mechanism — see doc/reasoning-trace.md's "limits" section — not a
// cryptographic signature chain: it detects that a stored Trace was
// mutated after this hash was recorded, the same way
// packages/provenance's content hash detects a mutated artifact, but it
// does not by itself prove who produced the original value or when.
func IntegrityHash(trace Trace) string {
	// Trace.Steps/Retrievals carry `error` interface fields, which have
	// no MarshalJSON and would marshal unpredictably (as {} for most
	// concrete error types). canonicalTrace renders those fields as
	// plain strings first so the hashed representation is well-defined
	// and stable across calls, independent of the concrete error type.
	b, err := json.Marshal(canonicalTrace(trace))
	if err != nil {
		// Unreachable given canonicalTraceView's all-JSON-safe field
		// types, but handled rather than ignored per this project's
		// error handling conventions.
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// VerifyIntegrity reports whether trace's current IntegrityHash matches
// expectedHash — i.e. whether trace has been mutated since expectedHash
// was recorded for it.
func VerifyIntegrity(trace Trace, expectedHash string) bool {
	return IntegrityHash(trace) == expectedHash
}

// canonicalStep is StageStep with Err rendered as a string, so the
// canonical JSON view does not depend on the unspecified marshaling of
// an `error` interface value.
type canonicalStep struct {
	Stage         string
	Index         int
	ModelCalled   bool
	Concluded     bool
	ToolCallCount int
	Err           string
	Duration      int64
}

// canonicalRetrieval is RetrievalEvent with Err rendered as a string, for
// the same reason as canonicalStep.
type canonicalRetrieval struct {
	Stage         string
	StepIndex     int
	ToolName      string
	Args          map[string]any
	ResultSummary string
	Err           string
}

// canonicalTraceView is the JSON-safe shape IntegrityHash actually
// hashes: identical information to Trace, with every `error` field
// rendered as its message string so the hash is well-defined and stable.
type canonicalTraceView struct {
	CaseID          string
	Steps           []canonicalStep
	Retrievals      []canonicalRetrieval
	Narrative       string
	Segments        []NarrativeSegment
	AuthorityTrails []AuthorityTrail
	GeneratedAt     int64
}

// canonicalTrace converts trace into its canonicalTraceView.
func canonicalTrace(trace Trace) canonicalTraceView {
	steps := make([]canonicalStep, 0, len(trace.Steps))
	for _, s := range trace.Steps {
		errText := ""
		if s.Err != nil {
			errText = s.Err.Error()
		}
		steps = append(steps, canonicalStep{
			Stage:         string(s.Stage),
			Index:         s.Index,
			ModelCalled:   s.ModelCalled,
			Concluded:     s.Concluded,
			ToolCallCount: s.ToolCallCount,
			Err:           errText,
			Duration:      int64(s.Duration),
		})
	}

	retrievals := make([]canonicalRetrieval, 0, len(trace.Retrievals))
	for _, r := range trace.Retrievals {
		errText := ""
		if r.Err != nil {
			errText = r.Err.Error()
		}
		retrievals = append(retrievals, canonicalRetrieval{
			Stage:         string(r.Stage),
			StepIndex:     r.StepIndex,
			ToolName:      r.ToolName,
			Args:          r.Args,
			ResultSummary: r.ResultSummary,
			Err:           errText,
		})
	}

	return canonicalTraceView{
		CaseID:          trace.CaseID,
		Steps:           steps,
		Retrievals:      retrievals,
		Narrative:       trace.Narrative,
		Segments:        trace.Segments,
		AuthorityTrails: trace.AuthorityTrails,
		GeneratedAt:     trace.GeneratedAt.UnixNano(),
	}
}
