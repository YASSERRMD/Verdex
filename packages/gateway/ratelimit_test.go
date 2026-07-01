package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/gateway"
)

func TestInMemoryRateLimiter_allowsUnderLimit(t *testing.T) {
	limiter := gateway.NewInMemoryRateLimiter(5, time.Minute)

	for i := range 5 {
		if !limiter.Allow("client-1") {
			t.Errorf("request %d should have been allowed", i+1)
		}
	}
}

func TestInMemoryRateLimiter_rejectsOverLimit(t *testing.T) {
	limiter := gateway.NewInMemoryRateLimiter(3, time.Minute)

	for range 3 {
		limiter.Allow("client-2")
	}

	if limiter.Allow("client-2") {
		t.Error("4th request should have been rejected")
	}
}

func TestInMemoryRateLimiter_separateKeys(t *testing.T) {
	limiter := gateway.NewInMemoryRateLimiter(2, time.Minute)

	limiter.Allow("a")
	limiter.Allow("a")

	// Key "a" is now at limit, but "b" should still be free.
	if !limiter.Allow("b") {
		t.Error("key 'b' should be allowed even though 'a' is at limit")
	}

	if limiter.Allow("a") {
		t.Error("key 'a' should be rejected")
	}
}

func TestInMemoryRateLimiter_slidingWindowExpiry(t *testing.T) {
	// Use a very short window to allow testing expiry.
	limiter := gateway.NewInMemoryRateLimiter(2, 50*time.Millisecond)

	limiter.Allow("c")
	limiter.Allow("c")

	if limiter.Allow("c") {
		t.Error("3rd request should be rejected within window")
	}

	// Wait for the window to expire.
	time.Sleep(60 * time.Millisecond)

	if !limiter.Allow("c") {
		t.Error("request after window expiry should be allowed")
	}
}

func TestRateLimitMiddleware_allowsUnderLimit(t *testing.T) {
	limiter := gateway.NewInMemoryRateLimiter(10, time.Minute)
	mw := gateway.RateLimitMiddleware(limiter, func(r *http.Request) string {
		return "fixed-key"
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)

	for range 5 {
		req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	}
}

func TestRateLimitMiddleware_rejectsOverLimit(t *testing.T) {
	limiter := gateway.NewInMemoryRateLimiter(3, time.Minute)
	mw := gateway.RateLimitMiddleware(limiter, func(r *http.Request) string {
		return "throttled-client"
	})

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := mw(inner)

	for range 3 {
		req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// 4th request should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/v1/cases", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
}
