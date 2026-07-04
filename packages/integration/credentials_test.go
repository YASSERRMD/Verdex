package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestConnectorCredentialsValidate(t *testing.T) {
	t.Parallel()

	tenantID := uuid.New()

	tests := []struct {
		name    string
		creds   integration.ConnectorCredentials
		wantErr error
	}{
		{
			name: "valid api key",
			creds: integration.ConnectorCredentials{
				TenantID:  tenantID,
				Kind:      integration.CredentialKindAPIKey,
				SecretRef: "keymanagement:key-123",
			},
		},
		{
			name: "valid none",
			creds: integration.ConnectorCredentials{
				TenantID: tenantID,
				Kind:     integration.CredentialKindNone,
			},
		},
		{
			name: "valid oauth",
			creds: integration.ConnectorCredentials{
				TenantID:  tenantID,
				Kind:      integration.CredentialKindOAuthClientCredentials,
				SecretRef: "keymanagement:client-secret-1",
				ClientID:  "verdex-client",
				TokenURL:  "https://auth.example.test/oauth/token",
			},
		},
		{
			name: "missing tenant",
			creds: integration.ConnectorCredentials{
				Kind:      integration.CredentialKindAPIKey,
				SecretRef: "ref",
			},
			wantErr: integration.ErrEmptyTenantID,
		},
		{
			name: "invalid kind",
			creds: integration.ConnectorCredentials{
				TenantID:  tenantID,
				Kind:      "bogus",
				SecretRef: "ref",
			},
			wantErr: integration.ErrInvalidCredentials,
		},
		{
			name: "api key missing secret ref",
			creds: integration.ConnectorCredentials{
				TenantID: tenantID,
				Kind:     integration.CredentialKindAPIKey,
			},
			wantErr: integration.ErrInvalidCredentials,
		},
		{
			name: "oauth missing client id",
			creds: integration.ConnectorCredentials{
				TenantID:  tenantID,
				Kind:      integration.CredentialKindOAuthClientCredentials,
				SecretRef: "ref",
				TokenURL:  "https://auth.example.test/oauth/token",
			},
			wantErr: integration.ErrInvalidCredentials,
		},
		{
			name: "oauth missing token url",
			creds: integration.ConnectorCredentials{
				TenantID:  tenantID,
				Kind:      integration.CredentialKindOAuthClientCredentials,
				SecretRef: "ref",
				ClientID:  "verdex-client",
			},
			wantErr: integration.ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.creds.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestConnectorCredentialsNilReceiver(t *testing.T) {
	t.Parallel()
	var c *integration.ConnectorCredentials
	if err := c.Validate(); !errors.Is(err, integration.ErrInvalidCredentials) {
		t.Fatalf("Validate() on nil receiver = %v, want ErrInvalidCredentials", err)
	}
}

func TestAuthorizeCredentialsStructuralOnly(t *testing.T) {
	t.Parallel()

	creds := integration.ConnectorCredentials{
		TenantID:  uuid.New(),
		Kind:      integration.CredentialKindAPIKey,
		SecretRef: "keymanagement:key-abc",
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	out, err := integration.AuthorizeCredentials(context.Background(), creds, nil, now)
	if err != nil {
		t.Fatalf("AuthorizeCredentials() error = %v", err)
	}
	if !out.LastVerifiedAt.Equal(now) {
		t.Errorf("LastVerifiedAt = %v, want %v", out.LastVerifiedAt, now)
	}
}

func TestAuthorizeCredentialsWithVerifyFunc(t *testing.T) {
	t.Parallel()

	creds := integration.ConnectorCredentials{
		TenantID:  uuid.New(),
		Kind:      integration.CredentialKindAPIKey,
		SecretRef: "keymanagement:key-abc",
	}

	t.Run("verify succeeds", func(t *testing.T) {
		t.Parallel()
		called := false
		verify := func(_ context.Context, c integration.ConnectorCredentials) error {
			called = true
			if c.SecretRef != creds.SecretRef {
				t.Errorf("verify received SecretRef %q, want %q", c.SecretRef, creds.SecretRef)
			}
			return nil
		}
		_, err := integration.AuthorizeCredentials(context.Background(), creds, verify, time.Now())
		if err != nil {
			t.Fatalf("AuthorizeCredentials() error = %v", err)
		}
		if !called {
			t.Error("verify was never invoked")
		}
	})

	t.Run("verify fails", func(t *testing.T) {
		t.Parallel()
		wantErr := errors.New("upstream rejected credential")
		verify := func(_ context.Context, _ integration.ConnectorCredentials) error {
			return wantErr
		}
		_, err := integration.AuthorizeCredentials(context.Background(), creds, verify, time.Now())
		if !errors.Is(err, wantErr) {
			t.Fatalf("AuthorizeCredentials() error = %v, want wrapping %v", err, wantErr)
		}
	})

	t.Run("invalid credentials never reach verify", func(t *testing.T) {
		t.Parallel()
		called := false
		verify := func(_ context.Context, _ integration.ConnectorCredentials) error {
			called = true
			return nil
		}
		bad := integration.ConnectorCredentials{Kind: integration.CredentialKindAPIKey}
		_, err := integration.AuthorizeCredentials(context.Background(), bad, verify, time.Now())
		if err == nil {
			t.Fatal("expected validation error")
		}
		if called {
			t.Error("verify should not be invoked when structural validation fails")
		}
	})
}
