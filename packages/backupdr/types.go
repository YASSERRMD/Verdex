package backupdr

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DataClass categorizes what is being backed up, so a BackupPolicy can
// be tuned per category rather than applying one blanket schedule to
// every kind of data this platform holds (task 1). DataClass is a
// closed enum, unlike packages/compliance.Framework's deliberately
// open string type: the set of data classes this platform stores is a
// property of its own schema, not something a customer or jurisdiction
// varies, so a fixed taxonomy is appropriate here.
type DataClass string

const (
	// DataClassCaseData covers case records, filings, hearings, and
	// other primary case-lifecycle data (packages/caselifecycle,
	// packages/persistence's case tables).
	DataClassCaseData DataClass = "case_data"

	// DataClassCorpusPrecedent covers the statute/precedent knowledge
	// corpus this platform indexes and retrieves from
	// (packages/statute, packages/precedent, packages/vectorindex).
	DataClassCorpusPrecedent DataClass = "corpus_precedent"

	// DataClassAuditLog covers the durable, hash-chained audit trail
	// itself (packages/auditlog, Phase 077) -- backing up the backup
	// system's own audit sink is deliberately still in scope.
	DataClassAuditLog DataClass = "audit_log"

	// DataClassConfig covers tenant and deployment configuration:
	// identity/role assignments, compliance profiles, jurisdiction and
	// reasoning-profile settings (packages/config, packages/tenancy).
	DataClassConfig DataClass = "config"
)

