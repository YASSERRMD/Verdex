package agentframework

import (
	"sync"
	"time"

	"github.com/YASSERRMD/verdex/packages/provider"
)

// Observation records the outcome of a single ToolCall made during a
// Step, paired with the call that produced it.
type Observation struct {
	Call   ToolCall
	Result ToolResult

	// Err is set when the tool invocation failed. Result is the zero
	// value in that case.
	Err error
}

// Step is one iteration of an agent run: the model request/response for
// that iteration, any tool calls the Agent's Interpret decided to make,
// and the resulting Observations. Steps accumulate on a Scratchpad in
// the order they occurred.
type Step struct {
	// Index is this step's 0-based position within the run.
	Index int

	// Request is the provider.ChatRequest sent for this step.
	Request provider.ChatRequest

	// Response is the provider.ChatResponse received for this step. Nil
	// if the model call failed (see Err).
	Response *provider.ChatResponse

	// Decision is the Agent's interpretation of Response. Zero value if
	// the model call failed before Interpret was reached.
	Decision Decision

	// Observations holds the result of every ToolCall dispatched for
	// this step, in call order.
	Observations []Observation

	// Err holds any error encountered during this step (model call,
	// tool invocation, or malformed output). Nil for a clean step.
	Err error

	// StartedAt and EndedAt bound this step's wall-clock duration.
	StartedAt time.Time
	EndedAt   time.Time
}

// Duration returns EndedAt minus StartedAt.
func (s Step) Duration() time.Duration {
	return s.EndedAt.Sub(s.StartedAt)
}

// Scratchpad is a CaseID-scoped, append-only record of every Step an
// Agent took during one Runner.Run call, plus arbitrary free-form notes
// an Agent may want to leave for itself between steps (e.g. a running
// summary it re-injects into the next BuildRequest instead of replaying
// the full Step history).
//
// A Scratchpad is not shared across cases: it is constructed fresh per
// Run and keyed by the same CaseID every knowledgeisolation-aware
// package in Verdex uses, so a future orchestration pipeline (Phase 059)
// that holds several agents' Scratchpads in memory at once can never
// confuse one case's intermediate reasoning with another's.
//
// Safe for concurrent use, though a single Runner.Run drives one
// Scratchpad sequentially; concurrent access is supported so a caller can
// inspect an in-progress run's Scratchpad from another goroutine (e.g.
// for a live progress UI).
type Scratchpad struct {
	mu       sync.RWMutex
	caseID   string
	steps    []Step
	notes    map[string]string
	toolLog  []Observation // flattened, cross-step observation history
	tenantID string
}

// NewScratchpad returns an empty Scratchpad scoped to caseID for tenantID.
// tenantID may be empty if the caller's routing layer does not need
// per-tenant distinction (Runner passes it straight through to
// router.Router.Chat/Embed's tenantID argument). Returns ErrEmptyCaseID
// if caseID is empty.
func NewScratchpad(caseID, tenantID string) (*Scratchpad, error) {
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	return &Scratchpad{
		caseID:   caseID,
		tenantID: tenantID,
		notes:    make(map[string]string),
	}, nil
}

// CaseID returns the case this Scratchpad is scoped to.
func (p *Scratchpad) CaseID() string {
	return p.caseID
}

// TenantID returns the tenant this Scratchpad's run is billed/routed
// under.
func (p *Scratchpad) TenantID() string {
	return p.tenantID
}

// AppendStep adds a completed Step to the run history and flattens its
// Observations into the cross-step tool log.
func (p *Scratchpad) AppendStep(s Step) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.steps = append(p.steps, s)
	p.toolLog = append(p.toolLog, s.Observations...)
}

// Steps returns a copy of the step history so far, in order.
func (p *Scratchpad) Steps() []Step {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Step, len(p.steps))
	copy(out, p.steps)
	return out
}

// StepCount returns the number of steps recorded so far.
func (p *Scratchpad) StepCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.steps)
}

// Observations returns every Observation recorded across every step so
// far, in call order.
func (p *Scratchpad) Observations() []Observation {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]Observation, len(p.toolLog))
	copy(out, p.toolLog)
	return out
}

// SetNote stores a free-form note under key, overwriting any previous
// value. Intended for an Agent to carry forward a running summary or
// intermediate conclusion between steps without replaying the full Step
// history into every BuildRequest call.
func (p *Scratchpad) SetNote(key, value string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notes[key] = value
}

// Note returns the note stored under key, and whether one was set.
func (p *Scratchpad) Note(key string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	v, ok := p.notes[key]
	return v, ok
}

// Notes returns a copy of every note currently stored.
func (p *Scratchpad) Notes() map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make(map[string]string, len(p.notes))
	for k, v := range p.notes {
		out[k] = v
	}
	return out
}
