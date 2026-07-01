package tenancy_test

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// rejectingExecutor implements persistence.Executor and fails the
// test immediately if any of its methods are ever invoked. It is used
// to prove that validation errors (e.g. an empty password) are
// returned before any statement would be sent to the database.
type rejectingExecutor struct {
	t *testing.T
}

func (r rejectingExecutor) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	r.t.Helper()
	r.t.Fatal("Exec must not be called")
	return pgconn.CommandTag{}, nil
}

func (r rejectingExecutor) Query(context.Context, string, ...any) (pgx.Rows, error) {
	r.t.Helper()
	r.t.Fatal("Query must not be called")
	return nil, nil
}

func (r rejectingExecutor) QueryRow(context.Context, string, ...any) pgx.Row {
	r.t.Helper()
	r.t.Fatal("QueryRow must not be called")
	return nil
}

func TestGenerateAppRolePassword_ProducesDistinctHexValues(t *testing.T) {
	a, err := tenancy.GenerateAppRolePassword()
	if err != nil {
		t.Fatalf("GenerateAppRolePassword: %v", err)
	}
	b, err := tenancy.GenerateAppRolePassword()
	if err != nil {
		t.Fatalf("GenerateAppRolePassword: %v", err)
	}

	if a == b {
		t.Fatal("expected two calls to produce different passwords")
	}
	if len(a) != 64 { // 32 random bytes, hex-encoded
		t.Fatalf("expected a 64-character hex password, got length %d", len(a))
	}
	if _, err := hex.DecodeString(a); err != nil {
		t.Fatalf("expected password to be valid hex, got %q: %v", a, err)
	}
}

func TestBuildAppRoleDSN_RewritesCredentialsOnly(t *testing.T) {
	dsn, err := tenancy.BuildAppRoleDSN("postgres://original_user:original_pass@localhost:5432/verdex?sslmode=disable", "new-secret-password")
	if err != nil {
		t.Fatalf("BuildAppRoleDSN: %v", err)
	}

	want := "postgres://verdex_app:new-secret-password@localhost:5432/verdex?sslmode=disable"
	if dsn != want {
		t.Fatalf("expected %q, got %q", want, dsn)
	}
}

func TestBuildAppRoleDSN_RejectsUnparseableDSN(t *testing.T) {
	if _, err := tenancy.BuildAppRoleDSN("://not a valid url", "password"); err == nil {
		t.Fatal("expected an error for an unparseable base DSN, got nil")
	}
}

func TestBootstrapAppRolePassword_RejectsNilExecutor(t *testing.T) {
	if err := tenancy.BootstrapAppRolePassword(context.Background(), nil, "password"); err == nil {
		t.Fatal("expected an error for a nil executor, got nil")
	}
}

func TestBootstrapAppRolePassword_RejectsEmptyPassword(t *testing.T) {
	// A non-nil, never-invoked stub is enough: the empty-password check
	// must reject before exec.Exec is ever called.
	if err := tenancy.BootstrapAppRolePassword(context.Background(), rejectingExecutor{t}, ""); err == nil {
		t.Fatal("expected an error for an empty password, got nil")
	}
}

func TestVerifyRLSEnforceable_RejectsNilPool(t *testing.T) {
	if err := tenancy.VerifyRLSEnforceable(context.Background(), nil); err == nil {
		t.Fatal("expected an error for a nil pool, got nil")
	}
}