// IsValid reports whether c is one of the named DataClass constants.
func (c DataClass) IsValid() bool {
	switch c {
	case DataClassCaseData, DataClassCorpusPrecedent, DataClassAuditLog, DataClassConfig:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (c DataClass) String() string { return string(c) }

// BackupLocation classifies where a completed backup's bytes actually
// live, distinguishing routine same-region storage from the
// cross-region/offline options task 4 asks for.
type BackupLocation string

const (
	// LocationPrimaryRegion means the backup was written to the
	// deployment's primary region's storage -- the default location for
	// routine automated backups.
	LocationPrimaryRegion BackupLocation = "primary_region"

	// LocationCrossRegion means the backup was replicated to (or
	// written directly into) a secondary region distinct from the
	// deployment's primary one, for resilience against a whole-region
	// outage.
	LocationCrossRegion BackupLocation = "cross_region"

	// LocationOffline means the backup was exported to offline/air-gapped
	// storage (e.g. a bundle handed to packages/airgapped's export
	// flow) with no live network path back to this deployment --
	// referenced by name/tag only, not imported.
	LocationOffline BackupLocation = "offline"
)

// IsValid reports whether l is one of the named BackupLocation
// constants.
func (l BackupLocation) IsValid() bool {
	switch l {
	case LocationPrimaryRegion, LocationCrossRegion, LocationOffline:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (l BackupLocation) String() string { return string(l) }

// BackupPolicy is the per-DataClass backup configuration: how often a
// DataClass is backed up, how long each backup is retained, whether
// encryption is required, and whether a cross-region/offline copy is
// mandated (task 1). One BackupPolicy exists per tenant/DataClass pair
// within a tenant's registered set -- mirroring
// packages/privacy.Engine's retentionPolicies registry shape exactly.
type BackupPolicy struct {
	// TenantID is the tenant this policy governs.
	TenantID uuid.UUID `json:"tenant_id"`

	// Class is the DataClass this policy governs.
	Class DataClass `json:"class"`

	// Frequency is how often a backup of Class must be taken (e.g.
	// time.Hour for hourly, 24*time.Hour for daily). Must be positive.
	Frequency time.Duration `json:"frequency"`

	// RetentionWindow is how long a completed backup of Class is kept
	// before it becomes eligible for deletion. Must be positive and
	// mirrors packages/auditlog.RetentionPolicy.Window's shape.
	RetentionWindow time.Duration `json:"retention_window"`

	// EncryptionRequired reports whether every backup of Class must be
	// wrapped via packages/encryption's EncryptBackup before being
	// written to any BackupLocation. Referenced by package name here,
	// not imported: this package does not call packages/encryption
	// directly (see doc/backup-dr.md), it records the requirement and
	// leaves enforcement to the backup-execution tooling that actually
	// writes bytes.
	EncryptionRequired bool `json:"encryption_required"`

	// CrossRegionRequired reports whether a copy of Class's backups
	// must additionally land in a BackupLocation other than
	// LocationPrimaryRegion (LocationCrossRegion or LocationOffline).
	CrossRegionRequired bool `json:"cross_region_required"`

	// CreatedBy is the identity.User who set this policy.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks p for structural well-formedness.
func (p *BackupPolicy) Validate() error {
	if p == nil {
		return ErrInvalidPolicy
	}
	if p.TenantID == uuid.Nil {
		return wrapf("BackupPolicy.Validate", ErrEmptyTenantID)
	}
	if !p.Class.IsValid() {
		return wrapf("BackupPolicy.Validate", ErrInvalidDataClass)
	}
	if p.Frequency <= 0 {
		return wrapf("BackupPolicy.Validate", ErrInvalidPolicy)
	}
	if p.RetentionWindow <= 0 {
		return wrapf("BackupPolicy.Validate", ErrInvalidPolicy)
	}
	return nil
}

// BackupStatus is the lifecycle state of a completed (or attempted)
// BackupRecord.
type BackupStatus string

const (
	// BackupStatusSucceeded means the backup completed and its
	// integrity hash was recorded successfully.
	BackupStatusSucceeded BackupStatus = "succeeded"

	// BackupStatusFailed means the backup attempt did not complete
	// successfully -- e.g. the source read failed, the encryption step
	// failed, or the write to BackupLocation failed.
	BackupStatusFailed BackupStatus = "failed"

	// BackupStatusVerifying means the backup bytes were written but
	// integrity verification (VerifyIntegrity) has not yet completed.
	BackupStatusVerifying BackupStatus = "verifying"
)

// IsValid reports whether s is one of the named BackupStatus constants.
func (s BackupStatus) IsValid() bool {
	switch s {
	case BackupStatusSucceeded, BackupStatusFailed, BackupStatusVerifying:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (s BackupStatus) String() string { return string(s) }

// BackupRecord is a single completed (or attempted) backup's metadata
// (task 2's centerpiece alongside EncryptBackup itself): what DataClass
// it covers, when it was taken, where it landed, its integrity hash,
// its size, and its outcome. BackupRecord never stores the backup's
// actual bytes -- those live at Location, addressed by Reference --
// mirroring how packages/provenance.ProvenanceRecord stores a
// ContentHash rather than the artifact itself.
type BackupRecord struct {
	// ID uniquely identifies this backup record.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this backup belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// Class is the DataClass this backup covers.
	Class DataClass `json:"class"`

	// TakenAt is when this backup was taken (the recovery point it
	// represents, for PITR purposes -- see pitr.go).
	TakenAt time.Time `json:"taken_at"`

	// Location classifies where this backup's bytes live.
	Location BackupLocation `json:"location"`

	// Reference addresses the backup artifact within Location (e.g. an
	// object-storage key, an offline-bundle serial number) -- reference
	// only, this package never dereferences it to read bytes.
	Reference string `json:"reference"`

	// IntegrityHash is the hex-encoded cryptographic hash of the
	// backup's encrypted bytes, computed the same way
	// packages/provenance.ProvenanceRecord.ContentHash is (task 8; see
	// integrity.go).
	IntegrityHash string `json:"integrity_hash"`

	// SizeBytes is the size of the backup artifact in bytes.
	SizeBytes int64 `json:"size_bytes"`

	// Encrypted reports whether this backup was wrapped via
	// packages/encryption's EncryptBackup before being written --
	// should be true whenever the governing BackupPolicy's
	// EncryptionRequired is true.
	Encrypted bool `json:"encrypted"`

	// Status is this backup's outcome.
	Status BackupStatus `json:"status"`

	// CreatedBy is the identity.User (or automated job, recorded as
	// uuid.Nil) that initiated this backup.
	CreatedBy uuid.UUID `json:"created_by"`

	// CreatedAt is when this record was written.
	CreatedAt time.Time `json:"created_at"`
}

// Validate checks r for structural well-formedness.
func (r *BackupRecord) Validate() error {
	if r == nil {
		return ErrInvalidRecord
	}
	if r.TenantID == uuid.Nil {
		return wrapf("BackupRecord.Validate", ErrEmptyTenantID)
	}
	if !r.Class.IsValid() {
		return wrapf("BackupRecord.Validate", ErrInvalidDataClass)
	}
	if !r.Location.IsValid() {
		return wrapf("BackupRecord.Validate", ErrInvalidRecord)
	}
	if strings.TrimSpace(r.Reference) == "" {
		return wrapf("BackupRecord.Validate", ErrInvalidRecord)
	}
	if !r.Status.IsValid() {
		return wrapf("BackupRecord.Validate", ErrInvalidRecord)
	}
	if r.Status == BackupStatusSucceeded && strings.TrimSpace(r.IntegrityHash) == "" {
		// A backup cannot be considered successful without a recorded
		// integrity hash to later verify against -- see VerifyIntegrity
		// in integrity.go.
		return wrapf("BackupRecord.Validate", ErrInvalidRecord)
	}
	if r.TakenAt.IsZero() {
		return wrapf("BackupRecord.Validate", ErrInvalidRecord)
	}
	return nil
}
