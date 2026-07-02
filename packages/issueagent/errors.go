package issueagent

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilKnowledgeAPI is returned when the agent is constructed with a
	// nil *knowledgeapi.KnowledgeAPI.
	ErrNilKnowledgeAPI = errors.New("issueagent: knowledge api must not be nil")

	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("issueagent: case id is required")

	// ErrNoIssueNodes is returned when a case's tree has no IssueNodes to
	// frame. This is a legitimate outcome for a case whose tree has not
	// yet had issues extracted (packages/issue's job) rather than an
	// internal error, but the agent cannot produce a meaningful
	// IssueAnalysisResult without at least one IssueNode.
	ErrNoIssueNodes = errors.New("issueagent: case tree has no issue nodes to frame")

	// ErrMalformedModelOutput is returned when the model's response cannot
	// be parsed into the structured framing output this agent expects.
	ErrMalformedModelOutput = errors.New("issueagent: malformed model output")

	// ErrNoTemplate is returned when no issue-framing prompt template can
	// be resolved for the requested locale/legal family.
	ErrNoTemplate = errors.New("issueagent: no issue-framing template available")
)
