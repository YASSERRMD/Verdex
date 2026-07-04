package scalability

import (
	"strings"
	"time"
)

// ChecklistAnswers is a service owner's self-attestation against the
// StatelessnessContract (tasks 1 and 3): what a service must NOT do to
// remain safely horizontally scalable. Every field defaults to the
// zero value (false), which Verify treats as "not yet attested" rather
// than "compliant" -- a service that has never filled out the
// checklist must fail Verify, not silently pass it.
type ChecklistAnswers struct {
	// NoInProcessSessionAffinity attests that no request handling in
	// this service depends on being routed back to the same process
	// instance (no in-memory session store, no sticky-session
	// assumption). A load balancer must be free to send consecutive
	// requests from the same caller to any replica.
	NoInProcessSessionAffinity bool

	// NoLocalDiskOnlyState attests that any state the service writes to
	// local disk (temp files, caches, embedded databases) is either
	// safely reconstructible from a durable store or is purely
	// ephemeral/discardable -- a replica may be killed and a fresh one
	// started with no local disk carried over.
	NoLocalDiskOnlyState bool

	// NoInMemorySingletonState attests that no long-lived, mutable,
	// process-global state (counters, caches with no external
	// invalidation, in-memory queues holding unacknowledged work) would
	// silently lose data if the process were replaced. Ephemeral
	// request-scoped state is fine; anything a caller depends on
	// surviving a restart is not.
	NoInMemorySingletonState bool

	// IdempotentRetries attests that every externally-triggered
	// operation this service performs is safe to retry (the caller, a
	// load balancer, or an orchestrator may resend a request after a
	// timeout without a local-affinity guarantee that the retry lands
	// on the same replica that received the original attempt).
	IdempotentRetries bool

	// ExternalizedConfiguration attests that runtime configuration
	// comes from the layered file/env mechanism in packages/config (or
	// an equivalent externalized source), not from a local file only
	// one replica has been manually given.
	ExternalizedConfiguration bool

	// HealthCheckExposed attests that the service exposes a liveness/
	// readiness signal an orchestrator (or the autoscaling machinery in
	// policy.go) can poll to decide whether a given replica should
	// receive traffic.
	HealthCheckExposed bool

	// GracefulShutdownHandled attests that the service drains in-flight
	// work and stops accepting new work on SIGTERM/shutdown signal
	// within a bounded window, rather than dropping requests
	// mid-flight when a replica is scaled down.
	GracefulShutdownHandled bool

	// Notes is free-text context a service owner can attach to explain
	// any answer above (e.g. why a particular local cache is safe).
	// Never evaluated by Verify -- purely for a human reviewer.
	Notes string
}

// totalChecks is the number of boolean attestations ChecklistAnswers
// carries. Kept in one place so Report's Score and Verify's pass/fail
// threshold stay consistent if a field is ever added.
const totalChecks = 7

// checks returns the boolean fields of a in a fixed, named order, so
// Verify and Report can iterate them without repeating the field list.
func (a ChecklistAnswers) checks() []checklistItem {
	return []checklistItem{
		{Name: "NoInProcessSessionAffinity", Passed: a.NoInProcessSessionAffinity},
		{Name: "NoLocalDiskOnlyState", Passed: a.NoLocalDiskOnlyState},
		{Name: "NoInMemorySingletonState", Passed: a.NoInMemorySingletonState},
		{Name: "IdempotentRetries", Passed: a.IdempotentRetries},
		{Name: "ExternalizedConfiguration", Passed: a.ExternalizedConfiguration},
		{Name: "HealthCheckExposed", Passed: a.HealthCheckExposed},
		{Name: "GracefulShutdownHandled", Passed: a.GracefulShutdownHandled},
	}
}

// checklistItem is a single named attestation and whether it passed.
type checklistItem struct {
	Name   string
	Passed bool
}

// Report is the outcome of running Contract.Verify for one service.
type Report struct {
	// ServiceName is the service this report was computed for.
	ServiceName string

	// Failed lists the names of every ChecklistAnswers field that was
	// false (not yet attested), in the fixed order checks() defines.
	// Empty means every attestation passed.
	Failed []string

	// Score is the fraction of the totalChecks attestations that
	// passed, in [0, 1].
	Score float64

	// Passed is true only when every attestation passed (Score == 1),
	// mirroring perf.Verdict's "overall pass requires every dimension
	// to pass" convention.
	Passed bool

	// EvaluatedAt is when Verify computed this report.
	EvaluatedAt time.Time
}

// Contract is the StatelessnessContract (also referred to as the
// "StatelessnessChecklist" in the phase brief): a documented,
// automatable architectural contract a service must satisfy to remain
// safely horizontally scalable (tasks 1 and 3 -- "make services
// horizontally scalable" and "add stateless service guarantees" are
// the same underlying contract, not two separate mechanisms). Contract
// carries no per-service state itself; Verify is a pure function of
// its inputs so it can be called freely from tests or a CI gate.
type Contract struct {
	// clock is overridable for deterministic tests; nil means
	// time.Now.
	clock func() time.Time
}

// NewContract returns a ready-to-use Contract.
func NewContract() *Contract {
	return &Contract{clock: time.Now}
}

func (c *Contract) now() time.Time {
	if c != nil && c.clock != nil {
		return c.clock().UTC()
	}
	return time.Now().UTC()
}

// Verify evaluates answers for serviceName and returns a Report with
// real pass/fail logic -- not prose, and not a stub that always
// reports passing. Returns ErrEmptyServiceName if serviceName is
// blank.
func (c *Contract) Verify(serviceName string, answers ChecklistAnswers) (Report, error) {
	if strings.TrimSpace(serviceName) == "" {
		return Report{}, wrapf("Verify", ErrEmptyServiceName)
	}

	var failed []string
	passedCount := 0
	for _, item := range answers.checks() {
		if item.Passed {
			passedCount++
		} else {
			failed = append(failed, item.Name)
		}
	}

	return Report{
		ServiceName: serviceName,
		Failed:      failed,
		Score:       float64(passedCount) / float64(totalChecks),
		Passed:      len(failed) == 0,
		EvaluatedAt: c.now(),
	}, nil
}
