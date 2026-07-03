CREATE TABLE IF NOT EXISTS annotations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    case_id       UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    author_id     UUID NOT NULL,
    body          TEXT NOT NULL,
    anchor_type   TEXT NOT NULL,
    anchor_id     TEXT NOT NULL DEFAULT '',
    parent_id     UUID REFERENCES annotations (id) ON DELETE CASCADE,
    resolved      BOOLEAN NOT NULL DEFAULT false,
    resolved_by   UUID,
    resolved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT annotations_body_not_blank CHECK (btrim(body) <> ''),
    CONSTRAINT annotations_anchor_type_allowed CHECK (
        anchor_type IN ('case', 'tree_node', 'evidence_segment')
    ),
    CONSTRAINT annotations_anchor_id_matches_type CHECK (
        (anchor_type = 'case' AND anchor_id = '')
        OR (anchor_type IN ('tree_node', 'evidence_segment') AND btrim(anchor_id) <> '')
    ),
    CONSTRAINT annotations_resolved_fields_consistent CHECK (
        (resolved = false AND resolved_by IS NULL AND resolved_at IS NULL)
        OR (resolved = true AND resolved_by IS NOT NULL AND resolved_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_annotations_tenant_id ON annotations (tenant_id);
CREATE INDEX IF NOT EXISTS idx_annotations_case_id ON annotations (case_id);
CREATE INDEX IF NOT EXISTS idx_annotations_case_anchor ON annotations (case_id, anchor_type, anchor_id);
CREATE INDEX IF NOT EXISTS idx_annotations_parent_id ON annotations (parent_id);

CREATE TABLE IF NOT EXISTS annotation_mentions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    annotation_id      UUID NOT NULL REFERENCES annotations (id) ON DELETE CASCADE,
    case_id            UUID NOT NULL REFERENCES cases (id) ON DELETE CASCADE,
    tenant_id          UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    author_id          UUID NOT NULL,
    mentioned_user_id  UUID NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_annotation_mentions_tenant_user ON annotation_mentions (tenant_id, mentioned_user_id);
CREATE INDEX IF NOT EXISTS idx_annotation_mentions_annotation_id ON annotation_mentions (annotation_id);

CREATE TABLE IF NOT EXISTS annotation_audit_events (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    annotation_id  UUID NOT NULL,
    case_id        UUID NOT NULL,
    tenant_id      UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    verb           TEXT NOT NULL,
    actor          UUID NOT NULL,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT annotation_audit_events_verb_allowed CHECK (
        verb IN (
            'annotation.created',
            'annotation.edited',
            'annotation.deleted',
            'annotation.resolved',
            'annotation.reopened'
        )
    )
    -- annotation_id intentionally carries no foreign key: audit
    -- records must survive the deletion of the annotation they
    -- describe (see Repository.Delete's doc comment), mirroring how
    -- packages/caselifecycle's case_transitions table is append-only
    -- history independent of the mutable row it describes.
);

CREATE INDEX IF NOT EXISTS idx_annotation_audit_events_tenant_id ON annotation_audit_events (tenant_id);
CREATE INDEX IF NOT EXISTS idx_annotation_audit_events_annotation_id ON annotation_audit_events (annotation_id);
CREATE INDEX IF NOT EXISTS idx_annotation_audit_events_occurred_at ON annotation_audit_events (occurred_at);
