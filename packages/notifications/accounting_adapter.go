package notifications

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
)

// AccountingAlertSink adapts a *Service to implement
// packages/accounting.AlertSink, so every budget-threshold AlertEvent
// packages/accounting's budget checker fires lands a KindBudgetAlert
// Notification in each of Recipients' inboxes.
//
// packages/accounting.AlertEvent carries a TenantID but, like
// packages/reasoningeval.Alert, no per-user recipient (a budget alert
// is a tenant-wide signal, not tied to one user's action), so this
// adapter is constructed with a fixed Recipients list — typically the
// tenant's admins who own budget monitoring. event.TenantID (not a
// field on the adapter) determines which tenant the resulting
// Notification belongs to, so one AccountingAlertSink can safely serve
// every tenant's alerts as long as Recipients are valid in each.
type AccountingAlertSink struct {
	Service    *Service
	Recipients []uuid.UUID
}

// NewAccountingAlertSink builds an AccountingAlertSink.
func NewAccountingAlertSink(service *Service, recipients []uuid.UUID) *AccountingAlertSink {
	return &AccountingAlertSink{Service: service, Recipients: recipients}
}

// Send implements packages/accounting.AlertSink.
func (a *AccountingAlertSink) Send(ctx context.Context, event accounting.AlertEvent) error {
	title := "Budget alert"
	body := fmt.Sprintf(
		"%s: usage=%d/%d tokens, cost=$%.2f/$%.2f",
		event.AlertType, event.CurrentUsage, event.Limit, event.CurrentCostUSD, event.LimitUSD,
	)
	for _, recipientID := range a.Recipients {
		_, err := a.Service.Notify(ctx, NotifyInput{
			TenantID:    event.TenantID,
			RecipientID: recipientID,
			Kind:        KindBudgetAlert,
			Title:       title,
			Body:        body,
		})
		if err != nil {
			return wrapf("AccountingAlertSink.Send", err)
		}
	}
	return nil
}

var _ accounting.AlertSink = (*AccountingAlertSink)(nil)
