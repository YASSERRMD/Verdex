package notifications

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Service is the entrypoint for notification delivery and inbox
// operations, composing a Repository (notification storage) with a
// PreferenceRepository (per-user, per-Kind opt-in/out), mirroring
// packages/caseversioning.Service's composition style.
//
// Notify is deliberately not actor-gated by identity permissions: it
// is called by trusted server-side event hooks (this package's own
// adapters in adapters.go, or future direct call sites in
// packages/ingestion / packages/caselifecycle) on behalf of the
// system, not directly by an end user acting through an HTTP handler.
// Inbox-reading operations (List, UnreadCount, MarkRead, MarkAllRead)
// are actor-gated via authorizeSelf: a caller may only ever operate on
// their own notifications.
type Service struct {
	repo  Repository
	prefs PreferenceRepository
	now   func() time.Time
}

// NewService builds a Service. repo and prefs must be non-nil.
func NewService(repo Repository, prefs PreferenceRepository) (*Service, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	if prefs == nil {
		return nil, ErrNilPreferenceRepository
	}
	return &Service{repo: repo, prefs: prefs, now: func() time.Time { return time.Now().UTC() }}, nil
}

// isEnabled reports whether recipientID wants to receive notifications
// of kind, consulting prefs. Per task 6, preferences are opt-out, not
// opt-in: absence of an explicit Preference row means enabled — only
// an explicit Preference{Enabled: false} row suppresses delivery.
func (s *Service) isEnabled(ctx context.Context, tenantID, recipientID uuid.UUID, kind Kind) (bool, error) {
	pref, err := s.prefs.Get(ctx, tenantID, recipientID, kind)
	if err != nil {
		if err == ErrNotFound {
			return true, nil
		}
		return false, err
	}
	return pref.Enabled, nil
}

// NotifyInput carries the fields needed to construct and persist a
// Notification via Notify.
type NotifyInput struct {
	TenantID        uuid.UUID
	RecipientID     uuid.UUID
	Kind            Kind
	Title           string
	Body            string
	CaseID          *uuid.UUID
	RelatedEntityID *uuid.UUID
}

// Notify persists a new Notification for input.RecipientID, unless
// that recipient has explicitly disabled input.Kind via Preference —
// in which case Notify returns (nil, nil): a suppressed notification
// is not an error, it is the preference working as intended (task 9,
// "suppressed Kind isn't stored/delivered"). This is the single write
// path every adapter in adapters.go funnels through, so every upstream
// event hook's delivery obeys the same preference and persistence
// rules uniformly.
func (s *Service) Notify(ctx context.Context, input NotifyInput) (*Notification, error) {
	enabled, err := s.isEnabled(ctx, input.TenantID, input.RecipientID, input.Kind)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, nil
	}

	n := &Notification{
		TenantID:        input.TenantID,
		RecipientID:     input.RecipientID,
		Kind:            input.Kind,
		Title:           input.Title,
		Body:            input.Body,
		CaseID:          input.CaseID,
		RelatedEntityID: input.RelatedEntityID,
		CreatedAt:       s.now(),
	}
	if err := s.repo.Create(ctx, input.TenantID, n); err != nil {
		return nil, err
	}

	if channels, err := s.channelsFor(ctx, input.TenantID, input.RecipientID, input.Kind); err == nil {
		deliverStubChannels(ctx, channels, n)
	}
	return n, nil
}

// channelsFor returns the additional (non-in-app) delivery channels
// configured for (recipientID, kind), defaulting to none when no
// explicit Preference row exists.
func (s *Service) channelsFor(ctx context.Context, tenantID, recipientID uuid.UUID, kind Kind) ([]Channel, error) {
	pref, err := s.prefs.Get(ctx, tenantID, recipientID, kind)
	if err != nil {
		if err == ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return pref.Channels, nil
}

// List returns notifications addressed to recipientID, optionally
// narrowed by filter, in newest-first order. The ctx actor must be
// recipientID (ErrForbidden otherwise).
func (s *Service) List(ctx context.Context, tenantID, recipientID uuid.UUID, filter Filter) ([]*Notification, error) {
	if _, err := authorizeSelf(ctx, recipientID); err != nil {
		return nil, err
	}
	return s.repo.ListForRecipient(ctx, tenantID, recipientID, filter)
}

// UnreadCount returns the number of unread notifications addressed to
// recipientID. The ctx actor must be recipientID (ErrForbidden
// otherwise).
func (s *Service) UnreadCount(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	if _, err := authorizeSelf(ctx, recipientID); err != nil {
		return 0, err
	}
	return s.repo.UnreadCount(ctx, tenantID, recipientID)
}

// MarkRead marks the notification identified by id as read on behalf
// of recipientID. The ctx actor must be recipientID (ErrForbidden
// otherwise).
func (s *Service) MarkRead(ctx context.Context, tenantID, recipientID, id uuid.UUID) error {
	if _, err := authorizeSelf(ctx, recipientID); err != nil {
		return err
	}
	return s.repo.MarkRead(ctx, tenantID, recipientID, id)
}

// MarkAllRead marks every unread notification addressed to
// recipientID as read, returning the count newly marked. The ctx
// actor must be recipientID (ErrForbidden otherwise).
func (s *Service) MarkAllRead(ctx context.Context, tenantID, recipientID uuid.UUID) (int, error) {
	if _, err := authorizeSelf(ctx, recipientID); err != nil {
		return 0, err
	}
	return s.repo.MarkAllRead(ctx, tenantID, recipientID)
}

// SetPreference upserts a Preference on behalf of userID. The ctx
// actor must be userID (ErrForbidden otherwise) — a user may only
// change their own notification preferences.
func (s *Service) SetPreference(ctx context.Context, tenantID, userID uuid.UUID, kind Kind, enabled bool, channels []Channel) (*Preference, error) {
	if _, err := authorizeSelf(ctx, userID); err != nil {
		return nil, err
	}
	pref := &Preference{
		TenantID: tenantID,
		UserID:   userID,
		Kind:     kind,
		Enabled:  enabled,
		Channels: channels,
	}
	if err := s.prefs.Upsert(ctx, tenantID, pref); err != nil {
		return nil, err
	}
	return pref, nil
}

// Preferences returns every explicit Preference row for userID. The
// ctx actor must be userID (ErrForbidden otherwise).
func (s *Service) Preferences(ctx context.Context, tenantID, userID uuid.UUID) ([]*Preference, error) {
	if _, err := authorizeSelf(ctx, userID); err != nil {
		return nil, err
	}
	return s.prefs.ListForUser(ctx, tenantID, userID)
}
