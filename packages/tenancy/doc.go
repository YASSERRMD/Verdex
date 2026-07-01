// Package tenancy builds the multi-tenant isolation, resolution, and
// enforcement machinery on top of the Tenant and Deployment entities
// already defined in packages/persistence. It does not redefine those
// entities; it provides:
//
//   - a request-scoped TenantContext carrying the resolved tenant
//     through a context.Context (see context.go),
//   - net/http middleware that attaches a resolved tenant to the
//     request context (see middleware.go),
//   - a Row-Level-Security-backed scoping helper, WithTenantScope, that
//     guarantees every statement inside its transaction is restricted
//     to a single tenant's rows (see scope.go),
//   - a deployment provisioning history record and repository
//     (see provisioning.go),
//   - tenant-scoped repository wrappers that compose WithTenantScope
//     with packages/persistence's repositories (see
//     deployment_repository.go),
//   - a placeholder header-based tenant resolver, to be superseded by
//     Phase 006's identity/session-based resolution (see resolve.go),
//   - an idempotent sandbox tenant seed (see seed.go).
//
// See README.md for the full tenancy model write-up, in particular why
// SET LOCAL (not SET) is mandatory inside WithTenantScope.
package tenancy
