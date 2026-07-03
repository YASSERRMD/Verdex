// Package signoff implements Phase 068's mandatory human sign-off
// workflow: the first real, persisted implementation of
// packages/guardrail.SignoffGate.
//
// See doc/signoff-workflow.md for the full write-up, including a
// worked wiring example that swaps guardrail.NoSignoffRecordedGate for
// this package's GateImpl.
package signoff
