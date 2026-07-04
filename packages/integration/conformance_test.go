package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/integration"
)

// ConnectorConformanceTest verifies that c satisfies the Connector
// contract, mirroring packages/provider.ProviderConformanceTest's
// shape exactly.
//
// Any connector adapter can call this function from its own test
// suite to confirm it meets the interface requirements before
// integration.
func ConnectorConformanceTest(t *testing.T, c integration.Connector) {
	t.Helper()
	ctx := context.Background()

	t.Run("ID", func(t *testing.T) {
		if id := c.ID(); id == "" {
			t.Fatal("ID() must return a non-empty string")
		}
	})

	t.Run("Capabilities", func(t *testing.T) {
		cap := c.Capabilities()
		if cap.ConnectorID == "" {
			t.Error("Capabilities().ConnectorID must be non-empty")
		}
		if cap.ConnectorID != c.ID() {
			t.Errorf("Capabilities().ConnectorID %q must match ID() %q", cap.ConnectorID, c.ID())
		}
	})

	t.Run("Ping", func(t *testing.T) {
		if err := c.Ping(ctx); err != nil {
			t.Fatalf("Ping() returned unexpected error: %v", err)
		}
	})

	t.Run("ImportCases", func(t *testing.T) {
		cap := c.Capabilities()
		if !cap.SupportsImport {
			t.Skip("connector does not advertise SupportsImport")
		}
		cases, err := c.ImportCases(ctx, time.Time{})
		if err != nil {
			t.Fatalf("ImportCases() returned unexpected error: %v", err)
		}
		if cases == nil {
			t.Error("ImportCases() should return a non-nil (possibly empty) slice")
		}
	})

	t.Run("DeliverReport", func(t *testing.T) {
		cap := c.Capabilities()
		if !cap.SupportsDelivery {
			t.Skip("connector does not advertise SupportsDelivery")
		}
		report := integration.OutboundReport{
			CaseExternalID: "conformance-case-1",
			ReportKind:     "conformance_test",
			Format:         "markdown",
			Payload:        []byte("# conformance test report"),
		}
		receipt, err := c.DeliverReport(ctx, report)
		if err != nil {
			t.Fatalf("DeliverReport() returned unexpected error: %v", err)
		}
		if receipt.AcceptedAt.IsZero() {
			t.Error("DeliverReport() receipt AcceptedAt must be set")
		}
	})
}
