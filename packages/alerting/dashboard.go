package alerting

import (
	"time"
)

// Panel is one named tile within a DashboardDefinition: a
// human-readable Title backed by a single Catalogue metric, referenced
// by name (task 2) -- not a live value itself, since this package
// does not own metric storage/querying (that remains
// observability.Registry/whatever scrapes its "/metrics" endpoint, per
// doc.go). A caller renders a Panel by looking up its MetricName
// against its own metrics backend.
type Panel struct {
	// Title is a short, human-readable label for this panel (e.g.
	// "Cases ingested").
	Title string `json:"title"`

	// MetricName names the Catalogue metric this panel visualizes
	// (one of the Metric* constants in metrics.go).
	MetricName string `json:"metric_name"`

	// Description explains what this panel shows and why it matters
	// for the flow it belongs to.
	Description string `json:"description"`
}

// DashboardDefinition is a named collection of Panels for one "key
// flow" (task 2): a structured data model, not a UI, mirroring
// packages/compliance.Dashboard/BuildDashboard's report-type shape and
// packages/accounting.DashboardAPI's per-flow summary idea. A caller's
// own presentation layer renders a DashboardDefinition however it
// needs to (a web dashboard, a CLI table, a Grafana provisioning
// file).
type DashboardDefinition struct {
	// FlowName identifies which key flow this dashboard covers (e.g.
	// "ingestion", "reasoning", "sign-off").
	FlowName string `json:"flow_name"`

	// Panels is the ordered set of Panels this dashboard displays.
	Panels []Panel `json:"panels"`

	// GeneratedAt is when this definition was built -- dashboard
	// *definitions* are static/versioned, but recording when a
	// specific instance was constructed helps a caller detect a stale
	// cached copy.
	GeneratedAt time.Time `json:"generated_at"`
}

// knownFlows is the fixed set of "key flows" BuildDashboard recognizes
// (task 2's brief: "a couple of named key flows"). Each entry's Panels
// name a Catalogue metric from metrics.go by string -- reference only,
// this file does not import or query any metrics backend itself.
var knownFlows = map[string][]Panel{
	"ingestion": {
		{
			Title:       "Cases ingested",
			MetricName:  MetricCasesIngestedTotal,
			Description: "Cases that have completed the ingestion pipeline, by outcome.",
		},
		{
			Title:       "Pending subject-access requests",
			MetricName:  MetricSARRequestsPending,
			Description: "Currently pending data-subject-access requests awaiting resolution.",
		},
	},
	"reasoning": {
		{
			Title:       "Reasoning runs completed",
			MetricName:  MetricReasoningRunsTotal,
			Description: "Reasoning-pipeline runs completed, by outcome.",
		},
		{
			Title:       "Alerts fired",
			MetricName:  MetricAlertsFiredTotal,
			Description: "Alert events raised across every severity, including reasoning-quality alerts.",
		},
	},
	"sign-off": {
		{
			Title:       "Opinions signed off",
			MetricName:  MetricOpinionsSignedOffTotal,
			Description: "Opinions that have completed the sign-off workflow, by disposition.",
		},
	},
}

// KnownFlows returns the names of every flow BuildDashboard recognizes,
// in a stable order, for callers (a CLI, an admin UI) that want to
// list available dashboards without hardcoding the set themselves.
func KnownFlows() []string {
	// Fixed, intentionally stable order -- not derived from map
	// iteration, so this list never reorders between calls.
	return []string{"ingestion", "reasoning", "sign-off"}
}

// BuildDashboard returns the DashboardDefinition for the named key
// flow (task 2). Returns ErrUnknownFlow if flowName is not one of
// KnownFlows.
func BuildDashboard(flowName string, now time.Time) (DashboardDefinition, error) {
	panels, ok := knownFlows[flowName]
	if !ok {
		return DashboardDefinition{}, wrapf("BuildDashboard", ErrUnknownFlow)
	}
	// Defensive copy so a caller mutating the returned slice can never
	// corrupt the package-level knownFlows table.
	cp := make([]Panel, len(panels))
	copy(cp, panels)
	return DashboardDefinition{
		FlowName:    flowName,
		Panels:      cp,
		GeneratedAt: now,
	}, nil
}
