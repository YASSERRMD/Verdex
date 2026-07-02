package synthesisagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/evidenceweighing"
	"github.com/YASSERRMD/verdex/packages/firstpartyagent"
	"github.com/YASSERRMD/verdex/packages/issueagent"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/lawapplication"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/provider"
	"github.com/YASSERRMD/verdex/packages/secondpartyagent"
)

// agentName identifies this agent for telemetry and error messages, per
// agentframework.Agent.Name's contract.
const agentName = "synthesis-agent"

// defaultJurisdictionName is used for the prompt's JURISDICTION line when
// no WithJurisdictionName option is supplied.
const defaultJurisdictionName = "the relevant jurisdiction"

// Agent implements agentframework.Agent for reasoned-opinion synthesis.
// Like packages/issueagent's and packages/firstpartyagent's Agents, it is
// a single-step agent by design: BuildRequest gathers every issue's
// arguments, evidence weights, and law application once via
// knowledgeapi/the upstream Results supplied to New, and renders one
// synthesis prompt covering every input issue; Interpret always concludes
// on the first model turn.
//
// Interpret re-fetches the same []issueSynthesisInput BuildRequest
// computed rather than smuggling it through the Scratchpad, keeping
// Agent stateless and safe to share across concurrent Runner.Run calls
// for different cases — mirroring firstpartyagent.Agent's own documented
// convention exactly.
type Agent struct {
	api              *knowledgeapi.KnowledgeAPI
	registry         *prompts.Registry
	locale           string
	legalFamily      string
	jurisdictionCode string
	jurisdictionName string
	issues           []issueagent.FramedIssue
	firstParty       firstpartyagent.ArgumentSet
	secondParty      secondpartyagent.ArgumentSet
	evidence         evidenceweighing.Result
	lawApplication   lawapplication.Result
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
// into the synthesis prompt's JURISDICTION line. Defaults to
// defaultJurisdictionName when unset.
func WithJurisdictionName(name string) Option {
	return func(a *Agent) { a.jurisdictionName = name }
}

// WithJurisdictionCode sets the stable jurisdiction code (opaque string,
// mirroring irac.RuleNode.JurisdictionCode's convention), carried through
// unused by this agent's own output today but accepted for parity with
// firstpartyagent.WithJurisdictionCode. Defaults to "".
func WithJurisdictionCode(code string) Option {
	return func(a *Agent) { a.jurisdictionCode = code }
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

// New constructs a synthesis Agent scoped to api's case, resolving every
// issue in issues by weighing firstParty's and secondParty's arguments,
// evidence's fact weights, and lawApp's issue applications. Returns
// ErrNilKnowledgeAPI if api is nil, and ErrNoFramedIssues if issues is
// empty.
//
// Either ArgumentSet may be its zero value if that party produced no
// arguments; synthesis still proceeds against whichever ArgumentSet is
// non-empty, mirroring lawapplication.Request's own tolerance for a
// one-sided record.
func New(
	api *knowledgeapi.KnowledgeAPI,
	issues []issueagent.FramedIssue,
	firstParty firstpartyagent.ArgumentSet,
	secondParty secondpartyagent.ArgumentSet,
	evidence evidenceweighing.Result,
	lawApp lawapplication.Result,
	opts ...Option,
) (*Agent, error) {
	if api == nil {
		return nil, ErrNilKnowledgeAPI
	}
	if len(issues) == 0 {
		return nil, ErrNoFramedIssues
	}
	a := &Agent{
		api:              api,
		issues:           issues,
		firstParty:       firstParty,
		secondParty:      secondParty,
		evidence:         evidence,
		lawApplication:   lawApp,
		registry:         prompts.DefaultRegistry,
		jurisdictionName: defaultJurisdictionName,
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

// BuildRequest implements agentframework.Agent. It fetches per-issue
// arguments/evidence/law-application context via knowledgeapi and the
// Results supplied to New, resolves the jurisdiction-aware
// opinion-synthesis template, and renders the single prompt this agent
// ever sends.
func (a *Agent) BuildRequest(ctx context.Context, pad *agentframework.Scratchpad) (provider.ChatRequest, error) {
	inputs, err := fetchSynthesisInputs(ctx, a.api, pad.CaseID(), a.issues, a.firstParty, a.secondParty, a.evidence, a.lawApplication)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	tmpl, err := resolveTemplate(a.registry, a.locale, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	body, err := buildSynthesisPrompt(tmpl, inputs, a.jurisdictionName, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, fmt.Errorf("synthesisagent: render synthesis prompt: %w", err)
	}

	return provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: body}},
	}, nil
}

// Interpret implements agentframework.Agent. This agent always concludes
// on its first (and only) step: it parses the model's structured JSON
// synthesis response, grounds every proposed conclusion's cited node IDs
// against the case's actual tree (rejecting fabrications, see ground.go),
// derives each conclusion's weakest link, and assembles the case's
// Opinion, carried as Decision.FinalText (JSON-encoded) for the caller to
// unmarshal — see Synthesize, which is the sanctioned way to get a typed
// Opinion back rather than parsing Result.FinalText by hand.
func (a *Agent) Interpret(ctx context.Context, pad *agentframework.Scratchpad, resp *provider.ChatResponse) (agentframework.Decision, error) {
	inputs, err := fetchSynthesisInputs(ctx, a.api, pad.CaseID(), a.issues, a.firstParty, a.secondParty, a.evidence, a.lawApplication)
	if err != nil {
		return agentframework.Decision{}, err
	}

	modelResp, err := parseModelSynthesisResponse(resp.Content)
	if err != nil {
		return agentframework.Decision{}, err
	}

	result := assembleOpinion(pad.CaseID(), inputs, modelResp)

	encoded, err := encodeResult(result)
	if err != nil {
		return agentframework.Decision{}, fmt.Errorf("synthesisagent: encode result: %w", err)
	}

	return agentframework.Decision{Conclude: true, FinalText: encoded}, nil
}
