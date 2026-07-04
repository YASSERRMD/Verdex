package identity_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// okHandler is a trivial handler that always returns 200 OK.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

// newTestUser builds a minimal User suitable for issuing a NoOp token.
func newTestUser(roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:        uuid.New(),
		TenantID:  uuid.New(),
		Email:     "test@example.com",
		Name:      "Test User",
		Roles:     roles,
		Status:    identity.UserStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func TestAuthMiddleware_NoToken_Returns401(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	handler := identity.AuthMiddleware(provider, nil)(okHandler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken_Returns401(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	handler := identity.AuthMiddleware(provider, nil)(okHandler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MalformedAuthHeader_Returns401(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"Basic dXNlcjpwYXNz",
		"Bearer",
		"bearer ",
		"Token abc123",
	}

	provider := &identity.NoOpProvider{}
	handler := identity.AuthMiddleware(provider, nil)(okHandler)

	for _, hdr := range cases {
		hdr := hdr
		t.Run(hdr, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if hdr != "" {
				req.Header.Set("Authorization", hdr)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("header=%q: expected 401, got %d", hdr, rec.Code)
			}
		})
	}
}

func TestAuthMiddleware_ValidToken_Returns200AndUserOnContext(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	user := newTestUser(identity.RoleJudge)
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	var capturedUser *identity.User
	capturingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := identity.UserFromContext(r.Context())
		if !ok {
			t.Error("UserFromContext returned false; expected a user on context")
		}
		capturedUser = u
		w.WriteHeader(http.StatusOK)
	})

	handler := identity.AuthMiddleware(provider, nil)(capturingHandler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if capturedUser == nil {
		t.Fatal("user was not stored on context")
	}
	if capturedUser.ID != user.ID {
		t.Errorf("context user ID = %v; want %v", capturedUser.ID, user.ID)
	}
}

func TestRequireRole_WrongRole_Returns403(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	// User is an advocate, but the route requires a judge.
	user := newTestUser(identity.RoleAdvocate)
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := identity.AuthMiddleware(provider, nil)(
		identity.RequireRole(identity.RoleJudge)(okHandler),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequireRole_CorrectRole_Returns200(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	user := newTestUser(identity.RoleJudge)
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := identity.AuthMiddleware(provider, nil)(
		identity.RequireRole(identity.RoleJudge)(okHandler),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequireRole_OneOfMultipleRoles_Returns200(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	user := newTestUser(identity.RoleClerk) // clerk satisfies judge|clerk requirement
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := identity.AuthMiddleware(provider, nil)(
		identity.RequireRole(identity.RoleJudge, identity.RoleClerk)(okHandler),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePermission_CorrectPermission_Returns200(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	// Admin has PermManageUsers.
	user := newTestUser(identity.RoleAdmin)
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := identity.AuthMiddleware(provider, nil)(
		identity.RequirePermission(identity.PermManageUsers)(okHandler),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestRequirePermission_MissingPermission_Returns403(t *testing.T) {
	t.Parallel()

	provider := &identity.NoOpProvider{}
	// Advocate does not have PermManageUsers.
	user := newTestUser(identity.RoleAdvocate)
	token, err := provider.IssueToken(t.Context(), user)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := identity.AuthMiddleware(provider, nil)(
		identity.RequirePermission(identity.PermManageUsers)(okHandler),
	)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRequirePermission_NoUser_Returns401(t *testing.T) {
	t.Parallel()

	// Call RequirePermission without AuthMiddleware — no user on context.
	handler := identity.RequirePermission(identity.PermViewCase)(okHandler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRequireRole_NoUser_Returns401(t *testing.T) {
	t.Parallel()

	handler := identity.RequireRole(identity.RoleJudge)(okHandler)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}
