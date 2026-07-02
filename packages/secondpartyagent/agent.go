package secondpartyagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// agentName identifies this agent for telemetry and error messages, per
// agentframework.Agent.Name's contract.
const agentName = "second-party-agent"

// defaultJurisdictionName is used for the prompt's JURISDICTION line when
// no WithJurisdictionName option is supplied.
const defaultJurisdictionName = "the relevant jurisdiction"

// defaultPartyLabel is used for the prompt's PARTY line when no
// WithPartyLabel option is supplied, falling back to a generic label
// rather than requiring a human-readable name for every PartyID.
const defaultPartyLabel = "the second party"

// Agent implements agentframework.Agent for second-party argument
// construction and rebuttal. Like packages/firstpartyagent's Agent, it is
// a single-step agent by design: BuildRequest gathers evidence for every
// issue once, and also folds in the first party's already-constructed
// ArgumentSet, rendering one argument-construction-and-rebuttal prompt
// covering every input issue; Interpret always concludes on the first
// model turn.
//
// This is the direct counterpart to packages/firstpartyagent's Agent —
// it builds the strongest good-faith case for the second party across
// the same framed issues, additionally targeting and rebutting specific
// first-party arguments. Interpret re-fetches the same issueEvidence
// slice BuildRequest computed rather than smuggling it through the
// Scratchpad, keeping Agent stateless and safe to share across
// concurrent Runner.Run calls for different cases — mirroring
// firstpartyagent.Agent's own documented convention exactly.
type Agent struct {
	api              *knowledgeapi.KnowledgeAPI
	partyID          PartyID
	registry         *prompts.Registry
	locale           string
	legalFamily      string
	jurisdictionCode string
	jurisdictionName string
	partyLabel       string
	issues           []issueagent.FramedIssue
	opposing         firstpartyagent.ArgumentSet
}

// Option configures an Agent constructed by New.
type Option func(*Agent)

// WithLocale sets the BCP-47 locale used for jurisdiction-aware template
// selection (see prompts.VariantSelector.SelectBest). Defaults to "".
func WithLocale(locale string) Option {
	return func(a *Agent) { a.locale = locale }
}

// WithLegalFamily sets the legal-family tag (e.g. "common_law",
// "civil_law") used for jurisdiction-aware template selection. Defaults
// to "".
func WithLegalFamily(legalFamily string) Option {
	return func(a *Agent) { a.legalFamily = legalFamily }
}

// WithJurisdictionName sets the human-readable jurisdiction name injected
// into the argument-construction prompt's JURISDICTION line. Defaults to
// defaultJurisdictionName when unset.
func WithJurisdictionName(name string) Option {
	return func(a *Agent) { a.jurisdictionName = name }
}

// WithJurisdictionCode sets the stable jurisdiction code (opaque string,
// mirroring irac.RuleNode.JurisdictionCode's convention) carried through
// unused by this agent's own output today but accepted for parity with
// firstpartyagent.WithJurisdictionCode and forward-compatibility with a
// future ArgumentSet field. Defaults to "".
func WithJurisdictionCode(code string) Option {
	return func(a *Agent) { a.jurisdictionCode = code }
}

// WithPartyLabel sets the human-readable label injected into the
// argument-construction prompt's PARTY line (e.g. "the defendant",
// "Acme Corp"). Defaults to defaultPartyLabel when unset.
func WithPartyLabel(label string) Option {
	return func(a *Agent) { a.partyLabel = label }
}

// WithRegistry overrides the prompts.Registry templates are resolved
// from. Defaults to prompts.DefaultRegistry, which this package's
// templates subpackage registers into via its init().
func WithRegistry(registry *prompts.Registry) Option {
	return func(a *Agent) {
		if registry != nil {
			a.registry = registry
		}
	}
}

