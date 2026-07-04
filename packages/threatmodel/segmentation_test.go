package threatmodel_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

func TestZone_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid zone passes", func(t *testing.T) {
		t.Parallel()
		z := threatmodel.Zone{Name: threatmodel.ZonePublicGateway, Description: "test"}
		if err := z.Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank name fails", func(t *testing.T) {
		t.Parallel()
		z := threatmodel.Zone{Name: "", Description: "test"}
		if err := z.Validate(); !errors.Is(err, threatmodel.ErrInvalidZone) {
			t.Errorf("Validate() = %v, want ErrInvalidZone", err)
		}
	})
}

func TestSegmentationRule_AllowsPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rule threatmodel.SegmentationRule
		port int
		want bool
	}{
		{"exact port match", threatmodel.SegmentationRule{Port: 443}, 443, true},
		{"port mismatch", threatmodel.SegmentationRule{Port: 443}, 8080, false},
		{"zero port allows any port", threatmodel.SegmentationRule{Port: 0}, 12345, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.rule.AllowsPort(tt.port); got != tt.want {
				t.Errorf("AllowsPort(%d) = %v, want %v", tt.port, got, tt.want)
			}
		})
	}
}

func validSegmentationPolicy() threatmodel.SegmentationPolicy {
	return threatmodel.SegmentationPolicy{
		Name: "test-policy",
		Zones: []threatmodel.Zone{
			{Name: "a", Description: "zone a"},
			{Name: "b", Description: "zone b"},
		},
		Rules: []threatmodel.SegmentationRule{
			{From: "a", To: "b", Port: 443},
		},
	}
}

func TestSegmentationPolicy_Validate(t *testing.T) {
	t.Parallel()

	t.Run("valid policy passes", func(t *testing.T) {
		t.Parallel()
		if err := validSegmentationPolicy().Validate(); err != nil {
			t.Errorf("Validate() = %v, want nil", err)
		}
	})

	t.Run("blank name fails", func(t *testing.T) {
		t.Parallel()
		p := validSegmentationPolicy()
		p.Name = ""
		if err := p.Validate(); !errors.Is(err, threatmodel.ErrInvalidSegmentationPolicy) {
			t.Errorf("Validate() = %v, want ErrInvalidSegmentationPolicy", err)
		}
	})

	t.Run("zero zones fails", func(t *testing.T) {
		t.Parallel()
		p := validSegmentationPolicy()
		p.Zones = nil
		if err := p.Validate(); !errors.Is(err, threatmodel.ErrInvalidSegmentationPolicy) {
			t.Errorf("Validate() = %v, want ErrInvalidSegmentationPolicy", err)
		}
	})

	t.Run("duplicate zone name fails", func(t *testing.T) {
		t.Parallel()
		p := validSegmentationPolicy()
		p.Zones = append(p.Zones, threatmodel.Zone{Name: "a", Description: "duplicate"})
		if err := p.Validate(); !errors.Is(err, threatmodel.ErrInvalidSegmentationPolicy) {
			t.Errorf("Validate() = %v, want ErrInvalidSegmentationPolicy", err)
		}
	})

	t.Run("rule referencing undefined From zone fails", func(t *testing.T) {
		t.Parallel()
		p := validSegmentationPolicy()
		p.Rules = append(p.Rules, threatmodel.SegmentationRule{From: "nonexistent", To: "b"})
		if err := p.Validate(); !errors.Is(err, threatmodel.ErrZoneNotFound) {
			t.Errorf("Validate() = %v, want ErrZoneNotFound", err)
		}
	})

	t.Run("rule referencing undefined To zone fails", func(t *testing.T) {
		t.Parallel()
		p := validSegmentationPolicy()
		p.Rules = append(p.Rules, threatmodel.SegmentationRule{From: "a", To: "nonexistent"})
		if err := p.Validate(); !errors.Is(err, threatmodel.ErrZoneNotFound) {
			t.Errorf("Validate() = %v, want ErrZoneNotFound", err)
		}
	})
}

