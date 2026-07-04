package iac

import (
	"errors"
	"testing"
)

func TestInjectionMechanismIsValid(t *testing.T) {
	tests := []struct {
		mechanism InjectionMechanism
		want      bool
	}{
		{InjectionMechanismEnvVar, true},
		{InjectionMechanismMountedFile, true},
		{InjectionMechanismKMSReference, true},
		{InjectionMechanism("carrier_pigeon"), false},
	}
	for _, tc := range tests {
		if got := tc.mechanism.IsValid(); got != tc.want {
			t.Errorf("InjectionMechanism(%q).IsValid() = %v, want %v", tc.mechanism, got, tc.want)
		}
	}
}

func TestSecretRefValidate(t *testing.T) {
	valid := SecretRef{Name: "VERDEX_DATABASE_DSN", Mechanism: InjectionMechanismEnvVar, Reference: "env://VERDEX_DATABASE_DSN"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid SecretRef failed validation: %v", err)
	}

	noName := valid
	noName.Name = ""
	if err := noName.Validate(); !errors.Is(err, ErrEmptySecretName) {
		t.Errorf("no name: got %v, want ErrEmptySecretName", err)
	}

	badMechanism := valid
	badMechanism.Mechanism = InjectionMechanism("bogus")
	if err := badMechanism.Validate(); !errors.Is(err, ErrInvalidSecretMechanism) {
		t.Errorf("bad mechanism: got %v, want ErrInvalidSecretMechanism", err)
	}

	noReference := valid
	noReference.Reference = ""
	if err := noReference.Validate(); !errors.Is(err, ErrEmptySecretName) {
		t.Errorf("no reference: got %v, want ErrEmptySecretName", err)
	}

	var nilRef *SecretRef
	if err := nilRef.Validate(); !errors.Is(err, ErrEmptySecretName) {
		t.Errorf("nil ref: got %v, want ErrEmptySecretName", err)
	}
}

func TestSecretInjectionPlanValidate(t *testing.T) {
	plan := SecretInjectionPlan{
		DeploymentID: "deployment-1",
		Tier:         TierCloud,
		Secrets: []SecretRef{
			{Name: "VERDEX_DATABASE_DSN", Mechanism: InjectionMechanismKMSReference, Reference: "kms://verdex-cloud/database-dsn"},
		},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid plan failed validation: %v", err)
	}

	noDeployment := plan
	noDeployment.DeploymentID = ""
	if err := noDeployment.Validate(); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("no deployment id: got %v, want ErrEmptyDeploymentID", err)
	}

	badTier := plan
	badTier.Tier = Tier("bogus")
	if err := badTier.Validate(); !errors.Is(err, ErrInvalidTier) {
		t.Errorf("bad tier: got %v, want ErrInvalidTier", err)
	}

	badSecret := plan
	badSecret.Secrets = []SecretRef{{Name: "", Mechanism: InjectionMechanismEnvVar, Reference: "env://X"}}
	if err := badSecret.Validate(); !errors.Is(err, ErrEmptySecretName) {
		t.Errorf("bad secret: got %v, want ErrEmptySecretName", err)
	}

	var nilPlan *SecretInjectionPlan
	if err := nilPlan.Validate(); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("nil plan: got %v, want ErrEmptyDeploymentID", err)
	}
}

// TestSecretInjectionPlanValidate_KMSReferenceRejectedForAirgapped
// proves the tier-appropriate-mechanism rule: an air-gapped
// deployment has no reachable KMS, so a plan claiming
// InjectionMechanismKMSReference for that tier must fail validation.
func TestSecretInjectionPlanValidate_KMSReferenceRejectedForAirgapped(t *testing.T) {
	plan := SecretInjectionPlan{
		DeploymentID: "deployment-1",
		Tier:         TierAirgapped,
		Secrets: []SecretRef{
			{Name: "VERDEX_ENCRYPTION_KEY", Mechanism: InjectionMechanismKMSReference, Reference: "kms://somewhere/key"},
		},
	}
	if err := plan.Validate(); !errors.Is(err, ErrInvalidSecretMechanism) {
		t.Errorf("got %v, want ErrInvalidSecretMechanism", err)
	}

	// The same mechanism is fine for TierCloud.
	plan.Tier = TierCloud
	if err := plan.Validate(); err != nil {
		t.Errorf("KMS reference should be valid for TierCloud: %v", err)
	}
}

func TestDefaultPlanForTier(t *testing.T) {
	for _, tier := range []Tier{TierCloud, TierOnPrem, TierAirgapped} {
		t.Run(string(tier), func(t *testing.T) {
			plan, err := DefaultPlanForTier("deployment-1", tier)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := plan.Validate(); err != nil {
				t.Fatalf("DefaultPlanForTier(%s) produced an invalid plan: %v", tier, err)
			}
			if len(plan.Secrets) == 0 {
				t.Fatalf("DefaultPlanForTier(%s) produced no secrets", tier)
			}
			for _, s := range plan.Secrets {
				if tier == TierAirgapped && s.Mechanism == InjectionMechanismKMSReference {
					t.Errorf("air-gapped tier must never use InjectionMechanismKMSReference, got secret %+v", s)
				}
			}
		})
	}

	if _, err := DefaultPlanForTier("", TierCloud); !errors.Is(err, ErrEmptyDeploymentID) {
		t.Errorf("empty deployment id: got %v, want ErrEmptyDeploymentID", err)
	}
	if _, err := DefaultPlanForTier("deployment-1", Tier("bogus")); !errors.Is(err, ErrInvalidTier) {
		t.Errorf("bad tier: got %v, want ErrInvalidTier", err)
	}
}
