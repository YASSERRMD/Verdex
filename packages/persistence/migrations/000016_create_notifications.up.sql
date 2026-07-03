CREATE TABLE IF NOT EXISTS notifications (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    recipient_id       UUID NOT NULL,
    kind               TEXT NOT NULL,
    title              TEXT NOT NULL DEFAULT '',
    body               TEXT NOT NULL DEFAULT '',
    case_id            UUID REFERENCES cases (id) ON DELETE CASCADE,
    related_entity_id  UUID,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at            TIMESTAMPTZ,
    CONSTRAINT notifications_kind_allowed CHECK (
        kind IN ('ingestion_complete', 'pending_signoff', 'mention', 'quality_alert', 'budget_alert', 'task_assignment')
    )
);

CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id ON notifications (tenant_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient ON notifications (tenant_id, recipient_id);
CREATE INDEX IF NOT EXISTS idx_notifications_recipient_unread ON notifications (tenant_id, recipient_id, read_at);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications (created_at);

CREATE TABLE IF NOT EXISTS notification_preferences (
    tenant_id  UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id    UUID NOT NULL,
    kind       TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    channels   TEXT[] NOT NULL DEFAULT '{}',
    PRIMARY KEY (tenant_id, user_id, kind),
    CONSTRAINT notification_preferences_kind_allowed CHECK (
        kind IN ('ingestion_complete', 'pending_signoff', 'mention', 'quality_alert', 'budget_alert', 'task_assignment')
    )
);

CREATE INDEX IF NOT EXISTS idx_notification_preferences_tenant_id ON notification_preferences (tenant_id);
