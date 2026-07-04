package threatmodel

import (
	"strings"
)

// This file is task 8's network-segmentation policy: named Zones and
// allow-rules between them, with real IsAllowed/Validate evaluation
// logic. It is structurally analogous to, but independent of and does
// not duplicate, packages/dataresidency.ResidencyPolicy (Phase 078):
// ResidencyPolicy governs which *geographic regions* tenant data may
// live in; SegmentationPolicy governs which *network zones* may talk
// to which other network zones, and on what port. The two compose
// conceptually -- a deployment can have both a residency policy and a
// segmentation policy active at once -- but this package does not
// import packages/dataresidency or call CheckProviderLocality; see
// doc.go.

// ZoneName identifies a network segmentation zone by name. An open
// string type (like packages/compliance.Framework), not a closed enum:
// a deployment's zone topology varies (a single-tenant on-prem
// deployment may have fewer zones than a multi-region SaaS
// deployment), so a closed Go enum would force a code change every
// time a deployment needed a differently-shaped topology.
type ZoneName string

// Common zone names this package's seed policies use as a starting
// point. A SegmentationPolicy is not restricted to these -- any
// non-blank ZoneName is structurally valid (see Zone.Validate) -- but
// most deployments' topology maps onto some subset of this three-zone
// shape.
const (
	// ZonePublicGateway is the internet-facing zone: the API gateway
	// (packages/gateway) and nothing else. It may only reach the
	// internal-services zone, never the data zone directly.
	ZonePublicGateway ZoneName = "public-gateway"

	// ZoneInternalServices is the zone containing every internal
	// service/package that is not directly internet-facing (ingestion,
	// reasoning orchestration, and so on). It may reach the data zone,
	// and receives traffic only from the public-gateway zone (or other
	// internal services).
	ZoneInternalServices ZoneName = "internal-services"

	// ZoneData is the most restricted zone: the database and any
	// durable store. It should only ever receive inbound connections
	// from the internal-services zone, never directly from
	// public-gateway.
	ZoneData ZoneName = "data"
)

// Zone is a single named network segment.
type Zone struct {
	// Name identifies this zone.
	Name ZoneName `json:"name"`

	// Description is a short human-readable summary of what lives in
	// this zone.
	Description string `json:"description"`
}

// Validate checks z for structural well-formedness.
func (z Zone) Validate() error {
	if strings.TrimSpace(string(z.Name)) == "" {
		return wrapf("Zone.Validate", ErrInvalidZone)
	}
	return nil
}

// SegmentationRule is a single allow-rule: traffic from From to To is
// permitted on Port (or any port, if Port is zero -- see
// SegmentationRule.AllowsPort).
type SegmentationRule struct {
	// From is the originating zone.
	From ZoneName `json:"from"`

	// To is the destination zone.
	To ZoneName `json:"to"`

	// Port is the specific destination port this rule allows. Zero
	// means "any port" -- a deliberately explicit wildcard rather than
	// overloading a specific port number (e.g. 0) that could otherwise
	// collide with a real port.
	Port int `json:"port,omitempty"`

	// Description explains why this rule exists.
	Description string `json:"description,omitempty"`
}

// AllowsPort reports whether r permits port. A zero r.Port always
// matches (any port); otherwise the ports must match exactly.
func (r SegmentationRule) AllowsPort(port int) bool {
	return r.Port == 0 || r.Port == port
}

// SegmentationPolicy is a structured network-segmentation policy: a
// set of named Zones plus allow-rules between them. Unlike
// packages/dataresidency.ResidencyPolicy's default-deny-unless-listed
// region allow-list, SegmentationPolicy is default-deny between EVERY
// pair of zones unless an explicit SegmentationRule permits it --
// including a zone and itself, which must also be listed explicitly
// if same-zone traffic is intended to be allowed, so a policy author
// cannot rely on an implicit "same zone always trusts itself" default
// they never actually wrote down.
type SegmentationPolicy struct {
	// Name identifies this policy (e.g. "production", "single-tenant-onprem").
	Name string `json:"name"`

	// Zones lists every zone this policy defines.
	Zones []Zone `json:"zones"`

	// Rules lists every allow-rule between zones. Any (From, To, Port)
	// combination not covered by a Rule is denied.
	Rules []SegmentationRule `json:"rules"`
}

