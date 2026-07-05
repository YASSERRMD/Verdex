package intake

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/tenancy"
)


const (
	// maxBodyBytes is the maximum total multipart body size accepted by the
	// handler.  Individual file size limits are enforced by QuotaConfig.
	maxBodyBytes = 100 * 1024 * 1024 // 100 MB
)

// IntakeHandler exposes the intake service over HTTP.
//
// Routes (to be registered with an external multiplexer):
//
//	POST /intake/upload       - multipart/form-data upload
//	GET  /intake/{id}/status  - poll the status of an intake operation
type IntakeHandler struct {
	svc *IntakeService
}

// NewIntakeHandler wraps svc in an HTTP handler.
func NewIntakeHandler(svc *IntakeService) *IntakeHandler {
	return &IntakeHandler{svc: svc}
}

// RegisterRoutes attaches the handler's endpoints to mux using the standard
// library's http.ServeMux pattern syntax (Go 1.22+).
func (h *IntakeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /intake/upload", h.handleUpload)
	mux.HandleFunc("GET /intake/{id}/status", h.handleStatus)
}

// handleUpload processes multipart/form-data POST requests.
//
//   - Reads the tenant from the request context via tenancy.TenantFromContext.
//   - Expects a form field named "file" containing the binary payload.
//   - Optional form fields: "case_id", "mime_type".
func (h *IntakeHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	// Enforce body size limit before parsing the multipart form.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	// Resolve tenant from context.
	t, ok := tenancy.TenantFromContext(r.Context())
	if !ok {
		h.writeError(w, http.StatusUnauthorized, "tenant not found in request context")
		return
	}

	// Parse multipart form; keep at most 32 MB in memory per Go default.
	// #nosec G120 -- overall request size is already bounded above by
	// http.MaxBytesReader(maxBodyBytes); this only sets the in-memory vs.
	// spill-to-disk threshold within that already-enforced cap. //nolint:gosec
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to parse multipart form: %s", err))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("field 'file' missing or unreadable: %s", err))
		return
	}
	defer file.Close()

	// Parse optional case_id.
	var caseID *uuid.UUID
	if raw := strings.TrimSpace(r.FormValue("case_id")); raw != "" {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil {
			h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid case_id: %s", parseErr))
			return
		}
		caseID = &parsed
	}

	// Determine MIME type from form field, falling back to header content-type.
	mimeType := strings.TrimSpace(r.FormValue("mime_type"))
	if mimeType == "" {
		mimeType = header.Header.Get("Content-Type")
	}
	// Strip parameters (e.g. "; charset=utf-8").
	if idx := strings.IndexByte(mimeType, ';'); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	req := IntakeRequest{
		TenantID:   t.ID,
		CaseID:     caseID,
		UploaderID: t.ID, // In a real system this would be the authenticated user ID.
		Filename:   header.Filename,
		MIMEType:   mimeType,
		SizeBytes:  header.Size,
		TTL:        5 * time.Minute,
	}

	result, err := h.svc.Ingest(r.Context(), req, file)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not permitted") ||
			strings.Contains(err.Error(), "mismatch") {
			status = http.StatusUnsupportedMediaType
		} else if strings.Contains(err.Error(), "quota") {
			status = http.StatusTooManyRequests
		} else if strings.Contains(err.Error(), "size") {
			status = http.StatusRequestEntityTooLarge
		}
		h.writeError(w, status, err.Error())
		return
	}

	h.writeJSON(w, http.StatusAccepted, result)
}

// handleStatus returns the last-known status for an intake operation by ID.
// Since IntakeResult is not persisted, this implementation returns 404 for
// all IDs (a real deployment would query a status store).
func (h *IntakeHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("id")
	id, err := uuid.Parse(rawID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid intake ID: %s", err))
		return
	}

	// Ensure the request comes from an authenticated tenant.
	if _, ok := tenancy.TenantFromContext(r.Context()); !ok {
		h.writeError(w, http.StatusUnauthorized, "tenant not found in request context")
		return
	}

	// The intake service does not store results after discard; callers should
	// use the IntakeResult returned from /intake/upload.
	h.writeError(w, http.StatusNotFound, fmt.Sprintf("intake %s not found or already discarded", id))
}

func (h *IntakeHandler) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func (h *IntakeHandler) writeError(w http.ResponseWriter, code int, msg string) {
	h.writeJSON(w, code, map[string]string{"error": msg})
}
