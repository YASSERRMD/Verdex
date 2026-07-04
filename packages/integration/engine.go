package integration

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// Engine is the integration-framework orchestrator: it composes the
// connector Registry, ConnectorConfig/ConnectorCredentials/
// FieldMapping storage, and ImportRun/DeliveryRun/
// ReconciliationResult history into one set of tenant- and
// permission-scoped operations, recording every attempt via AuditSink.
// Engine mirrors packages/compliance.Engine's shape closely:
// authenticate, check tenant match, check permission, mutate, audit
// regardless of outcome.
type Engine struct {
	configs         ConfigRepository
	credentials     CredentialsRepository
	mappings        FieldMappingRepository
	imports         ImportRunRepository
	deliveries      DeliveryRunRepository
	reconciliations ReconciliationRepository
	registry        *Registry
	audit           *AuditSink
	retryPolicy     RetryPolicy
	clock           func() time.Time
}

// EngineDeps bundles Engine's constructor dependencies, following
// packages/keymanagement.NewService's multi-dependency-constructor
// shape but as a struct (six repositories plus a registry plus an
// audit sink is unwieldy as positional arguments).
type EngineDeps struct {
	Configs         ConfigRepository
	Credentials     CredentialsRepository
	Mappings        FieldMappingRepository
	Imports         ImportRunRepository
	Deliveries      DeliveryRunRepository
	Reconciliations ReconciliationRepository

	// Registry resolves a ConnectorConfig.ConnectorType to a live
	// Connector. Required.
	Registry *Registry

	// Audit records every operation. Required -- unlike
	// packages/compliance.Engine, which tolerates a nil audit sink for
	// lightweight unit tests, this package's operations reach an
	// external system and must always be audited.
	Audit *AuditSink

	// RetryPolicy configures WithRetry for every Connector call this
	// Engine makes. Zero value means DefaultRetryPolicy.
	RetryPolicy RetryPolicy
}

// NewEngine builds an Engine from deps. Every repository, Registry,
// and Audit must be non-nil.
func NewEngine(deps EngineDeps) (*Engine, error) {
	if deps.Configs == nil || deps.Credentials == nil || deps.Mappings == nil ||
		deps.Imports == nil || deps.Deliveries == nil || deps.Reconciliations == nil {
		return nil, ErrNilStore
	}
	if deps.Registry == nil {
		return nil, wrapf("NewEngine", ErrNilConnector)
	}
	if deps.Audit == nil {
		return nil, ErrNilAuditSink
	}
	policy := deps.RetryPolicy
	if policy.MaxAttempts == 0 {
		policy = DefaultRetryPolicy()
	}
	return &Engine{
		configs:         deps.Configs,
		credentials:     deps.Credentials,
		mappings:        deps.Mappings,
		imports:         deps.Imports,
		deliveries:      deps.Deliveries,
		reconciliations: deps.Reconciliations,
		registry:        deps.Registry,
		audit:           deps.Audit,
		retryPolicy:     policy,
		clock:           time.Now,
	}, nil
}

func (e *Engine) now() time.Time {
	if e.clock != nil {
		return e.clock().UTC()
	}
	return time.Now().UTC()
}

// RegisterConnectorConfig validates and persists cfg, requiring
// managePermission. Every attempt (successful or not) is recorded via
// AuditSink.RecordConnectorRegister.
func (e *Engine) RegisterConnectorConfig(ctx context.Context, cfg ConnectorConfig) (ConnectorConfig, error) {
	user, authErr := authorizeManage(ctx)
	result, err := e.registerConnectorConfig(ctx, user, authErr, cfg)
	actor := actorFromCtx(ctx)
	if _, auditErr := e.audit.RecordConnectorRegister(ctx, cfg.TenantID, actor, result, err); auditErr != nil {
		return ConnectorConfig{}, auditErr
	}
	return result, err
}

func (e *Engine) registerConnectorConfig(ctx context.Context, user *identity.User, authErr error, cfg ConnectorConfig) (ConnectorConfig, error) {
	if authErr != nil {
		return cfg, authErr
	}
	if err := requireMatchingUserTenant(user, cfg.TenantID); err != nil {
		return cfg, err
	}
	if cfg.ID == uuid.Nil {
		cfg.ID = uuid.New()
	}
	now := e.now()
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = now
	}
	cfg.UpdatedAt = now
	if cfg.CreatedBy == uuid.Nil {
		cfg.CreatedBy = user.ID
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	if err := e.configs.Create(ctx, cfg.TenantID, &cfg); err != nil {
		return cfg, wrapf("RegisterConnectorConfig", err)
	}
	return cfg, nil
}

