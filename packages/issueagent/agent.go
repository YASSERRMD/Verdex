package issueagent

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/agentframework"
	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
	"github.com/YASSERRMD/verdex/packages/prompts"
	"github.com/YASSERRMD/verdex/packages/provider"
)

// agentName identifies this agent for telemetry and error messages, per
// agentframework.Agent.Name's contract.
const agentName = "issue-agent"

// defaultJurisdictionName is used for the prompt's JURISDICTION line when
// no WithJurisdictionName option is supplied.
const defaultJurisdictionName = "the relevant jurisdiction"

// Agent implements agentframework.Agent for issue framing. It is a
// single-step agent by design: BuildRequest gathers every IssueNode and
// its governing rules once via knowledgeapi, renders one framing prompt
// for the whole case, and Interpret always concludes on the first model
// turn — there is no iterative tool-calling loop, because framing reads
// the whole tree up front rather than exploring it incrementally. This
// keeps the agent's Budget usage minimal (one step, one model call) while
// still fitting agentframework.Runner's general step-loop contract, so a
// future revision that does want iterative exploration (e.g. following up
// on an ambiguous issue with a targeted knowledgeapi tool call) is a
// change to Interpret's Decision, not a different framework.
//
// Interpret re-fetches the same issueContext slice BuildRequest computed
// (a cheap, in-memory-indexed knowledgeapi.GetTree call, see fetch.go)
// rather than smuggling it through the Scratchpad or a field on Agent, so
// Agent stays exactly the stateless, concurrency-safe shape
// agentframework.Agent's interface doc comment expects — safe to share
// across concurrent Runner.Run calls for different cases.
type Agent struct {
	api              *knowledgeapi.KnowledgeAPI
	registry         *prompts.Registry
	locale           string
	legalFamily      string
	jurisdictionCode string
	jurisdictionName string
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
// into the framing prompt's JURISDICTION line. Defaults to
// defaultJurisdictionName when unset.
func WithJurisdictionName(name string) Option {
	return func(a *Agent) { a.jurisdictionName = name }
}

// WithJurisdictionCode sets the stable jurisdiction code (opaque string,
// mirroring irac.RuleNode.JurisdictionCode's convention — no hard
// dependency on packages/jurisdiction) carried on the resulting
// IssueAnalysisResult.JurisdictionCode. Distinct from
// WithJurisdictionName, which only affects prompt wording. Defaults to
// "".
func WithJurisdictionCode(code string) Option {
	return func(a *Agent) { a.jurisdictionCode = code }
}

// WithRegistry overrides the prompts.Registry templates are resolved
// from. Defaults to prompts.DefaultRegistry, which this package's
// templates subpackage registers into via its init() (see prompt.go's
// import of issueagent/templates).
func WithRegistry(registry *prompts.Registry) Option {
	return func(a *Agent) {
		if registry != nil {
			a.registry = registry
		}
	}
}

// New constructs an issue-framing Agent scoped to api's case. Returns
// ErrNilKnowledgeAPI if api is nil.
func New(api *knowledgeapi.KnowledgeAPI, opts ...Option) (*Agent, error) {
	if api == nil {
		return nil, ErrNilKnowledgeAPI
	}
	a := &Agent{
		api:              api,
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

// BuildRequest implements agentframework.Agent. It fetches every
// IssueNode (and governing rules) for the case via knowledgeapi, resolves
// the jurisdiction-aware framing template, and renders the single prompt
// this agent ever sends.
func (a *Agent) BuildRequest(ctx context.Context, pad *agentframework.Scratchpad) (provider.ChatRequest, error) {
	contexts, err := fetchIssueContexts(ctx, a.api, pad.CaseID())
	if err != nil {
		return provider.ChatRequest{}, err
	}

	tmpl, err := resolveTemplate(a.registry, a.locale, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, err
	}

	body, err := buildFramingPrompt(tmpl, contexts, a.jurisdictionName, a.legalFamily)
	if err != nil {
		return provider.ChatRequest{}, fmt.Errorf("issueagent: render framing prompt: %w", err)
	}

	return provider.ChatRequest{
		Messages: []provider.Message{{Role: "user", Content: body}},
	}, nil
}

// Interpret implements agentframework.Agent. This agent always concludes
// on its first (and only) step: it parses the model's structured JSON
// framing response, assembles the case's IssueAnalysisResult, and carries
// it as Decision.FinalText (JSON-encoded) for the caller to unmarshal —
// see Analyze, which is the sanctioned way to get a typed
// IssueAnalysisResult back rather than parsing Result.FinalText by hand.
func (a *Agent) Interpret(ctx context.Context, pad *agentframework.Scratchpad, resp *provider.ChatResponse) (agentframework.Decision, error) {
	contexts, err := fetchIssueContexts(ctx, a.api, pad.CaseID())
	if err != nil {
		return agentframework.Decision{}, err
	}

	modelResp, err := parseModelFramingResponse(resp.Content)
	if err != nil {
		return agentframework.Decision{}, err
	}

	result := assembleResult(pad.CaseID(), a.jurisdictionCode, a.legalFamily, contexts, modelResp)

	encoded, err := encodeResult(result)
	if err != nil {
		return agentframework.Decision{}, fmt.Errorf("issueagent: encode result: %w", err)
	}

	return agentframework.Decision{Conclude: true, FinalText: encoded}, nil
}
