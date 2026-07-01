package setup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

// tenantIDKey is the context key used to propagate the tenant UUID through the
// request context.  Middleware upstream is expected to set this value.
type tenantIDKey struct{}

// TenantIDFromContext retrieves the tenant UUID stored in ctx.
// Returns uuid.Nil and false when no tenant ID is present.
func TenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v, ok := ctx.Value(tenantIDKey{}).(uuid.UUID)
	return v, ok
}

// WithTenantID returns a copy of ctx with the given tenant UUID attached.
// This helper is intended for use in middleware and tests.
func WithTenantID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, tenantIDKey{}, id)
}

// Handler bundles all HTTP handlers for the setup wizard API.
type Handler struct {
	svc *SetupService
}

// NewHandler returns a new [Handler] using the given service.
func NewHandler(svc *SetupService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes registers all setup endpoints on mux.
//
//	GET  /setup/status        → GetStatus
//	POST /setup/jurisdiction  → PostJurisdiction
//	POST /setup/court         → PostCourt
//	POST /setup/languages     → PostLanguages
//	POST /setup/provider      → PostProvider
//	POST /setup/complete      → PostComplete
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /setup/status", h.GetStatus)
	mux.HandleFunc("POST /setup/jurisdiction", h.PostJurisdiction)
	mux.HandleFunc("POST /setup/court", h.PostCourt)
	mux.HandleFunc("POST /setup/languages", h.PostLanguages)
	mux.HandleFunc("POST /setup/provider", h.PostProvider)
	mux.HandleFunc("POST /setup/complete", h.PostComplete)
}

// --------------------------------------------------------------------------
// helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func tenantFromRequest(r *http.Request) (uuid.UUID, bool) {
	return TenantIDFromContext(r.Context())
}

func mapServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSetupNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrSetupLocked):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrSetupAlreadyComplete):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidTransition):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, ErrMissingJurisdiction):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrMissingLanguages):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// --------------------------------------------------------------------------
// GET /setup/status

// GetStatus returns the current wizard state for the calling tenant.
func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	wizard, err := h.svc.GetStatus(r.Context(), tenantID)
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}

// --------------------------------------------------------------------------
// POST /setup/jurisdiction

type jurisdictionRequest struct {
	JurisdictionID uuid.UUID `json:"jurisdiction_id"`
}

// PostJurisdiction starts the wizard (if needed) and applies the jurisdiction
// selection step.
func (h *Handler) PostJurisdiction(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	var req jurisdictionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.JurisdictionID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "jurisdiction_id is required")
		return
	}

	// Ensure the wizard is started first.
	if _, err := h.svc.StartSetup(r.Context(), tenantID); err != nil {
		mapServiceError(w, err)
		return
	}

	wizard, err := h.svc.ApplyStep(r.Context(), tenantID, func(wiz *SetupWizard) error {
		return StepSelectJurisdiction(wiz, req.JurisdictionID)
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}

// --------------------------------------------------------------------------
// POST /setup/court

type courtRequest struct {
	CourtLevel string `json:"court_level"`
}

// PostCourt applies the court-level selection step.
func (h *Handler) PostCourt(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	var req courtRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.CourtLevel == "" {
		writeError(w, http.StatusBadRequest, "court_level is required")
		return
	}

	wizard, err := h.svc.ApplyStep(r.Context(), tenantID, func(wiz *SetupWizard) error {
		return StepSelectCourt(wiz, req.CourtLevel)
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}

// --------------------------------------------------------------------------
// POST /setup/languages

type languagesRequest struct {
	Languages []string `json:"languages"`
}

// PostLanguages applies the language selection step.
func (h *Handler) PostLanguages(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	var req languagesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if len(req.Languages) == 0 {
		writeError(w, http.StatusBadRequest, "at least one language is required")
		return
	}

	wizard, err := h.svc.ApplyStep(r.Context(), tenantID, func(wiz *SetupWizard) error {
		return StepSelectLanguages(wiz, req.Languages)
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}

// --------------------------------------------------------------------------
// POST /setup/provider

type providerRequest struct {
	ProviderType string `json:"provider_type"`
	Endpoint     string `json:"endpoint"`
	ModelID      string `json:"model_id"`
}

// PostProvider applies the provider configuration step.
func (h *Handler) PostProvider(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	var req providerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ProviderType == "" {
		writeError(w, http.StatusBadRequest, "provider_type is required")
		return
	}

	wizard, err := h.svc.ApplyStep(r.Context(), tenantID, func(wiz *SetupWizard) error {
		return StepConfigureProvider(wiz, ProviderConfigStub{
			ProviderType: req.ProviderType,
			Endpoint:     req.Endpoint,
			ModelID:      req.ModelID,
		})
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}

// --------------------------------------------------------------------------
// POST /setup/complete

// PostComplete applies the completion step, finalising the wizard.
func (h *Handler) PostComplete(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := tenantFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing tenant context")
		return
	}

	wizard, err := h.svc.ApplyStep(r.Context(), tenantID, func(wiz *SetupWizard) error {
		return StepComplete(wiz)
	})
	if err != nil {
		mapServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wizard)
}