// GetConnectorConfig returns the ConnectorConfig identified by id,
// requiring viewPermission.
func (e *Engine) GetConnectorConfig(ctx context.Context, tenantID, id uuid.UUID) (ConnectorConfig, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return ConnectorConfig{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ConnectorConfig{}, err
	}
	cfg, err := e.configs.Get(ctx, tenantID, id)
	if err != nil {
		return ConnectorConfig{}, wrapf("GetConnectorConfig", err)
	}
	return *cfg, nil
}

// ListConnectorConfigs returns every ConnectorConfig for tenantID,
// requiring viewPermission.
func (e *Engine) ListConnectorConfigs(ctx context.Context, tenantID uuid.UUID) ([]ConnectorConfig, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.configs.List(ctx, tenantID)
	if err != nil {
		return nil, wrapf("ListConnectorConfigs", err)
	}
	return list, nil
}

// SetCredentials validates (structurally, and live if verify is
// non-nil) and persists creds, requiring managePermission. Every
// attempt is recorded via AuditSink.RecordCredentialsSet, never
// logging SecretRef.
func (e *Engine) SetCredentials(ctx context.Context, creds ConnectorCredentials, verify VerifyFunc) (ConnectorCredentials, error) {
	user, authErr := authorizeManage(ctx)
	result, err := e.setCredentials(ctx, user, authErr, creds, verify)
	actor := actorFromCtx(ctx)
	if _, auditErr := e.audit.RecordCredentialsSet(ctx, creds.TenantID, actor, result, err); auditErr != nil {
		return ConnectorCredentials{}, auditErr
	}
	return result, err
}

func (e *Engine) setCredentials(ctx context.Context, user *identity.User, authErr error, creds ConnectorCredentials, verify VerifyFunc) (ConnectorCredentials, error) {
	if authErr != nil {
		return creds, authErr
	}
	if err := requireMatchingUserTenant(user, creds.TenantID); err != nil {
		return creds, err
	}
	if creds.ID == uuid.Nil {
		creds.ID = uuid.New()
	}
	now := e.now()
	if creds.CreatedAt.IsZero() {
		creds.CreatedAt = now
	}
	creds.UpdatedAt = now
	if creds.CreatedBy == uuid.Nil {
		creds.CreatedBy = user.ID
	}

	verified, err := AuthorizeCredentials(ctx, creds, verify, now)
	if err != nil {
		return creds, err
	}
	if err := e.credentials.Create(ctx, verified.TenantID, &verified); err != nil {
		return verified, wrapf("SetCredentials", err)
	}
	return verified, nil
}

