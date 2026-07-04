package iac

import (
	"strings"
	"time"
)

// InjectionMechanism names how a SecretRef's value reaches a running
// container/process at deploy time. A closed enum: the mechanism
// determines which orchestrator primitive (Kubernetes Secret volume
// vs. env var vs. an external KMS call) a deploy-time tool must
// actually use, so it is a structural property of the deployment, not
// a free-form label.
type InjectionMechanism string

const (
	// InjectionMechanismEnvVar injects the secret as a process
	// environment variable, resolved through
	// packages/config/secrets.go's "env://VAR_NAME" scheme -- see
	// infra/*/docker-compose.yml's environment: blocks, which
	// reference every secret this way.
	InjectionMechanismEnvVar InjectionMechanism = "env_var"

	// InjectionMechanismMountedFile injects the secret as a file
	// mounted into the container's filesystem (a Kubernetes Secret
	// volume, or a bind-mounted file for docker-compose), for values a
	// process is expected to read from disk rather than an env var
	// (e.g. packages/keymanagement.FileProvider's on-disk key material
	// root, composing with that package by reference rather than this
	// package re-implementing file-backed key storage).
	InjectionMechanismMountedFile InjectionMechanism = "mounted_file"

	// InjectionMechanismKMSReference injects the secret indirectly: the
	// deployed process holds only a reference (an ARN, a key ID, a
	// path) and resolves the real value at runtime via a KMS/secrets-
	// manager call, composing with packages/config's documented-but-
	// unimplemented "vault://path#key" VaultResolver placeholder
	// (packages/config/secrets.go) and packages/keymanagement.Provider's
	// documented future cloud-KMS-backed implementation (see
	// packages/keymanagement/provider.go's doc comment on "Implementing
	// a cloud KMS backend") by reference. Neither of those integrations
	// exists yet in this codebase; SecretInjectionPlan only records the
	// *intent* to use this mechanism for a given secret, it does not
	// perform the KMS call itself.
	InjectionMechanismKMSReference InjectionMechanism = "kms_reference"
)

// allInjectionMechanisms is the exhaustive set of recognized
// InjectionMechanism values, used by IsValid.
var allInjectionMechanisms = map[InjectionMechanism]struct{}{
	InjectionMechanismEnvVar:       {},
	InjectionMechanismMountedFile:  {},
	InjectionMechanismKMSReference: {},
}

// IsValid reports whether m is one of the named InjectionMechanism
// constants.
func (m InjectionMechanism) IsValid() bool {
	_, ok := allInjectionMechanisms[m]
	return ok
}

// String satisfies fmt.Stringer.
func (m InjectionMechanism) String() string { return string(m) }

// SecretRef names a single secret this plan injects, by reference
// only -- Name and Reference are strings a human/operator uses to
// locate the real value in whichever backend actually holds it; this
// type never carries the value itself. Mirrors
// packages/compliance.Control.MappedTo's and
// packages/keymanagement.KeyMetadata.WrappedKeyRef's "opaque,
// backend-specific reference, never the material" convention.
type SecretRef struct {
	// Name is the logical secret name a deployment's manifests use to
	// refer to this value (e.g. "VERDEX_DATABASE_DSN",
	// "VERDEX_ENCRYPTION_KEY" -- matching packages/config's
	// VERDEX_-prefixed env var convention and this phase's infra/
	// manifests' env: keys).
	Name string `json:"name"`

	// Mechanism is how this secret's value reaches the deployment.
	Mechanism InjectionMechanism `json:"mechanism"`

	// Reference is the backend-specific locator for where the real
	// value lives: an "env://VAR_NAME" string
	// (packages/config/secrets.go's scheme) for InjectionMechanismEnvVar,
	// a mount path for InjectionMechanismMountedFile, or a KMS key
	// ARN/ID for InjectionMechanismKMSReference. Never the secret value
	// itself -- Validate does not (and cannot) check that this
	// reference actually resolves, only that it is present.
	Reference string `json:"reference"`

	// ComposesWith names, by string tag only (matching
	// packages/compliance.Control.MappedTo's convention exactly), which
	// existing package this secret's storage/rotation actually composes
	// with (e.g. "packages/keymanagement.FileProvider",
	// "packages/encryption.KeySource"). Optional, informational.
	ComposesWith string `json:"composes_with,omitempty"`
}

// Validate checks r for structural well-formedness.
func (r *SecretRef) Validate() error {
	if r == nil {
		return wrapf("Validate", ErrEmptySecretName)
	}
	if strings.TrimSpace(r.Name) == "" {
		return wrapf("Validate", ErrEmptySecretName)
	}
	if !r.Mechanism.IsValid() {
		return wrapf("Validate", ErrInvalidSecretMechanism)
	}
	if strings.TrimSpace(r.Reference) == "" {
		return wrapf("Validate", ErrEmptySecretName)
	}
	return nil
}

