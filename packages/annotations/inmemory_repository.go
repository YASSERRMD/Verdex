package annotations

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InMemoryRepository is an in-process Repository implementation for
// tests and other packages' test fixtures, mirroring
// packages/casesearch.InMemorySavedSearchRepository's convention
// exactly. It is safe for concurrent use.
type InMemoryRepository struct {
	mu       sync.RWMutex
	records  map[uuid.UUID]*Annotation
	mentions []Mention
	audit    map[uuid.UUID][]*AuditRecord
}

// NewInMemoryRepository builds an empty InMemoryRepository.
func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		records: make(map[uuid.UUID]*Annotation),
		audit:   make(map[uuid.UUID][]*AuditRecord),
	}
}

func cloneAnnotation(a *Annotation) *Annotation {
	if a == nil {
		return nil
	}
	cp := *a
	if a.ParentID != nil {
		id := *a.ParentID
		cp.ParentID = &id
	}
	if a.ResolvedBy != nil {
		id := *a.ResolvedBy
		cp.ResolvedBy = &id
	}
	if a.ResolvedAt != nil {
		t := *a.ResolvedAt
		cp.ResolvedAt = &t
	}
	return &cp
}

// Create implements Repository.
func (r *InMemoryRepository) Create(_ context.Context, tenantID uuid.UUID, a *Annotation) error {
	if a == nil {
		return wrapf("InMemoryRepository.Create", ErrNilAnnotation)
	}
	if a.TenantID == uuid.Nil {
		a.TenantID = tenantID
	}
	if err := requireMatchingTenant(tenantID, a.TenantID); err != nil {
		return err
	}
	if err := a.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if a.ParentID != nil {
		parent, ok := r.records[*a.ParentID]
		if !ok || parent.TenantID != tenantID || parent.CaseID != a.CaseID {
			return ErrParentNotFound
		}
		if parent.IsReply() {
			return ErrParentIsReply
		}
	}

	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	now := time.Now().UTC()
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	r.records[a.ID] = cloneAnnotation(a)
	r.deriveMentions(a)
	return nil
}

// deriveMentions parses a's current Body for "@<userID>" tokens and
// appends a Mention record for each, so MentionsFor can answer queries
// without re-parsing every annotation's Body on every call. Must be
// called with r.mu held.
func (r *InMemoryRepository) deriveMentions(a *Annotation) {
	for _, userID := range ExtractMentions(a.Body) {
		r.mentions = append(r.mentions, Mention{
			AnnotationID:    a.ID,
			CaseID:          a.CaseID,
			TenantID:        a.TenantID,
			AuthorID:        a.AuthorID,
			MentionedUserID: userID,
			CreatedAt:       a.UpdatedAt,
		})
	}
}

// Get implements Repository.
func (r *InMemoryRepository) Get(_ context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.records[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	return cloneAnnotation(a), nil
}

// ListByCase implements Repository.
func (r *InMemoryRepository) ListByCase(_ context.Context, tenantID, caseID uuid.UUID, filter AnchorFilter) ([]*Annotation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*Annotation, 0)
	for _, a := range r.records {
		if a.TenantID != tenantID || a.CaseID != caseID {
			continue
		}
		if filter.Type != "" {
			if a.AnchorType != filter.Type {
				continue
			}
			if filter.ID != "" && a.AnchorID != filter.ID {
				continue
			}
		}
		out = append(out, cloneAnnotation(a))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// Thread implements Repository.
func (r *InMemoryRepository) Thread(_ context.Context, tenantID, rootID uuid.UUID) ([]*Annotation, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	root, ok := r.records[rootID]
	if !ok || root.TenantID != tenantID {
		return nil, ErrNotFound
	}

	out := []*Annotation{cloneAnnotation(root)}
	var replies []*Annotation
	for _, a := range r.records {
		if a.TenantID != tenantID {
			continue
		}
		if a.ParentID != nil && *a.ParentID == rootID {
			replies = append(replies, cloneAnnotation(a))
		}
	}
	sort.Slice(replies, func(i, j int) bool {
		return replies[i].CreatedAt.Before(replies[j].CreatedAt)
	})
	return append(out, replies...), nil
}

// UpdateBody implements Repository.
func (r *InMemoryRepository) UpdateBody(_ context.Context, tenantID, id uuid.UUID, body string) (*Annotation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.records[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	updated := cloneAnnotation(a)
	updated.Body = body
	updated.UpdatedAt = time.Now().UTC()
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	r.records[id] = cloneAnnotation(updated)
	r.deriveMentions(updated)
	return cloneAnnotation(updated), nil
}

// Delete implements Repository. Deleting a thread root cascades to its
// replies, mirroring the Postgres FK ON DELETE CASCADE.
func (r *InMemoryRepository) Delete(_ context.Context, tenantID, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.records[id]
	if !ok || a.TenantID != tenantID {
		return ErrNotFound
	}
	delete(r.records, id)
	for otherID, other := range r.records {
		if other.ParentID != nil && *other.ParentID == id {
			delete(r.records, otherID)
		}
	}
	return nil
}

// Resolve implements Repository.
func (r *InMemoryRepository) Resolve(_ context.Context, tenantID, id, resolvedBy uuid.UUID) (*Annotation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.records[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if a.Resolved {
		return nil, ErrAlreadyResolved
	}
	now := time.Now().UTC()
	updated := cloneAnnotation(a)
	updated.Resolved = true
	updated.ResolvedBy = &resolvedBy
	updated.ResolvedAt = &now
	updated.UpdatedAt = now
	r.records[id] = cloneAnnotation(updated)
	return cloneAnnotation(updated), nil
}

// Reopen implements Repository.
func (r *InMemoryRepository) Reopen(_ context.Context, tenantID, id uuid.UUID) (*Annotation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.records[id]
	if !ok || a.TenantID != tenantID {
		return nil, ErrNotFound
	}
	if !a.Resolved {
		return nil, ErrNotResolved
	}
	updated := cloneAnnotation(a)
	updated.Resolved = false
	updated.ResolvedBy = nil
	updated.ResolvedAt = nil
	updated.UpdatedAt = time.Now().UTC()
	r.records[id] = cloneAnnotation(updated)
	return cloneAnnotation(updated), nil
}

// MentionsFor implements Repository.
func (r *InMemoryRepository) MentionsFor(_ context.Context, tenantID, userID uuid.UUID) ([]Mention, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Mention, 0)
	for _, m := range r.mentions {
		if m.TenantID == tenantID && m.MentionedUserID == userID {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// AppendAudit implements Repository.
func (r *InMemoryRepository) AppendAudit(_ context.Context, tenantID uuid.UUID, rec *AuditRecord) error {
	if rec == nil {
		return wrapf("InMemoryRepository.AppendAudit", ErrNilAnnotation)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *rec
	r.audit[rec.AnnotationID] = append(r.audit[rec.AnnotationID], &cp)
	_ = tenantID
	return nil
}

// ListAudit implements Repository.
func (r *InMemoryRepository) ListAudit(_ context.Context, tenantID, annotationID uuid.UUID) ([]*AuditRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]*AuditRecord, 0)
	for _, rec := range r.audit[annotationID] {
		if rec.TenantID != tenantID {
			continue
		}
		cp := *rec
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].OccurredAt.Before(out[j].OccurredAt)
	})
	return out, nil
}

var _ Repository = (*InMemoryRepository)(nil)