// SetFieldMapping validates and persists m, requiring managePermission.
func (e *Engine) SetFieldMapping(ctx context.Context, m FieldMapping) (FieldMapping, error) {
	user, err := authorizeManage(ctx)
	if err != nil {
		return FieldMapping{}, err
	}
	if err := requireMatchingUserTenant(user, m.TenantID); err != nil {
		return FieldMapping{}, err
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	now := e.now()
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	if m.CreatedBy == uuid.Nil {
		m.CreatedBy = user.ID
	}
	if err := m.Validate(); err != nil {
		return m, err
	}
	if err := e.mappings.Create(ctx, m.TenantID, &m); err != nil {
		return m, wrapf("SetFieldMapping", err)
	}
	return m, nil
}

// resolveConnector looks up the live Connector for cfg's ConnectorType
// in e.registry, wrapped for a consistent error message.
func (e *Engine) resolveConnector(cfg ConnectorConfig) (Connector, error) {
	conn, err := e.registry.Get(cfg.ConnectorType)
	if err != nil {
		return nil, wrapf("resolveConnector", err)
	}
	return conn, nil
}

// RunImport triggers an inbound case import through the ConnectorConfig
// identified by connectorConfigID: resolves the live Connector,
// retries ImportCases with backoff, records the outcome as an
// ImportRun, and audits the attempt regardless of outcome (task 2).
// mapFn, if non-nil, is invoked once per InboundCase to translate it
// via a FieldMapping and accept it into this platform's case store;
// mapFn's error marks that one external ID as failed without aborting
// the rest of the run. A nil mapFn skips mapping (useful for a
// Ping-only or count-only import run).
func (e *Engine) RunImport(ctx context.Context, tenantID, connectorConfigID uuid.UUID, since time.Time, mapFn func(InboundCase) error) (ImportRun, error) {
	user, authErr := authorizeManage(ctx)
	run, err := e.runImport(ctx, user, authErr, tenantID, connectorConfigID, since, mapFn)
	actor := actorFromCtx(ctx)
	if _, auditErr := e.audit.RecordImportRun(ctx, tenantID, actor, run, err); auditErr != nil {
		return ImportRun{}, auditErr
	}
	return run, err
}

func (e *Engine) runImport(ctx context.Context, user *identity.User, authErr error, tenantID, connectorConfigID uuid.UUID, since time.Time, mapFn func(InboundCase) error) (ImportRun, error) {
	startedAt := e.now()
	run := ImportRun{
		ID:                uuid.New(),
		TenantID:          tenantID,
		ConnectorConfigID: connectorConfigID,
		Since:             since,
		StartedAt:         startedAt,
	}
	if authErr != nil {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = authErr.Error()
		run.FinishedAt = e.now()
		return run, authErr
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = err.Error()
		run.FinishedAt = e.now()
		return run, err
	}
	run.TriggeredBy = user.ID

	cfg, err := e.configs.Get(ctx, tenantID, connectorConfigID)
	if err != nil {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = err.Error()
		run.FinishedAt = e.now()
		return run, wrapf("RunImport", err)
	}
	if !cfg.Enabled {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = "connector configuration is disabled"
		run.FinishedAt = e.now()
		err := wrapf("RunImport", ErrInvalidConnectorConfig)
		return run, err
	}

	conn, err := e.resolveConnector(*cfg)
	if err != nil {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = err.Error()
		run.FinishedAt = e.now()
		return run, err
	}

	var cases []InboundCase
	retryErr := WithRetry(ctx, e.retryPolicy, nil, func(ctx context.Context) error {
		result, callErr := conn.ImportCases(ctx, since)
		if callErr != nil {
			return callErr
		}
		cases = result
		return nil
	})
	if retryErr != nil {
		run.Status = ImportRunStatusFailed
		run.ErrorMessage = retryErr.Error()
		run.FinishedAt = e.now()
		if err := e.imports.Create(ctx, tenantID, &run); err != nil {
			return run, wrapf("RunImport", err)
		}
		return run, wrapf("RunImport", retryErr)
	}

	failedIDs := make([]string, 0)
	mappedCount := 0
	for _, c := range cases {
		if mapFn == nil {
			mappedCount++
			continue
		}
		if err := mapFn(c); err != nil {
			failedIDs = append(failedIDs, c.ExternalID)
			continue
		}
		mappedCount++
	}

	status, importedIDs := summarizeImportRun(cases, failedIDs)
	run.Status = status
	run.ImportedCount = len(cases)
	run.MappedCount = mappedCount
	run.FailedExternalIDs = failedIDs
	run.ImportedExternalIDs = importedIDs
	run.FinishedAt = e.now()

	if err := e.imports.Create(ctx, tenantID, &run); err != nil {
		return run, wrapf("RunImport", err)
	}
	return run, nil
}

// RunDelivery triggers an outbound report delivery through the
// ConnectorConfig identified by connectorConfigID: resolves the live
// Connector, retries DeliverReport with backoff, records the outcome
// as a DeliveryRun, and audits the attempt regardless of outcome
// (task 3).
func (e *Engine) RunDelivery(ctx context.Context, tenantID, connectorConfigID uuid.UUID, report OutboundReport) (DeliveryRun, error) {
	user, authErr := authorizeManage(ctx)
	run, err := e.runDelivery(ctx, user, authErr, tenantID, connectorConfigID, report)
	actor := actorFromCtx(ctx)
	if _, auditErr := e.audit.RecordDeliveryRun(ctx, tenantID, actor, run, err); auditErr != nil {
		return DeliveryRun{}, auditErr
	}
	return run, err
}

func (e *Engine) runDelivery(ctx context.Context, user *identity.User, authErr error, tenantID, connectorConfigID uuid.UUID, report OutboundReport) (DeliveryRun, error) {
	startedAt := e.now()
	run := DeliveryRun{
		ID:                uuid.New(),
		TenantID:          tenantID,
		ConnectorConfigID: connectorConfigID,
		CaseExternalID:    report.CaseExternalID,
		ReportKind:        report.ReportKind,
		StartedAt:         startedAt,
	}
	if authErr != nil {
		run.Status = DeliveryRunStatusFailed
		run.Detail = authErr.Error()
		run.FinishedAt = e.now()
		return run, authErr
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		run.Status = DeliveryRunStatusFailed
		run.Detail = err.Error()
		run.FinishedAt = e.now()
		return run, err
	}
	run.TriggeredBy = user.ID

	cfg, err := e.configs.Get(ctx, tenantID, connectorConfigID)
	if err != nil {
		run.Status = DeliveryRunStatusFailed
		run.Detail = err.Error()
		run.FinishedAt = e.now()
		return run, wrapf("RunDelivery", err)
	}
	if !cfg.Enabled {
		run.Status = DeliveryRunStatusFailed
		run.Detail = "connector configuration is disabled"
		run.FinishedAt = e.now()
		err := wrapf("RunDelivery", ErrInvalidConnectorConfig)
		return run, err
	}

	conn, err := e.resolveConnector(*cfg)
	if err != nil {
		run.Status = DeliveryRunStatusFailed
		run.Detail = err.Error()
		run.FinishedAt = e.now()
		return run, err
	}

	attempts := 0
	var receipt DeliveryReceipt
	retryErr := WithRetry(ctx, e.retryPolicy, nil, func(ctx context.Context) error {
		attempts++
		result, callErr := conn.DeliverReport(ctx, report)
		if callErr != nil {
			return callErr
		}
		receipt = result
		return nil
	})
	run.AttemptCount = attempts
	if retryErr != nil {
		run.Status = DeliveryRunStatusFailed
		run.Detail = retryErr.Error()
		run.FinishedAt = e.now()
		if err := e.deliveries.Create(ctx, tenantID, &run); err != nil {
			return run, wrapf("RunDelivery", err)
		}
		return run, wrapf("RunDelivery", retryErr)
	}

	run.Status = statusFromReceipt(receipt)
	run.ExternalReceiptID = receipt.ExternalReceiptID
	run.Detail = receipt.Detail
	run.FinishedAt = e.now()

	if err := e.deliveries.Create(ctx, tenantID, &run); err != nil {
		return run, wrapf("RunDelivery", err)
	}
	return run, nil
}

// RunReconciliation compares expected against observed external IDs
// for connectorConfigID (task 6), persists the ReconciliationResult,
// and audits the attempt regardless of outcome.
func (e *Engine) RunReconciliation(ctx context.Context, tenantID, connectorConfigID uuid.UUID, kind ReconciliationKind, expected, observed []string) (ReconciliationResult, error) {
	user, authErr := authorizeManage(ctx)
	result, err := e.runReconciliation(ctx, user, authErr, tenantID, connectorConfigID, kind, expected, observed)
	actor := actorFromCtx(ctx)
	if _, auditErr := e.audit.RecordReconciliation(ctx, tenantID, actor, result, err); auditErr != nil {
		return ReconciliationResult{}, auditErr
	}
	return result, err
}

func (e *Engine) runReconciliation(ctx context.Context, user *identity.User, authErr error, tenantID, connectorConfigID uuid.UUID, kind ReconciliationKind, expected, observed []string) (ReconciliationResult, error) {
	if authErr != nil {
		return ReconciliationResult{}, authErr
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return ReconciliationResult{}, err
	}

	result, err := Reconcile(tenantID, connectorConfigID, kind, expected, observed, user.ID, e.now())
	if err != nil {
		return result, err
	}
	if err := e.reconciliations.Create(ctx, tenantID, &result); err != nil {
		return result, wrapf("RunReconciliation", err)
	}
	return result, nil
}

// ListImportRuns returns every ImportRun for connectorConfigID,
// requiring viewPermission.
func (e *Engine) ListImportRuns(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ImportRun, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.imports.ListForConnector(ctx, tenantID, connectorConfigID)
	if err != nil {
		return nil, wrapf("ListImportRuns", err)
	}
	return list, nil
}

// ListDeliveryRuns returns every DeliveryRun for connectorConfigID,
// requiring viewPermission.
func (e *Engine) ListDeliveryRuns(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]DeliveryRun, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.deliveries.ListForConnector(ctx, tenantID, connectorConfigID)
	if err != nil {
		return nil, wrapf("ListDeliveryRuns", err)
	}
	return list, nil
}

// ListReconciliations returns every ReconciliationResult for
// connectorConfigID, requiring viewPermission.
func (e *Engine) ListReconciliations(ctx context.Context, tenantID, connectorConfigID uuid.UUID) ([]ReconciliationResult, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}
	list, err := e.reconciliations.ListForConnector(ctx, tenantID, connectorConfigID)
	if err != nil {
		return nil, wrapf("ListReconciliations", err)
	}
	return list, nil
}