func TestSegmentationPolicy_IsAllowed(t *testing.T) {
	t.Parallel()
	p := validSegmentationPolicy()

	t.Run("explicitly allowed pair and port", func(t *testing.T) {
		t.Parallel()
		if !p.IsAllowed("a", "b", 443) {
			t.Error("IsAllowed(a, b, 443) = false, want true")
		}
	})

	t.Run("allowed pair but wrong port", func(t *testing.T) {
		t.Parallel()
		if p.IsAllowed("a", "b", 8080) {
			t.Error("IsAllowed(a, b, 8080) = true, want false")
		}
	})

	t.Run("reverse direction not allowed", func(t *testing.T) {
		t.Parallel()
		if p.IsAllowed("b", "a", 443) {
			t.Error("IsAllowed(b, a, 443) = true, want false (no rule for this direction)")
		}
	})

	t.Run("same-zone traffic denied unless explicitly listed", func(t *testing.T) {
		t.Parallel()
		if p.IsAllowed("a", "a", 443) {
			t.Error("IsAllowed(a, a, 443) = true, want false (default-deny, no same-zone rule listed)")
		}
	})

	t.Run("undefined zone pair denied", func(t *testing.T) {
		t.Parallel()
		if p.IsAllowed("nonexistent", "b", 443) {
			t.Error("IsAllowed(nonexistent, b, 443) = true, want false")
		}
	})
}

func TestSegmentationPolicy_RulesFrom(t *testing.T) {
	t.Parallel()
	p := validSegmentationPolicy()

	got := p.RulesFrom("a")
	if len(got) != 1 || got[0].To != "b" {
		t.Errorf("RulesFrom(a) = %+v, want exactly one rule to b", got)
	}

	if got := p.RulesFrom("b"); len(got) != 0 {
		t.Errorf("RulesFrom(b) = %+v, want no rules (b has no outbound rules)", got)
	}
}

func TestDefaultSegmentationPolicy_IsValid(t *testing.T) {
	t.Parallel()

	p := threatmodel.DefaultSegmentationPolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("DefaultSegmentationPolicy().Validate() = %v, want nil", err)
	}
}

func TestDefaultSegmentationPolicy_GatewayCanReachInternalServices(t *testing.T) {
	t.Parallel()

	p := threatmodel.DefaultSegmentationPolicy()
	if !p.IsAllowed(threatmodel.ZonePublicGateway, threatmodel.ZoneInternalServices, 8443) {
		t.Error("public gateway cannot reach internal services on the application port, want allowed")
	}
}

func TestDefaultSegmentationPolicy_InternalServicesCanReachData(t *testing.T) {
	t.Parallel()

	p := threatmodel.DefaultSegmentationPolicy()
	if !p.IsAllowed(threatmodel.ZoneInternalServices, threatmodel.ZoneData, 5432) {
		t.Error("internal services cannot reach the data zone on the database port, want allowed")
	}
}

// TestDefaultSegmentationPolicy_GatewayCannotReachDataDirectly is the
// single most important invariant this policy encodes: the internet-
// facing gateway must never be able to reach the data zone directly,
// on any port, bypassing internal services entirely. This is a
// defense-in-depth control independent of whatever the database's own
// authentication enforces.
func TestDefaultSegmentationPolicy_GatewayCannotReachDataDirectly(t *testing.T) {
	t.Parallel()

	p := threatmodel.DefaultSegmentationPolicy()
	for _, port := range []int{5432, 443, 8443, 22, 0} {
		if p.IsAllowed(threatmodel.ZonePublicGateway, threatmodel.ZoneData, port) {
			t.Errorf("IsAllowed(public-gateway, data, %d) = true, want false: gateway must never reach the data zone directly", port)
		}
	}
}

func TestDefaultSegmentationPolicy_DataZoneCannotInitiateOutbound(t *testing.T) {
	t.Parallel()

	p := threatmodel.DefaultSegmentationPolicy()
	if len(p.RulesFrom(threatmodel.ZoneData)) != 0 {
		t.Error("the data zone has an outbound rule, want none: the data zone should only ever receive connections, never initiate them")
	}
}
