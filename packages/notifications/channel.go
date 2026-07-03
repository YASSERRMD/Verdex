package notifications

import (
	"context"
	"log"
)

// ChannelDeliverer pushes an already-persisted Notification out
// through one additional delivery channel (beyond the always-on
// in-app inbox). Per task 1, only a no-op/logged stub is required for
// this phase — a real email/push transport is future work; this
// interface is the seam a later phase wires a real implementation
// into.
type ChannelDeliverer interface {
	// Deliver pushes n through this channel. Implementations should be
	// fast and non-blocking; heavy I/O should be offloaded to a
	// goroutine, mirroring every NotificationSink/AlertSink/MentionSink
	// convention in this repository.
	Deliver(ctx context.Context, n *Notification) error
}

// LoggingChannelDeliverer is a ChannelDeliverer that writes to the
// standard logger, tagged with the channel name it stands in for. It
// is the default (and, for this phase, only) implementation for both
// ChannelEmail and ChannelPush — see EmailChannel/PushChannel.
type LoggingChannelDeliverer struct {
	// ChannelName labels log lines so a multi-channel deployment can
	// tell which stub fired (e.g. "email", "push").
	ChannelName string
	Logger      *log.Logger
}

// Deliver implements ChannelDeliverer by writing n to the configured
// logger.
func (d *LoggingChannelDeliverer) Deliver(_ context.Context, n *Notification) error {
	logger := d.Logger
	if logger == nil {
		logger = log.Default()
	}
	logger.Printf(
		"[notifications] channel=%s stub-delivery recipient=%s kind=%s title=%q",
		d.ChannelName, n.RecipientID, n.Kind, n.Title,
	)
	return nil
}

// EmailChannel is the no-op/logged stub for ChannelEmail, per task 1
// ("email/push as no-op/logged stubs behind an interface, not fully
// built"). Swap for a real SMTP/API-backed ChannelDeliverer once an
// email transport is chosen.
var EmailChannel ChannelDeliverer = &LoggingChannelDeliverer{ChannelName: string(ChannelEmail)}

// PushChannel is the no-op/logged stub for ChannelPush, mirroring
// EmailChannel. Swap for a real push-notification transport (e.g.
// FCM/APNs) once one is chosen.
var PushChannel ChannelDeliverer = &LoggingChannelDeliverer{ChannelName: string(ChannelPush)}

// deliverers maps each Channel constant to the ChannelDeliverer that
// handles it. Both entries are logged stubs today (task 1) — see
// EmailChannel/PushChannel.
var deliverers = map[Channel]ChannelDeliverer{
	ChannelEmail: EmailChannel,
	ChannelPush:  PushChannel,
}

// deliverStubChannels fans n out to every deliverer named in channels,
// ignoring unknown Channel values (Preference.Validate already rejects
// those before persistence, so this is defense in depth, not the
// primary validation path) and swallowing individual delivery errors —
// mirroring every *Sink.Notify's "best effort, in-app persistence is
// the source of truth" posture in this package: a stub channel failing
// must never roll back or block the already-persisted in-app
// Notification.
func deliverStubChannels(ctx context.Context, channels []Channel, n *Notification) {
	for _, c := range channels {
		d, ok := deliverers[c]
		if !ok {
			continue
		}
		_ = d.Deliver(ctx, n)
	}
}
