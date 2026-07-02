package knowledgeapi

import "github.com/YASSERRMD/verdex/packages/gateway"

// NewRouter returns a gateway.Router pre-configured with
// gateway.VersionMiddleware, gateway.RequestIDMiddleware, and
// gateway.Recovery, mounted so every route registered through Handler.
// Routes is automatically served under gateway's current API version
// prefix (gateway.CurrentVersion, "v1" as of this writing). This is the
// intended way to serve a KnowledgeAPI's Handler over HTTP: it guarantees
// every knowledgeapi response is versioned the same way every other
// Verdex HTTP surface is versioned, rather than this package inventing
// its own scheme.
//
// A caller wanting additional gateway middleware (CORS, rate limiting,
// security headers) should call router.Use with the relevant
// gateway.*Middleware functions before calling Handler.Routes, or wrap
// the *gateway.Router returned here with gateway.Chain directly.
func NewRouter() *gateway.Router {
	router := gateway.NewRouter()
	versioned := router.Group("/" + gateway.CurrentVersion.String())
	versioned.Use(gateway.VersionMiddleware, gateway.RequestIDMiddleware, gateway.Recovery)
	return versioned
}
