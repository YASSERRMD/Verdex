package tenancy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// AppRoleName is the dedicated PostgreSQL role application traffic
// must connect as for the Row-Level Security policy in
// migrations/000003_enable_rls_deployments.up.sql (and any later
// RLS-protected table) to actually provide isolation. It is created,
// non-superuser and non-BYPASSRLS, by
// migrations/000005_create_app_role.up.sql.
const AppRoleName = "verdex_app"

// GenerateAppRolePassword returns a fresh, cryptographically random
// hex-encoded password suitable for BootstrapAppRolePassword. Hex
// encoding guarantees the result contains only [0-9a-f], so it can
// never itself carry a SQL metacharacter — the same "cannot carry an
// injection payload by construction" property scope.go relies on for
// interpolating a uuid.UUID.String() into SET LOCAL.
func GenerateAppRolePassword() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("tenancy: GenerateAppRolePassword: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// BootstrapAppRolePassword sets (or rotates) AppRoleName's login
// password to password, by calling the verdex_set_app_role_password
// SQL function created in migrations/000005_create_app_role.up.sql.
//
// exec must be a connection/transaction with privilege to invoke that
// function — in practice, the same elevated bootstrap connection used
// to run schema migrations, never a connection already authenticated
// as AppRoleName itself (the function's grants are set up so
// AppRoleName cannot call it). password is passed as an ordinary bound
// query parameter; the function itself performs the necessary
// literal-quoting server-side before executing the ALTER ROLE
// statement, so no client-side string concatenation of password into
// DDL text ever occurs here.
func BootstrapAppRolePassword(ctx context.Context, exec persistence.Executor, password string) error {
	if exec == nil {
		return fmt.Errorf("tenancy: BootstrapAppRolePassword: exec must not be nil")
	}
	if password == "" {
		return fmt.Errorf("tenancy: BootstrapAppRolePassword: password must not be empty")
	}

	if _, err := exec.Exec(ctx, "SELECT verdex_set_app_role_password($1)", password); err != nil {
		return fmt.Errorf("tenancy: BootstrapAppRolePassword: %w", err)
	}
	return nil
}

// BuildAppRoleDSN rewrites baseDSN (a standard PostgreSQL connection
// URL, as returned by e.g. testcontainers or assembled from
// config.Config.Database) to authenticate as AppRoleName with
// password instead of whatever credentials baseDSN originally
// carried, preserving host, port, database, and query parameters
// (sslmode, etc.) unchanged.
//
// This exists so callers (bootstrap tooling, tests) that only have a
// superuser/admin DSN on hand can derive the DSN application code
// should actually use, without hand-assembling a connection string.
func BuildAppRoleDSN(baseDSN, password string) (string, error) {
	u, err := url.Parse(baseDSN)
	if err != nil {
		return "", fmt.Errorf("tenancy: BuildAppRoleDSN: parse base DSN: %w", err)
	}
	u.User = url.UserPassword(AppRoleName, password)
	return u.String(), nil
}

// VerifyRLSEnforceable checks that the role pool is currently
// authenticated as does not have the BYPASSRLS attribute and is not a
// superuser — either of which would cause PostgreSQL to silently
// ignore every Row-Level Security policy for that connection,
// regardless of FORCE ROW LEVEL SECURITY, defeating the isolation
// guarantee WithTenantScope depends on.
//
// Call this once at service startup, against the pool the service
// will actually use for tenant-scoped operations (i.e. one
// authenticated as AppRoleName in every real deployment), and fail
// startup if it returns an error. It is intentionally not called
// automatically inside WithTenantScope: that runs per-request, and
// this check's query cost is only worth paying once per process
// lifetime, not once per request.
func VerifyRLSEnforceable(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("tenancy: VerifyRLSEnforceable: pool must not be nil")
	}

	var bypasses bool
	err := pool.QueryRow(ctx,
		"SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user",
	).Scan(&bypasses)
	if err != nil {
		return fmt.Errorf("tenancy: VerifyRLSEnforceable: query current role: %w", err)
	}

	if bypasses {
		return fmt.Errorf(
			"tenancy: VerifyRLSEnforceable: the connecting role has BYPASSRLS or is a superuser; " +
				"Row-Level Security policies are silently ignored for this connection regardless of " +
				"FORCE ROW LEVEL SECURITY, so tenant isolation is NOT enforced; " +
				"reconfigure cfg.Database.DSN to authenticate as the '" + AppRoleName + "' role instead " +
				"(see BootstrapAppRolePassword / BuildAppRoleDSN)",
		)
	}
	return nil
}
