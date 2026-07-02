package knowledgeapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/YASSERRMD/verdex/packages/gateway"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

// Handler adapts a KnowledgeAPI onto packages/gateway's HTTP conventions:
// versioned routes (mounted under gateway's /v1 prefix via
// gateway.VersionMiddleware), the standard gateway.Response[T]/
// gateway.ErrorResponse envelopes, and gateway.ParsePagination's
// page/per_page query parameters. Handler is a thin adapter only — every
// method delegates immediately to the matching KnowledgeAPI method; no
// business logic lives here.
//
// Handler does not itself apply identity.AuthMiddleware or
// identity.RequirePermission: composing those is the caller's
// responsibility (see Routes), because only the caller knows which
// identity.Provider/SessionStore to authenticate against. Every
// KnowledgeAPI method still independently enforces identity.PermViewCase
// via this package's own authorize helper (access.go), so a caller that
// forgets to wire RequirePermission does not bypass access control — it
// only loses the earlier, cheaper 401/403 short-circuit RequirePermission
// would otherwise provide.
type Handler struct {
	api *KnowledgeAPI
}

// NewHandler wraps api as an HTTP Handler. Returns ErrNilService if api is
// nil.
func NewHandler(api *KnowledgeAPI) (*Handler, error) {
	if api == nil {
		return nil, ErrNilService
	}
	return &Handler{api: api}, nil
}

// Routes registers this Handler's endpoints onto router under the given
// case-scoped path prefix (e.g. "/cases/{caseID}" is not pattern-matched
// by gateway.Router — callers pass a fixed prefix per case-scoped Handler
// instance, consistent with KnowledgeAPI itself being constructed
// per-case). requirePermission, if non-nil, is applied as gateway
// Middleware around every route (typically identity.RequirePermission
// wrapped to satisfy gateway.Middleware's signature); pass nil to skip
// (e.g. in tests that construct an already-authenticated context
// directly).
func (h *Handler) Routes(router *gateway.Router, requirePermission gateway.Middleware) {
	handlers := map[string]http.HandlerFunc{
		"/tree":              h.handleGetTree,
		"/nodes":             h.handleGetNode,
		"/paths":             h.handleLookupPaths,
		"/retrieve":          h.handleRetrieve,
		"/citations":         h.handleResolveCitation,
		"/validation-status": h.handleValidationStatus,
	}
	for pattern, fn := range handlers {
		if requirePermission != nil {
			router.Handle(pattern, requirePermission(fn))
			continue
		}
		router.HandleFunc(pattern, fn)
	}
}

func (h *Handler) handleGetTree(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := gateway.ParsePagination(r)
	if err != nil {
		gateway.WriteErrorWithRequest(w, r, err)
		return
	}

	resp, err := h.api.GetTree(r.Context(), GetTreeRequest{
		CaseID:         h.api.caseID,
		NodeTypeFilter: r.URL.Query().Get("node_type"),
		Page:           PageRequest{Page: page, PerPage: perPage},
	})
	writeResult(w, r, resp, &resp.Meta, err)
}

func (h *Handler) handleGetNode(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.GetNode(r.Context(), GetNodeRequest{
		CaseID: h.api.caseID,
		NodeID: r.URL.Query().Get("node_id"),
	})
	writeResult(w, r, resp, nil, err)
}

func (h *Handler) handleLookupPaths(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := gateway.ParsePagination(r)
	if err != nil {
		gateway.WriteErrorWithRequest(w, r, err)
		return
	}

	maxDepth := 0
	if raw := r.URL.Query().Get("max_depth"); raw != "" {
		maxDepth, _ = strconv.Atoi(raw)
	}

	resp, err := h.api.LookupPaths(r.Context(), LookupPathsRequest{
		CaseID:     h.api.caseID,
		FromNodeID: r.URL.Query().Get("from_node_id"),
		EdgeType:   r.URL.Query().Get("edge_type"),
		MaxDepth:   maxDepth,
		Page:       PageRequest{Page: page, PerPage: perPage},
	})
	writeResult(w, r, resp, &resp.Meta, err)
}