// Validate checks p for structural well-formedness: a non-blank Name,
// at least one Zone, every Zone individually valid with no duplicate
// Zone.Name, and every Rule referencing Zones actually defined in
// p.Zones.
func (p SegmentationPolicy) Validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return wrapf("SegmentationPolicy.Validate", ErrInvalidSegmentationPolicy)
	}
	if len(p.Zones) == 0 {
		return wrapf("SegmentationPolicy.Validate", ErrInvalidSegmentationPolicy)
	}

	seen := make(map[ZoneName]struct{}, len(p.Zones))
	for _, z := range p.Zones {
		if err := z.Validate(); err != nil {
			return wrapf("SegmentationPolicy.Validate", err)
		}
		if _, dup := seen[z.Name]; dup {
			return wrapf("SegmentationPolicy.Validate", ErrInvalidSegmentationPolicy)
		}
		seen[z.Name] = struct{}{}
	}

	for _, r := range p.Rules {
		if _, ok := seen[r.From]; !ok {
			return wrapf("SegmentationPolicy.Validate", ErrZoneNotFound)
		}
		if _, ok := seen[r.To]; !ok {
			return wrapf("SegmentationPolicy.Validate", ErrZoneNotFound)
		}
	}
	return nil
}

// IsAllowed reports whether p permits traffic from fromZone to toZone
// on port, per its own default-deny-unless-listed semantics: true iff
// at least one Rule in p.Rules has From==fromZone, To==toZone, and
// AllowsPort(port). Returns false (not an error) for a zone pair with
// no matching rule, or for a fromZone/toZone p.Zones does not even
// define -- an undefined zone is trivially unreachable, which
// IsAllowed reports the same way as an explicitly denied one, since
// both mean "this traffic does not happen." Callers that need to
// distinguish "denied by an explicit topology" from "references a zone
// that does not exist in this policy at all" should call Validate
// first.
func (p SegmentationPolicy) IsAllowed(fromZone, toZone ZoneName, port int) bool {
	for _, r := range p.Rules {
		if r.From == fromZone && r.To == toZone && r.AllowsPort(port) {
			return true
		}
	}
	return false
}

// RulesFrom returns every Rule in p.Rules whose From matches zone,
// convenience for reporting/auditing code that wants "everything this
// zone is allowed to reach" without the caller re-filtering p.Rules
// itself.
func (p SegmentationPolicy) RulesFrom(zone ZoneName) []SegmentationRule {
	out := make([]SegmentationRule, 0)
	for _, r := range p.Rules {
		if r.From == zone {
			out = append(out, r)
		}
	}
	return out
}

// DefaultSegmentationPolicy returns a starter three-zone policy (task
// 8) matching this platform's real shape: a public-gateway zone
// (packages/gateway, Phase 009 -- the only internet-facing component),
// an internal-services zone (every other backend package: ingestion,
// reasoning orchestration, and so on), and a data zone (the database).
// The allow-rules encode the platform's actual intended data-flow: the
// public gateway may reach internal services on the application port,
// internal services may reach the data zone on the database port, and
// the public gateway may NEVER reach the data zone directly -- a
// defense-in-depth measure independent of, and in addition to,
// whatever authentication/authorization the data zone's own database
// enforces.
func DefaultSegmentationPolicy() SegmentationPolicy {
	const (
		appPort = 8443
		dbPort  = 5432
	)
	return SegmentationPolicy{
		Name: "default-three-zone",
		Zones: []Zone{
			{Name: ZonePublicGateway, Description: "Internet-facing API gateway (packages/gateway, Phase 009). The only zone reachable from outside the deployment."},
			{Name: ZoneInternalServices, Description: "Every internal backend package (ingestion, reasoning orchestration, and so on) that is not directly internet-facing."},
			{Name: ZoneData, Description: "The database and any durable store. The most restricted zone."},
		},
		Rules: []SegmentationRule{
			{
				From: ZonePublicGateway, To: ZoneInternalServices, Port: appPort,
				Description: "The public gateway forwards authenticated, validated requests to internal services on the application port.",
			},
			{
				From: ZoneInternalServices, To: ZoneData, Port: dbPort,
				Description: "Internal services read/write the database on the database port.",
			},
			{
				From: ZoneInternalServices, To: ZoneInternalServices,
				Description: "Internal services may call one another (e.g. reasoning orchestration calling the knowledge API) on any port -- explicitly listed rather than relying on an implicit same-zone-trusts-itself default.",
			},
			// Deliberately no rule for (ZonePublicGateway, ZoneData, *):
			// the public gateway must never reach the data zone
			// directly, on any port. This absence is the policy, not
			// an oversight -- TestDefaultSegmentationPolicy_GatewayCannotReachDataDirectly
			// asserts it explicitly.
		},
	}
}
