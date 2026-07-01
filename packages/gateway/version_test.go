package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/YASSERRMD/verdex/packages/gateway"
)

func TestParseVersion_valid(t *testing.T) {
	cases := []struct {
		input string
		want  gateway.APIVersion
	}{
		{"v1", gateway.VersionV1},
		{"V1", gateway.VersionV1},
		{"v2", gateway.VersionV2},
		{"V2", gateway.VersionV2},
	}

	for _, tc := range cases {
		got, err := gateway.ParseVersion(tc.input)
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tc.input, err)
		}
		if got != tc.want {
			t.Errorf("ParseVersion(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseVersion_invalid(t *testing.T) {
	cases := []string{"v3", "v0", "", "1", "version1", "latest"}

	for _, raw := range cases {
		_, err := gateway.ParseVersion(raw)
		if err == nil {
			t.Errorf("ParseVersion(%q) expected error, got nil", raw)
		}
	}
}

func TestVersionMiddleware_allowsV1(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := gateway.VersionMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called for /v1/cases")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestVersionMiddleware_allowsV2(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := gateway.VersionMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v2/cases", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called for /v2/cases")
	}
}

func TestVersionMiddleware_rejectsUnknownVersion(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not have been called")
		w.WriteHeader(http.StatusOK)
	})

	handler := gateway.VersionMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/v99/cases", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown version, got %d", rec.Code)
	}
}

func TestVersionMiddleware_noVersionSegment(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		// Default version should be set in context.
		v := gateway.VersionFromContext(r.Context())
		if v != gateway.CurrentVersion {
			t.Errorf("expected current version %v, got %v", gateway.CurrentVersion, v)
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := gateway.VersionMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called for /")
	}
}

func TestVersionFromContext_default(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	v := gateway.VersionFromContext(req.Context())
	if v != gateway.CurrentVersion {
		t.Errorf("expected %v, got %v", gateway.CurrentVersion, v)
	}
}
