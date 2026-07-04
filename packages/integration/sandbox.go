package integration

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// SandboxConnector is a deterministic, in-process fake Connector
// implementation (task 8): Verdex's "no-op/mock provider for tests"
// equivalent from Phase 011 (packages/provider.NoOpProvider), applied
// to external court case-management systems. It is designed for use in
// unit tests, CI pipelines, local development, and as a live
// end-to-end demonstration connector where a real court system
// connection is unavailable or undesirable.
//
// Behaviour:
//   - ImportCases returns every seeded InboundCase whose
//     ExternalUpdatedAt is at or after since, sorted by ExternalID for
//     determinism.
//   - DeliverReport always accepts, recording the delivery so
//     Delivered can be inspected by tests, and returns an
//     ExternalReceiptID derived from a monotonic counter.
//   - Ping fails only if Unavailable is set to true, letting tests
//     exercise the retry/reconciliation paths against a connector that
//     is briefly down.
//
// SandboxConnector is safe for concurrent use.
type SandboxConnector struct {
	mu sync.Mutex

	id string

	// Unavailable makes Ping (and therefore anything that checks Ping
	// first) fail with ErrConnectorUnavailable. Toggle it from a test
	// to simulate an outage.
	Unavailable bool

	cases     map[string]InboundCase
	delivered []OutboundReport
	receiptN  int

	clock func() time.Time
}

// NewSandboxConnector builds an empty SandboxConnector registered
// under id.
func NewSandboxConnector(id string) *SandboxConnector {
	return &SandboxConnector{
		id:    id,
		cases: make(map[string]InboundCase),
		clock: time.Now,
	}
}

func (s *SandboxConnector) now() time.Time {
	if s.clock != nil {
		return s.clock().UTC()
	}
	return time.Now().UTC()
}

// SeedCase registers an InboundCase the sandbox will return from a
// future ImportCases call whose since is at or before c's
// ExternalUpdatedAt. Intended for test setup only.
func (s *SandboxConnector) SeedCase(c InboundCase) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cases[c.ExternalID] = c
}

// Delivered returns a copy of every OutboundReport DeliverReport has
// accepted so far, in call order. Intended for test assertions.
func (s *SandboxConnector) Delivered() []OutboundReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OutboundReport, len(s.delivered))
	copy(out, s.delivered)
	return out
}

// ID implements Connector.
func (s *SandboxConnector) ID() string { return s.id }

// Capabilities implements Connector.
func (s *SandboxConnector) Capabilities() ConnectorCapability {
	return ConnectorCapability{
		ConnectorID:      s.id,
		SystemName:       "Sandbox Test Connector",
		SupportsImport:   true,
		SupportsDelivery: true,
		MaxBatchSize:     0,
		Region:           "local",
	}
}

// Ping implements Connector.
func (s *SandboxConnector) Ping(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Unavailable {
		return ErrConnectorUnavailable
	}
	return nil
}

// ImportCases implements Connector.
func (s *SandboxConnector) ImportCases(ctx context.Context, since time.Time) ([]InboundCase, error) {
	if err := s.Ping(ctx); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]InboundCase, 0, len(s.cases))
	for _, c := range s.cases {
		if since.IsZero() || !c.ExternalUpdatedAt.Before(since) {
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ExternalID < out[j].ExternalID })
	return out, nil
}

// DeliverReport implements Connector.
func (s *SandboxConnector) DeliverReport(ctx context.Context, report OutboundReport) (DeliveryReceipt, error) {
	if err := s.Ping(ctx); err != nil {
		return DeliveryReceipt{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.receiptN++
	s.delivered = append(s.delivered, report)
	return DeliveryReceipt{
		ExternalReceiptID: fmt.Sprintf("sandbox-receipt-%04d", s.receiptN),
		AcceptedAt:        s.now(),
		Accepted:          true,
	}, nil
}

var _ Connector = (*SandboxConnector)(nil)