// SecretInjectionPlan describes which secrets get injected into which
// Tier via which InjectionMechanism (task 6) -- never the secret
// values themselves. This type is a deployment-time declaration only;
// it composes with packages/keymanagement (Phase 076) and
// packages/encryption (Phase 075) for how a referenced secret is
// actually stored/rotated/decrypted, by reference (SecretRef.ComposesWith),
// rather than reimplementing any secret storage, KMS client, or
// envelope-encryption logic itself.
type SecretInjectionPlan struct {
	// DeploymentID identifies the deployment this plan governs,
	// matching packages/iac.DeploymentProfile.DeploymentID.
	DeploymentID string `json:"deployment_id"`

	// Tier is the deployment tier this plan targets. Different tiers
	// favor different mechanisms in practice (see
	// DefaultPlanForTier): a cloud deployment typically defers to a KMS
	// reference, an on-prem deployment to a mounted file or env var
	// backed by an operator-supplied .env, and an air-gapped deployment
	// exclusively to env var/mounted file -- InjectionMechanismKMSReference
	// is never valid for TierAirgapped, since there is no reachable KMS
	// by definition (composing with packages/airgapped.NetworkPolicy's
	// loopback-only posture).
	Tier Tier `json:"tier"`

	// Secrets lists every secret this plan injects.
	Secrets []SecretRef `json:"secrets"`

	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Validate checks p for internal consistency: every SecretRef must
// itself validate, and InjectionMechanismKMSReference is rejected
// outright for TierAirgapped (task 6's tier-appropriate-mechanism
// requirement, composing with packages/airgapped's zero-egress
// guarantee -- an air-gapped deployment has no reachable KMS to defer
// to, so a plan claiming one is a structural error, not merely an
// operational risk).
func (p *SecretInjectionPlan) Validate() error {
	if p == nil {
		return wrapf("Validate", ErrEmptyDeploymentID)
	}
	if strings.TrimSpace(p.DeploymentID) == "" {
		return wrapf("Validate", ErrEmptyDeploymentID)
	}
	if !p.Tier.IsValid() {
		return wrapf("Validate", ErrInvalidTier)
	}
	for i := range p.Secrets {
		ref := &p.Secrets[i]
		if err := ref.Validate(); err != nil {
			return wrapf("Validate", err)
		}
		if p.Tier == TierAirgapped && ref.Mechanism == InjectionMechanismKMSReference {
			return wrapf("Validate", ErrInvalidSecretMechanism)
		}
	}
	return nil
}

// DefaultPlanForTier returns a SecretInjectionPlan seeded with the
// mechanism this phase's own infra/<tier>/ manifests actually use for
// the two secrets every tier's gateway/router service needs
// (VERDEX_DATABASE_DSN, VERDEX_ENCRYPTION_KEY), composing with
// packages/keymanagement/packages/encryption by reference:
//
//   - TierCloud: InjectionMechanismKMSReference, composing with
//     "packages/keymanagement.Provider" (a future cloud-KMS-backed
//     implementation -- see packages/keymanagement/provider.go's doc
//     comment) by reference, since infra/cloud/deployment.yaml already
//     defers secret material to a Kubernetes Secret a real deployment
//     backs with its cloud provider's KMS.
//   - TierOnPrem: InjectionMechanismEnvVar, composing with
//     "packages/config.envResolver" (packages/config/secrets.go) by
//     reference, matching infra/onprem/docker-compose.yml's
//     env://VAR_NAME convention -- no managed secret store to defer
//     to.
//   - TierAirgapped: InjectionMechanismEnvVar as well, composing with
//     "packages/keymanagement.FileProvider" by reference for
//     VERDEX_ENCRYPTION_KEY specifically (the mandated offline key
//     source packages/airgapped.Profile requires), matching
//     infra/airgapped/docker-compose.yml's env://VAR_NAME convention.
//
// A caller is free to construct a SecretInjectionPlan directly instead
// of using this constructor; it exists only to seed the common case
// this phase's own manifests already establish.
func DefaultPlanForTier(deploymentID string, tier Tier) (SecretInjectionPlan, error) {
	if strings.TrimSpace(deploymentID) == "" {
		return SecretInjectionPlan{}, ErrEmptyDeploymentID
	}
	if !tier.IsValid() {
		return SecretInjectionPlan{}, ErrInvalidTier
	}

	now := time.Now().UTC()
	plan := SecretInjectionPlan{
		DeploymentID: deploymentID,
		Tier:         tier,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	switch tier {
	case TierCloud:
		plan.Secrets = []SecretRef{
			{Name: "VERDEX_DATABASE_DSN", Mechanism: InjectionMechanismKMSReference, Reference: "kms://verdex-cloud/database-dsn", ComposesWith: "packages/keymanagement.Provider"},
			{Name: "VERDEX_ENCRYPTION_KEY", Mechanism: InjectionMechanismKMSReference, Reference: "kms://verdex-cloud/encryption-key", ComposesWith: "packages/encryption.KeySource"},
		}
	case TierOnPrem:
		plan.Secrets = []SecretRef{
			{Name: "VERDEX_DATABASE_DSN", Mechanism: InjectionMechanismEnvVar, Reference: "env://VERDEX_DATABASE_DSN", ComposesWith: "packages/config.envResolver"},
			{Name: "VERDEX_ENCRYPTION_KEY", Mechanism: InjectionMechanismEnvVar, Reference: "env://VERDEX_ENCRYPTION_KEY", ComposesWith: "packages/encryption.KeySource"},
		}
	case TierAirgapped:
		plan.Secrets = []SecretRef{
			{Name: "VERDEX_DATABASE_DSN", Mechanism: InjectionMechanismEnvVar, Reference: "env://VERDEX_DATABASE_DSN", ComposesWith: "packages/config.envResolver"},
			{Name: "VERDEX_ENCRYPTION_KEY", Mechanism: InjectionMechanismMountedFile, Reference: "/var/lib/verdex/keys", ComposesWith: "packages/keymanagement.FileProvider"},
		}
	}

	return plan, nil
}
