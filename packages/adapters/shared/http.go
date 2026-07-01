// Package shared contains utilities shared across all cloud provider adapters.
package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

// BuildHTTPClient returns an *http.Client configured with the given timeout.
// A zero timeout means no timeout (not recommended for production).
func BuildHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}
}

// DoRequest performs an HTTP request and returns the response body bytes, the
// HTTP status code, and any transport-level error. A non-2xx status code is
// NOT treated as an error here; callers should call MapHTTPStatus on the
// returned body and status to convert it into a ProviderError.
func DoRequest(ctx context.Context, client *http.Client, method, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("building request: %w", err)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if len(body) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// ParseJSON unmarshals src into dst. It returns a descriptive error that
// includes a prefix identifying the call site.
func ParseJSON(src []byte, dst any) error {
	if err := json.Unmarshal(src, dst); err != nil {
		return fmt.Errorf("parsing JSON response: %w", err)
	}
	return nil
}

// WithRetry calls fn up to maxAttempts times, applying exponential backoff
// between attempts. It stops immediately if ctx is cancelled or if fn returns
// a non-retryable error (i.e. not a temporary network error).
//
// Backoff: 200 ms * 2^attempt, capped at 10 s, with no jitter to keep the
// implementation simple and deterministic in tests.
func WithRetry(ctx context.Context, maxAttempts int, fn func() error) error {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !isRetryable(lastErr) {
			return lastErr
		}
		if attempt == maxAttempts-1 {
			break
		}
		delay := backoffDelay(attempt)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// RetryableError wraps an error and marks it as eligible for retry.
type RetryableError struct {
	Err error
}

func (r *RetryableError) Error() string { return r.Err.Error() }
func (r *RetryableError) Unwrap() error { return r.Err }

func isRetryable(err error) bool {
	var re *RetryableError
	for e := err; e != nil; {
		if _, ok := e.(*RetryableError); ok { //nolint:errorlint
			_ = re
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

func backoffDelay(attempt int) time.Duration {
	base := 200 * time.Millisecond
	exp := time.Duration(math.Pow(2, float64(attempt)))
	d := base * exp
	max := 10 * time.Second
	if d > max {
		d = max
	}
	return d
}