func (h *Handler) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	page, perPage, err := gateway.ParsePagination(r)
	if err != nil {
		gateway.WriteErrorWithRequest(w, r, err)
		return
	}

	var body RetrieveRequest
	if decodeErr := gateway.ValidateRequest(r, &body); decodeErr != nil {
		gateway.WriteValidationError(w, r, decodeErr)
		return
	}
	body.CaseID = h.api.caseID
	body.Page = PageRequest{Page: page, PerPage: perPage}

	resp, err := h.api.Retrieve(r.Context(), body)
	writeResult(w, r, resp, &resp.Meta, err)
}

func (h *Handler) handleResolveCitation(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ResolveCitation(r.Context(), ResolveCitationRequest{
		CaseID: h.api.caseID,
		NodeID: r.URL.Query().Get("node_id"),
	})
	writeResult(w, r, resp, nil, err)
}

func (h *Handler) handleValidationStatus(w http.ResponseWriter, r *http.Request) {
	resp, err := h.api.ValidationStatus(r.Context(), ValidationStatusRequest{
		CaseID: h.api.caseID,
	})
	writeResult(w, r, resp, nil, err)
}

// writeResult writes data as a gateway.Response[T] envelope, or maps err
// to the appropriate gateway.APIError otherwise.
func writeResult[T any](w http.ResponseWriter, r *http.Request, data T, meta *PageMeta, err error) {
	if err != nil {
		gateway.WriteErrorWithRequest(w, r, toAPIError(err))
		return
	}
	if meta != nil {
		gateway.OKWithMeta(w, r, data, &gateway.PaginationMeta{
			Page:       meta.Page,
			PerPage:    meta.PerPage,
			Total:      meta.Total,
			TotalPages: meta.TotalPages,
		})
		return
	}
	gateway.OK(w, r, data)
}

// toAPIError maps a knowledgeapi/knowledgeisolation sentinel error to a
// gateway.APIError with the appropriate HTTP status code, so this
// package's access-control and isolation errors surface with the correct
// semantics (401/403/404/400) instead of collapsing to a generic 500.
func toAPIError(err error) *gateway.APIError {
	switch {
	case errors.Is(err, ErrUnauthenticated):
		return &gateway.APIError{Code: gateway.ErrCodeUnauthorized, Message: err.Error(), Err: err}
	case errors.Is(err, ErrForbidden),
		errors.Is(err, knowledgeisolation.ErrCrossCaseAccess),
		errors.Is(err, knowledgeisolation.ErrCaseNotAuthorized):
		return &gateway.APIError{Code: gateway.ErrCodeForbidden, Message: err.Error(), Err: err}
	case errors.Is(err, ErrEmptyCaseID),
		errors.Is(err, ErrEmptyNodeID),
		errors.Is(err, ErrInvalidPagination),
		errors.Is(err, ErrEmptyQuery):
		return &gateway.APIError{Code: gateway.ErrCodeBadRequest, Message: err.Error(), Err: err}
	default:
		var apiErr *gateway.APIError
		if errors.As(err, &apiErr) {
			return apiErr
		}
		return &gateway.APIError{Code: gateway.ErrCodeInternal, Message: "an internal error occurred", Err: err}
	}
}

// RequirePermissionMiddleware adapts identity.RequirePermission's
// func(http.Handler) http.Handler middleware onto gateway.Middleware,
// which is defined identically but as a named type in packages/gateway.
// Pass its result as Routes' requirePermission argument to get an early,
// router-level 401/403 short-circuit ahead of every KnowledgeAPI method's
// own independent authorize check (access.go) — e.g.:
//
//	h.Routes(router, knowledgeapi.RequirePermissionMiddleware(identity.PermViewCase))
func RequirePermissionMiddleware(perms ...identity.Permission) gateway.Middleware {
	mw := identity.RequirePermission(perms...)
	return gateway.Middleware(mw)
}
