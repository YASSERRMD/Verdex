package casesearch

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Mode selects which content-matching strategy Search uses within each
// candidate case. See doc.go, "Search modes".
type Mode string

const (
	// ModeAuto picks ModeIssueRule when Query.IssueOrRuleID is set, and
	// ModeSemantic otherwise. This is the default when Query.Mode is left
	// blank.
	ModeAuto Mode = ""

	// ModeKeyword matches Query.Text as a plain, case-insensitive
	// substring/term search against node text and case metadata (title,
	// reference). No embeddings, no LLM call.
	ModeKeyword Mode = "keyword"

	// ModeSemantic embeds Query.Text and delegates to the resolved
	// CaseSearcher's vector/hybrid retrieval (packages/hybridretrieval via
	// packages/knowledgeapi).
	ModeSemantic Mode = "semantic"

	// ModeIssueRule targets packages/treeindex paths rooted at
	// Query.IssueOrRuleID: "find cases where this issue/rule/statute node
	// was applied."
	ModeIssueRule Mode = "issue_rule"
)

// allModes is the exhaustive set of recognized Mode values, used by
// IsValid.
var allModes = map[Mode]struct{}{
	ModeAuto:      {},
	ModeKeyword:   {},
	ModeSemantic:  {},
	ModeIssueRule: {},
}

// IsValid reports whether m is one of the recognized Mode constants.
func (m Mode) IsValid() bool {
	_, ok := allModes[m]
	return ok
}

// resolve returns the concrete Mode this Query should run in, expanding
// ModeAuto per its documented rule.
func (m Mode) resolve(q Query) Mode {
	if m != ModeAuto {
		return m
	}
	if strings.TrimSpace(q.IssueOrRuleID) != "" {
		return ModeIssueRule
	}
	return ModeSemantic
}

// Filter narrows Search to cases matching every non-zero field. Mirrors
// packages/hybridretrieval.Filter's "empty/zero means unrestricted"
// convention.
type Filter struct {
	// CategoryCode, if non-empty, restricts results to cases with this
	// packages/category taxonomy code (caselifecycle.Case.CategoryID).
	CategoryCode string

	// JurisdictionID, if non-nil, restricts results to cases under this
	// packages/jurisdiction entry.
	JurisdictionID uuid.UUID

	// PartyName, if non-empty, restricts results to cases involving a
	// party whose name matches (case-insensitively, substring) via the
	// injected PartyLookup. A Query with PartyName set but no PartyLookup
	// configured on the Engine matches no cases, rather than silently
	// ignoring the filter — see Engine.WithPartyLookup.
	PartyName string

	// State, if non-empty, restricts results to cases in this
	// caselifecycle.State.
	State string

	// DateFrom, if non-zero, restricts results to cases created at or
	// after this time (inclusive).
	DateFrom time.Time

	// DateTo, if non-zero, restricts results to cases created at or
	// before this time (inclusive).
	DateTo time.Time
}

// IsZero reports whether f has no fields set.
func (f Filter) IsZero() bool {
	return f.CategoryCode == "" &&
		f.JurisdictionID == uuid.Nil &&
		f.PartyName == "" &&
		f.State == "" &&
		f.DateFrom.IsZero() &&
		f.DateTo.IsZero()
}

// Page bounds a Search result set. A zero-value Page means "use
// DefaultPageSize starting at page 1".
type Page struct {
	// Number is the 1-based page number. Zero defaults to 1.
	Number int

	// Size is the number of results per page. Zero defaults to
	// DefaultPageSize.
	Size int
}

// DefaultPageSize is the Page.Size used when a Page leaves it zero.
const DefaultPageSize = 20

// MaxPageSize is the largest Page.Size Search accepts.
const MaxPageSize = 100

// normalize returns a copy of p with zero fields replaced by their
// documented defaults, or ErrInvalidPage if either field is negative.
func (p Page) normalize() (Page, error) {
	if p.Number < 0 || p.Size < 0 {
		return Page{}, ErrInvalidPage
	}
	out := p
	if out.Number == 0 {
		out.Number = 1
	}
	if out.Size == 0 {
		out.Size = DefaultPageSize
	}
	if out.Size > MaxPageSize {
		out.Size = MaxPageSize
	}
	return out, nil
}

// DefaultTopKPerCase is how many per-case content hits a CaseSearcher is
// asked for when Query.TopKPerCase is left zero.
const DefaultTopKPerCase = 5

// Query describes one cross-case search request. Construct with NewQuery
// and chain the With* methods, or build the struct literal directly.
type Query struct {
	// Text is the free-text search string. Required for ModeKeyword and
	// ModeSemantic (see AllowEmptyText for the escape hatch when a
	// filter-only search is intended).
	Text string

	// Mode selects the content-matching strategy. ModeAuto (the zero
	// value) picks a concrete mode per Mode.resolve's rule.
	Mode Mode

	// IssueOrRuleID names the treeindex root node ID to search from in
	// ModeIssueRule (or ModeAuto, which promotes to ModeIssueRule when
	// this is set).
	IssueOrRuleID string

	// Filter narrows the candidate case set. See Filter.
	Filter Filter

	// TopKPerCase caps how many content hits a CaseSearcher returns per
	// matched case, before cross-case ranking and pagination. Zero means
	// DefaultTopKPerCase.
	TopKPerCase int

	// AllowEmptyText permits a Query with a blank Text and blank
	// IssueOrRuleID to proceed as a filter-only search (list cases
	// matching Filter, with no content-level ranking) instead of
	// returning ErrEmptyQuery. Ignored unless Filter is also non-zero.
	AllowEmptyText bool

	// Page bounds the returned Results.
	Page Page
}

// NewQuery constructs a Query with the given free-text search string.
// Chain the With* methods to configure mode, filters, and paging.
func NewQuery(text string) Query {
	return Query{Text: text}
}

// WithMode returns a copy of q with Mode set to m.
func (q Query) WithMode(m Mode) Query {
	out := q
	out.Mode = m
	return out
}

// WithIssueOrRule returns a copy of q with IssueOrRuleID set to nodeID.
func (q Query) WithIssueOrRule(nodeID string) Query {
	out := q
	out.IssueOrRuleID = nodeID
	return out
}

// WithFilter returns a copy of q with Filter set to f.
func (q Query) WithFilter(f Filter) Query {
	out := q
	out.Filter = f
	return out
}

// WithPage returns a copy of q with Page set to p.
func (q Query) WithPage(p Page) Query {
	out := q
	out.Page = p
	return out
}

// WithTopKPerCase returns a copy of q with TopKPerCase set to k.
func (q Query) WithTopKPerCase(k int) Query {
	out := q
	out.TopKPerCase = k
	return out
}

// validate checks q for the structural errors Search rejects before ever
// touching a repository or CaseSearcher.
func (q Query) validate() error {
	if q.Mode != ModeAuto && !q.Mode.IsValid() {
		return ErrInvalidMode
	}
	if q.TopKPerCase < 0 {
		return ErrInvalidPage
	}
	resolved := q.Mode.resolve(q)
	textRequired := resolved == ModeKeyword || resolved == ModeSemantic
	if resolved == ModeIssueRule && strings.TrimSpace(q.IssueOrRuleID) == "" {
		return ErrEmptyQuery
	}
	if textRequired && strings.TrimSpace(q.Text) == "" {
		if q.AllowEmptyText && !q.Filter.IsZero() {
			return nil
		}
		return ErrEmptyQuery
	}
	return nil
}
