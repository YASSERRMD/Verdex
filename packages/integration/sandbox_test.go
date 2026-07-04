package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestSandboxConnectorConformance(t *testing.T) {
	t.Parallel()
	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.SeedCase(integration.InboundCase{
		ExternalID:         "case-1",
		ExternalUpdatedAt:  time.Now(),
		Fields:             map[string]string{"case_title": "Seed Case"},
	})
	ConnectorConformanceTest(t, sandbox)
}

func TestSandboxConnectorImportCasesFiltersBySince(t *testing.T) {
	t.Parallel()
	sandbox := integration.NewSandboxConnector("sandbox")

	older := time.Now().Add(-48 * time.Hour)
	newer := time.Now()

	sandbox.SeedCase(integration.InboundCase{ExternalID: "old-case", ExternalUpdatedAt: older, Fields: map[string]string{"title": "old"}})
	sandbox.SeedCase(integration.InboundCase{ExternalID: "new-case", ExternalUpdatedAt: newer, Fields: map[string]string{"title": "new"}})

	cutoff := time.Now().Add(-24 * time.Hour)
	cases, err := sandbox.ImportCases(context.Background(), cutoff)
	if err != nil {
		t.Fatalf("ImportCases() error = %v", err)
	}
	if len(cases) != 1 || cases[0].ExternalID != "new-case" {
		t.Fatalf("ImportCases(%v) = %+v, want only new-case", cutoff, cases)
	}
}

func TestSandboxConnectorImportCasesZeroSinceReturnsAll(t *testing.T) {
	t.Parallel()
	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.SeedCase(integration.InboundCase{ExternalID: "a", ExternalUpdatedAt: time.Now().Add(-1000 * time.Hour)})
	sandbox.SeedCase(integration.InboundCase{ExternalID: "b", ExternalUpdatedAt: time.Now()})

	cases, err := sandbox.ImportCases(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("ImportCases() error = %v", err)
	}
	if len(cases) != 2 {
		t.Fatalf("ImportCases(zero) returned %d cases, want 2", len(cases))
	}
}

func TestSandboxConnectorDeliverReportRecordsDelivery(t *testing.T) {
	t.Parallel()
	sandbox := integration.NewSandboxConnector("sandbox")

	report := integration.OutboundReport{CaseExternalID: "case-1", ReportKind: "opinion_summary", Format: "markdown", Payload: []byte("body")}
	receipt, err := sandbox.DeliverReport(context.Background(), report)
	if err != nil {
		t.Fatalf("DeliverReport() error = %v", err)
	}
	if !receipt.Accepted {
		t.Error("expected receipt.Accepted = true")
	}
	if receipt.ExternalReceiptID == "" {
		t.Error("expected non-empty ExternalReceiptID")
	}

	delivered := sandbox.Delivered()
	if len(delivered) != 1 || delivered[0].CaseExternalID != "case-1" {
		t.Fatalf("Delivered() = %+v, want one delivery for case-1", delivered)
	}
}

func TestSandboxConnectorUnavailable(t *testing.T) {
	t.Parallel()
	sandbox := integration.NewSandboxConnector("sandbox")
	sandbox.Unavailable = true

	if err := sandbox.Ping(context.Background()); !errors.Is(err, integration.ErrConnectorUnavailable) {
		t.Fatalf("Ping() error = %v, want ErrConnectorUnavailable", err)
	}
	if _, err := sandbox.ImportCases(context.Background(), time.Time{}); !errors.Is(err, integration.ErrConnectorUnavailable) {
		t.Fatalf("ImportCases() error = %v, want ErrConnectorUnavailable", err)
	}
	if _, err := sandbox.DeliverReport(context.Background(), integration.OutboundReport{}); !errors.Is(err, integration.ErrConnectorUnavailable) {
		t.Fatalf("DeliverReport() error = %v, want ErrConnectorUnavailable", err)
	}
}
