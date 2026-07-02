package synthesisagent

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilKnowledgeAPI is returned when the agent is constructed with a
	// nil *knowledgeapi.KnowledgeAPI.
	ErrNilKnowledgeAPI = errors.New("synthesisagent: knowledge api must not be nil")

	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("synthesisagent: case id is required")

	// ErrNoFramedIssues is returned when the supplied
	// issueagent.IssueAnalysisResult has no issues to synthesize.
	ErrNoFramedIssues = errors.New("synthesisagent: no framed issues to synthesize")

	// ErrMalformedModelOutput is returned when the model's response cannot
	// be parsed into the structured synthesis output this agent expects.
	ErrMalformedModelOutput = errors.New("synthesisagent: malformed model output")

	// ErrNoTemplate is returned when no synthesis prompt template can be
	// resolved for the requested locale/legal family.
	ErrNoTemplate = errors.New("synthesisagent: no synthesis template available")

	// ErrNoGroundedConclusions is returned when every conclusion the model
	// proposed failed the anti-fabrication grounding check, leaving no
	// valid TentativeConclusion for any issue.
	ErrNoGroundedConclusions = errors.New("synthesisagent: no conclusions survived grounding validation")

	// ErrVerdictLanguage is returned by the ConclusionProvider adapter when
	// a TentativeConclusion's Text contains verdict or directive language
	// (per irac.ContainsVerdictLanguage) and is rejected rather than
	// converted into an irac.ConclusionNode.
	ErrVerdictLanguage = errors.New("synthesisagent: conclusion text contains verdict language")
)
