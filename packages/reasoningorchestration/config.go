package reasoningorchestration

import (
	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/reasoningprofile"
	"github.com/YASSERRMD/verdex/packages/router"
)

// PartyConfig bundles the per-party identifiers/labels Run needs to
// construct firstpartyagent.Agent/secondpartyagent.Agent. The two
// parties are configured independently (rather than a single "Parties
// [2]PartyConfig" array) since the first and second party have distinct
// downstream types (firstpartyagent.PartyID vs secondpartyagent.PartyID)
// and the second party additionally rebuts the first.
type PartyConfig struct {
	// ID is the party's opaque identifier, threaded into
	// firstpartyagent.New/secondpartyagent.New.
	ID string

	// Label is the human-readable label injected into that party's
	// argument-construction prompt (see firstpartyagent.WithPartyLabel /
	// secondpartyagent.WithPartyLabel). Optional.
	Label string
}

// RunConfig bundles every dependency Run needs to drive one case through
// the full pipeline: the KnowledgeAPI the tree-reading stages fetch
// through, the Router every LLM-agent stage dispatches through, the two
// parties' identifiers, jurisdiction context, the CheckpointStore
// persistence is written to, and the budgets/guardrail gate bounding the
// run.
//
// RunConfig deliberately does not accept a provider.LLMProvider directly
// — Router is the only sanctioned path to a model call, mirroring
// agentframework.Config's own model-agnostic-by-construction rule.
type RunConfig struct {
	// API is the KnowledgeAPI every tree-reading stage (issue framing,
	// argument construction, law application's rule/fact catalog fetch)
	// reads through. Required.
	API *knowledgeapi.KnowledgeAPI

	// Router is the *router.Router every LLM-agent stage's model call is
	// dispatched through. Required.
	Router *router.Router

	// FirstParty and SecondParty configure the two adversarial agents.
	// Both IDs are required.
	FirstParty  PartyConfig
	SecondParty PartyConfig

	// Locale is the BCP-47 locale used for jurisdiction-aware template
	// selection across every LLM-agent stage. Optional.
	Locale string

	// LegalFamily is the legal-family tag (e.g. "common_law") used for
	// jurisdiction-aware template selection and for
	// evidenceweighing/lawapplication's own family-keyed profiles.
	// Optional; when empty, each stage's own zero-value default applies.
	LegalFamily string

	// JurisdictionName is the human-readable jurisdiction name injected
	// into every LLM-agent stage's prompt. Optional.
	JurisdictionName string

	// JurisdictionCode is the stable jurisdiction code carried through to
	// every LLM-agent stage's WithJurisdictionCode option. Optional.
	JurisdictionCode string

	// Weights is the reasoningprofile.Weights resolved for this case's
	// legal family (typically via reasoningprofile.ResolveFamily plus
	// reasoningprofile.WeightsForFamily, run concurrently with issue
	// framing — see doc/reasoning-orchestration.md's concurrency
	// section). Zero value is valid: it is informational context for a
	// future caller wanting to record which profile governed a run, and
	// does not itself change any stage's computation, since
	// evidenceweighing/lawapplication key their own profiles off
	// LegalFamily directly rather than accepting a Weights value.
	Weights reasoningprofile.Weights

	// Checkpoints persists every stage's typed result. Required; use
	// NewInMemoryCheckpointStore for a default in-memory implementation.
	Checkpoints CheckpointStore

	// SignoffGate is consulted at StageGuardrailCheck via
	// guardrail.CanFinalize. Defaults to guardrail.NoSignoffRecordedGate
	// (fail-closed) when nil.
	SignoffGate guardrail.SignoffGate

	// Budget bounds the whole run. Zero value uses PipelineBudget's own
	// defaults (see PipelineBudget.withDefaults).
	Budget PipelineBudget

	// TenantID is passed through to every LLM-agent stage's Config.TenantID
	// and to the underlying router.Router.Chat calls.
	TenantID string

	// Seed configures deterministic-mode metadata for every LLM-agent
	// stage's model call. Zero value (disabled) is valid.
	Seed agentframework.Seed
}

// validate returns ErrNilRunConfig if any required dependency is
// missing, and ErrEmptyCaseID-adjacent field errors for missing party
// IDs.
func (c RunConfig) validate() error {
	if c.API == nil || c.Router == nil || c.Checkpoints == nil {
		return ErrNilRunConfig
	}
	if c.FirstParty.ID == "" || c.SecondParty.ID == "" {
		return ErrNilRunConfig
	}
	return nil
}
