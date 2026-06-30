package observability

import (
	"context"
	"encoding/json"
	"net/http"
)

// Checker reports whether a single readiness dependency (a database
// connection, a downstream service, a cache, etc.) is currently
// healthy. A nil return means healthy; any error is treated as
// unhealthy and its message is surfaced in the /readyz response body.
type Checker func(ctx context.Context) error

// NamedChecker pairs a Checker with a human-readable name used to
// identify it in the /readyz JSON response.
type NamedChecker struct {
	Name    string
	Checker Checker
}

// LivenessHandler returns an http.Handler for a liveness probe
// ("/healthz" by convention). It always responds 200 OK with a small
// JSON body as long as the process is up and able to handle HTTP
// requests at all - it never inspects external dependencies, since
// liveness is meant to answer "is this process alive", not "is this
// process useful".
func LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeHealthJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})
}

// ReadinessHandler returns an http.Handler for a readiness probe
// ("/readyz" by convention) that runs every given NamedChecker on each
// request. If all checkers succeed it responds 200 OK; if any fail it
// responds 503 Service Unavailable with a JSON body listing every
// failure by name. Checkers run sequentially in the order given,
// against the incoming request's context (so a checker can honor
// caller cancellation/timeouts).
func ReadinessHandler(checkers ...NamedChecker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		failures := map[string]string{}
		for _, c := range checkers {
			if c.Checker == nil {
				continue
			}
			if err := c.Checker(r.Context()); err != nil {
				failures[c.Name] = err.Error()
			}
		}

		if len(failures) > 0 {
			writeHealthJSON(w, http.StatusServiceUnavailable, healthResponse{
				Status:   "unavailable",
				Failures: failures,
			})
			return
		}

		writeHealthJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	})
}

// healthResponse is the JSON body shape returned by both health
// endpoints.
type healthResponse struct {
	Status   string            `json:"status"`
	Failures map[string]string `json:"failures,omitempty"`
}

func writeHealthJSON(w http.ResponseWriter, statusCode int, body healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	// Encoding errors here would mean healthResponse itself is
	// unmarshalable, which cannot happen for this fixed, all-string
	// shape; nothing meaningful to do with the error besides ignore it
	// since headers/status are already written.
	_ = json.NewEncoder(w).Encode(body)
}
