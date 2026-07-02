package secondpartyagent

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilKnowledgeAPI is returned when the agent is constructed with a
	// nil *knowledgeapi.KnowledgeAPI.
	ErrNilKnowledgeAPI = errors.New("secondpartyagent: knowledge api must not be nil")

	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("secondpartyagent: case id is required")

	// ErrEmptyPartyID is returned when the agent is constructed without a
	// PartyID identifying which party it argues for.
	ErrEmptyPartyID = errors.New("secondpartyagent: party id is required")

	// ErrNoFramedIssues is returned when the supplied
	// issueagent.IssueAnalysisResult has no issues to argue.
	ErrNoFramedIssues = errors.New("secondpartyagent: no framed issues to argue")

	// ErrMalformedModelOutput is returned when the model's response cannot
	// be parsed into the structured argument output this agent expects.
	ErrMalformedModelOutput = errors.New("secondpartyagent: malformed model output")

	// ErrNoTemplate is returned when no argument-construction prompt
	// template can be resolved for the requested locale/legal family.
	ErrNoTemplate = errors.New("secondpartyagent: no argument-construction template available")

	// ErrNoGroundedArguments is returned when every argument the model
	// proposed failed the anti-fabrication grounding check, leaving an
	// issue with zero valid arguments after grounding.
	ErrNoGroundedArguments = errors.New("secondpartyagent: no arguments survived grounding validation")
)
