// Package setup provides the first-run deployment provisioning wizard for the
// Verdex judicial reasoning platform.
//
// The setup wizard drives a tenant through a sequence of configuration steps
// required before the platform can handle judicial analysis requests:
//
//  1. Jurisdiction selection — choose the governing legal jurisdiction
//  2. Court level selection — set the court tier (e.g. supreme, appellate, trial)
//  3. Language selection    — pick supported reasoning languages
//  4. Provider configuration — configure the AI inference provider stub
//  5. Complete / Lock       — finalise and lock the wizard against re-run
//
// # State machine
//
// A [SetupWizard] moves through the following states in order:
//
//	pending → in_progress → jurisdiction_selected → court_selected →
//	language_selected → provider_configured → completed → locked
//
// Once a wizard reaches the "locked" terminal state it cannot be modified.
// Any attempt to apply further steps returns [ErrSetupLocked].
//
// # Entry points
//
// Use [SetupService] to drive the wizard from application code, and the
// HTTP handlers in handler.go for REST API access.
package setup
