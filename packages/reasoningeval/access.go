package reasoningeval

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// requiredPermission is the identity.Permission required to read
// case-scoped QualityScores, ExpertReviews, or Alerts through Store or
// Dashboard. A QualityScore/ExpertReview exposes the same case-scoped
// reasoning-quality signal (dimension scores, review comments tied to a
// specific Opinion) that packages/grounding and packages/reasoningtrace
// already gate on identity.PermViewCase, so this package reuses that
// permission rather than inventing a new one.
const requiredPermission = identity.PermViewCase

// RequireViewPermission checks that ctx carries an authenticated
// identity.User who holds identity.PermViewCase, mirroring
// packages/grounding.RequireCheckPermission and
// packages/reasoningtrace.RequireViewPermission exactly.
//
// Returns ErrUnauthenticated if no user is present on ctx, or
// ErrForbidden if the user lacks the permission.
func RequireViewPermission(ctx context.Context) error {
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return ErrUnauthenticated
	}
	if !user.HasPermission(requiredPermission) {
		return ErrForbidden
	}
	return nil
}
