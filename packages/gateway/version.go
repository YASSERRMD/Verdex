package gateway

import (
	"fmt"
	"net/http"
	"strings"
)

// APIVersion represents a supported API version.
type APIVersion string

const (
	// VersionV1 is the first stable API version.
	VersionV1 APIVersion = "v1"
	// VersionV2 is the second stable API version.
	VersionV2 APIVersion = "v2"

	// CurrentVersion is the default API version used when no version is specified.
	CurrentVersion APIVersion = VersionV1
)

// supportedVersions is the set of all known, accepted API versions.
var supportedVersions = map[APIVersion]struct{}{
	VersionV1: {},
	VersionV2: {},
}

// ParseVersion converts a raw string (e.g. "v1", "V2") to an APIVersion.
// It returns an error when the string does not match a supported version.
func ParseVersion(raw string) (APIVersion, error) {
	v := APIVersion(strings.ToLower(strings.TrimSpace(raw)))
	if _, ok := supportedVersions[v]; !ok {
		return "", fmt.Errorf("gateway: unsupported API version %q", raw)
	}
	return v, nil
}

// String returns the string representation of the version.
func (v APIVersion) String() string { return string(v) }

// VersionMiddleware extracts the API version from the first path segment
// (e.g. /v1/...) and rejects requests with unsupported versions.
// Supported requests continue to the next handler with the version available
// in the request context via VersionFromContext.
func VersionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract first path segment: "/v1/some/path" -> "v1"
		path := r.URL.Path
		if path == "" || path == "/" {
			// No version segment – default to current version and proceed.
			ctx := withVersion(r.Context(), CurrentVersion)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Strip leading slash and split.
		trimmed := strings.TrimPrefix(path, "/")
		parts := strings.SplitN(trimmed, "/", 2)
		segment := parts[0]

		v, err := ParseVersion(segment)
		if err != nil {
			WriteError(w, &APIError{
				Code:    ErrCodeNotFound,
				Message: fmt.Sprintf("API version %q is not supported", segment),
			})
			return
		}

		ctx := withVersion(r.Context(), v)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