// New constructs a second-party argument Agent scoped to api's case and
// partyID, arguing every issue in issues and rebutting opposing (the
// first party's already-constructed ArgumentSet — see
// packages/firstpartyagent). Returns ErrNilKnowledgeAPI if api is nil,
// ErrEmptyPartyID if partyID is empty, and ErrNoFramedIssues if issues is
// empty.
//
// opposing may legitimately be the zero value (no first-party arguments
// yet available, e.g. the first-party agent skipped every issue) — this
// agent still constructs the second party's own affirmative case in that
// case, simply with an empty rebuttal target set, rather than requiring
// a non-empty first-party ArgumentSet as a precondition.
func New(api *knowledgeapi.KnowledgeAPI, partyID PartyID, issues []issueagent.FramedIssue, opposing firstpartyagent.ArgumentSet, opts ...Option) (*Agent, error) {
	if api == nil {
		return nil, ErrNilKnowledgeAPI
	}
	if partyID == "" {
		return nil, ErrEmptyPartyID
	}
	if len(issues) == 0 {
		return nil, ErrNoFramedIssues
	}
	a := &Agent{
		api:              api,
		partyID:          partyID,
		issues:           issues,
		opposing:         opposing,
		registry:         prompts.DefaultRegistry,
		jurisdictionName: defaultJurisdictionName,
		partyLabel:       defaultPartyLabel,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a, nil
}

// Name implements agentframework.Agent.
func (a *Agent) Name() string { return agentName }

// TaskType implements agentframework.Agent, routing every model call as a
// structured reasoning task.
func (a *Agent) TaskType() provider.TaskType { return provider.TaskReason }

// BuildRequest implements agentframework.Agent. It fetches favorable
// fact/rule evidence for every input issue via knowledgeapi, resolves the
// jurisdiction-aware argument-rebuttal template, and renders the single
// prompt this agent ever sends — including the first party's opposing
// arguments as a rebuttal target list.
func (a *Agent) BuildRequest(ctx context.Context, pad *agentframework.Scratchpad) (provider.ChatRequest, error) {
	evidence, err := fetchIssueEvidence(ctx, a.api, pad.CaseID(), a.issues)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	tmpl, err := resolveTemplate(a.registry, a.locale, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	body, err := buildArgumentPrompt(tmpl, evidence, a.opposing.Arguments, a.jurisdictionName, a.legalFamily, a.partyLabel)
	if err != nil {
		return provider.ChatRequest{}, fmt.Errorf("secondpartyagent: render argument prompt: %w", err)
	}

	return provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: body}},
	}, nil
}

// Interpret implements agentframework.Agent. This agent always concludes
// on its first (and only) step: it parses the model's structured JSON
// argument response, grounds every proposed argument's cited node IDs
// against the case's actual tree and every proposed rebuttal target ID
// against the first party's real Argument IDs (rejecting fabrications of
// either kind, see ground.go), resolves citations and strength scores,
// and assembles the case's ArgumentSet, carried as Decision.FinalText
// (JSON-encoded) for the caller to unmarshal — see Argue, which is the
// sanctioned way to get a typed ArgumentSet back rather than parsing
// Result.FinalText by hand.
func (a *Agent) Interpret(ctx context.Context, pad *agentframework.Scratchpad, resp *provider.ChatResponse) (agentframework.Decision, error) {
	evidence, err := fetchIssueEvidence(ctx, a.api, pad.CaseID(), a.issues)
	if err != nil {
		return agentframework.Decision{}, err
	}

	modelResp, err := parseModelArgumentResponse(resp.Content)
	if err != nil {
		return agentframework.Decision{}, err
	}

	result := assembleArgumentSet(ctx, a.api, pad.CaseID(), a.partyID, evidence, a.opposing.Arguments, modelResp)

	encoded, err := encodeResult(result)
	if err != nil {
		return agentframework.Decision{}, fmt.Errorf("secondpartyagent: encode result: %w", err)
	}

	return agentframework.Decision{Conclude: true, FinalText: encoded}, nil
}
