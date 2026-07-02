package firstpartyagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// agentName identifies this agent for telemetry and error messages, per
// agentframework.Agent.Name's contract.
const agentName = "first-party-agent"

// defaultJurisdictionName is used for the prompt's JURISDICTION line when
// no WithJurisdictionName option is supplied.
const defaultJurisdictionName = "the relevant jurisdiction"

// defaultPartyLabel is used for the prompt's PARTY line when no
// WithPartyLabel option is supplied, falling back to a generic label
// rather than requiring a human-readable name for every PartyID.
const defaultPartyLabel = "the first party"

// Agent implements agentframework.Agent for first-party argument
// construction. Like packages/issueagent's Agent, it is a single-step
// agent by design: BuildRequest gathers every issue's evidence once via
// knowledgeapi and renders one argument-construction prompt covering
// every input issue, and Interpret always concludes on the first model
// turn. Argument construction, like issue framing, benefits from seeing
// the whole evidence set up front rather than exploring incrementally;
// a future revision wanting iterative exploration (e.g. following up a
// weak argument with a targeted knowledgeapi.Retrieve call) is a change
// to Interpret's Decision, not a different framework.
//
// Interpret re-fetches the same issueEvidence slice BuildRequest computed
// rather than smuggling it through the Scratchpad, keeping Agent
// stateless and safe to share across concurrent Runner.Run calls for
// different cases — mirroring issueagent.Agent's own documented
// convention exactly.
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
// issueagent.WithJurisdictionCode and forward-compatibility with a future
// ArgumentSet field. Defaults to "".
func WithJurisdictionCode(code string) Option {
	return func(a *Agent) { a.jurisdictionCode = code }
}

// WithPartyLabel sets the human-readable label injected into the
// argument-construction prompt's PARTY line (e.g. "the plaintiff",
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

// New constructs a first-party argument Agent scoped to api's case and
// partyID, arguing every issue in issues. Returns ErrNilKnowledgeAPI if
// api is nil, ErrEmptyPartyID if partyID is empty, and ErrNoFramedIssues
// if issues is empty.
func New(api *knowledgeapi.KnowledgeAPI, partyID PartyID, issues []issueagent.FramedIssue, opts ...Option) (*Agent, error) {
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
// jurisdiction-aware argument-construction template, and renders the
// single prompt this agent ever sends.
func (a *Agent) BuildRequest(ctx context.Context, pad *agentframework.Scratchpad) (provider.ChatRequest, error) {
	evidence, err := fetchIssueEvidence(ctx, a.api, pad.CaseID(), a.issues)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	tmpl, err := resolveTemplate(a.registry, a.locale, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	body, err := buildArgumentPrompt(tmpl, evidence, a.jurisdictionName, a.legalFamily, a.partyLabel)
	if err != nil {
		return provider.ChatRequest{}, fmt.Errorf("firstpartyagent: render argument prompt: %w", err)
	}

	return provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: body}},
	}, nil
}

// Interpret implements agentframework.Agent. This agent always concludes
// on its first (and only) step: it parses the model's structured JSON
// argument response, grounds every proposed argument's cited node IDs
// against the case's actual tree (rejecting fabrications, see ground.go),
// resolves citations and strength scores, and assembles the case's
// ArgumentSet, carried as Decision.FinalText (JSON-encoded) for the
// caller to unmarshal — see Argue, which is the sanctioned way to get a
// typed ArgumentSet back rather than parsing Result.FinalText by hand.
func (a *Agent) Interpret(ctx context.Context, pad *agentframework.Scratchpad, resp *provider.ChatResponse) (agentframework.Decision, error) {
	evidence, err := fetchIssueEvidence(ctx, a.api, pad.CaseID(), a.issues)
	if err != nil {
		return agentframework.Decision{}, err
	}

	modelResp, err := parseModelArgumentResponse(resp.Content)
	if err != nil {
		return agentframework.Decision{}, err
	}

	result := assembleArgumentSet(ctx, a.api, pad.CaseID(), a.partyID, evidence, modelResp)

	encoded, err := encodeResult(result)
	if err != nil {
		return agentframework.Decision{}, fmt.Errorf("firstpartyagent: encode result: %w", err)
	}

	return agentframework.Decision{Conclude: true, FinalText: encoded}, nil
}
