package provenance

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Handler bundles the HTTP handlers for the provenance API.
type Handler struct {
	svc *ProvenanceService
}

// NewHandler creates a Handler backed by svc.
func NewHandler(svc *ProvenanceService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the provenance endpoints on mux.
//
//	POST   /provenance                    — create a record
//	GET    /provenance/{id}               — fetch a record
//	GET    /provenance/{id}/verify        — verify signature & chain
//	GET    /provenance/case/{caseID}      — list records for a case
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /provenance", h.CreateRecord)
	mux.HandleFunc("GET /provenance/case/{caseID}", h.ListByCase)
	mux.HandleFunc("GET /provenance/{id}/verify", h.VerifyRecord)
	mux.HandleFunc("GET /provenance/{id}", h.GetRecord)
}

// createRecordRequest is the JSON body for POST /provenance.
type createRecordRequest struct {
	TenantID    string  `json:"tenant_id"`
	CaseID      *string `json:"case_id,omitempty"`
	UploaderID  string  `json:"uploader_id"`
	Filename    string  `json:"filename"`
	MIMEType    string  `json:"mime_type"`
	SizeBytes   int64   `json:"size_bytes"`
	ContentHash string  `json:"content_hash"`
}

// CreateRecord handles POST /provenance.
func (h *Handler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	var req createRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	tenantID, err := uuid.Parse(req.TenantID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tenant_id")
		return
	}
	uploaderID, err := uuid.Parse(req.UploaderID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid uploader_id")
		return
	}

	var caseID *uuid.UUID
	if req.CaseID != nil {
		cid, err := uuid.Parse(*req.CaseID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid case_id")
			return
		}
		caseID = &cid
	}

	rec, err := h.svc.CreateRecord(r.Context(), tenantID, caseID, uploaderID,
		req.Filename, req.MIMEType, req.SizeBytes, req.ContentHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rec)
}

// GetRecord handles GET /provenance/{id}.
func (h *Handler) GetRecord(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	rec, err := h.svc.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProvenanceNotFound) {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// verifyResponse is returned by GET /provenance/{id}/verify.
type verifyResponse struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

// VerifyRecord handles GET /provenance/{id}/verify.
func (h *Handler) VerifyRecord(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	valid, reason, err := h.svc.Verify(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrProvenanceNotFound) {
			writeError(w, http.StatusNotFound, "record not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, verifyResponse{Valid: valid, Reason: reason})
}

// listByCaseResponse is returned by GET /provenance/case/{caseID}.
type listByCaseResponse struct {
	Records   []ProvenanceRecord `json:"records"`
	Total     int                `json:"total"`
	Timestamp time.Time          `json:"timestamp"`
}

// ListByCase handles GET /provenance/case/{caseID}.
func (h *Handler) ListByCase(w http.ResponseWriter, r *http.Request) {
	caseID, err := uuid.Parse(r.PathValue("caseID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid caseID")
		return
	}

	records, err := h.svc.store.GetByCase(r.Context(), caseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := listByCaseResponse{
		Records:   records,
		Total:     len(records),
		Timestamp: time.Now().UTC(),
	}
	writeJSON(w, http.StatusOK, resp)
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// errorResponse is the standard error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
