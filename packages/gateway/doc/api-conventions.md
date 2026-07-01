# Verdex API Conventions

This document describes the conventions all Verdex HTTP APIs must follow. It is authoritative; deviations require a recorded decision.

---

## 1. Versioning

All API endpoints are served under a versioned path prefix:

```
/v{major}/resource
```

Examples: `/v1/cases`, `/v2/tenants/abc/rulings`.

- **Increment the major version** when introducing breaking changes (removed fields, changed semantics, new required parameters).
- **Append new, optional fields** to existing responses without bumping the version. Consumers must ignore unknown fields.
- The `VersionMiddleware` rejects unknown versions with `404 NOT_FOUND`.

Supported versions are declared in `version.go`. Add a new constant (`VersionV3`, etc.) and register it in `supportedVersions` when a new version ships.

---

## 2. Response Envelope

Every successful response body is a JSON object with the following top-level fields:

| Field        | Type    | Always present | Description                                   |
|--------------|---------|----------------|-----------------------------------------------|
| `version`    | string  | yes            | API version that handled the request (`v1`)   |
| `status`     | string  | yes            | `"success"` for 2xx responses                 |
| `data`       | any     | yes            | The primary payload                           |
| `meta`       | object  | no             | Pagination metadata (see §4)                  |
| `request_id` | string  | no             | UUID echoed from `X-Request-ID`               |

```jsonc
// GET /v1/cases/123 → 200 OK
{
  "version": "v1",
  "status": "success",
  "data": {
    "id": "123",
    "title": "Smith v. Jones"
  },
  "request_id": "01234567-89ab-cdef-0123-456789abcdef"
}
```

---

## 3. Error Envelope

Every error response body is a JSON object with the following top-level fields:

| Field        | Type     | Always present | Description                                       |
|--------------|----------|----------------|---------------------------------------------------|
| `version`    | string   | yes            | API version                                       |
| `status`     | string   | yes            | `"error"`                                         |
| `code`       | string   | yes            | Machine-readable error code (see §3.1)            |
| `message`    | string   | yes            | Human-readable explanation                        |
| `details`    | []string | no             | Per-field validation messages                     |
| `request_id` | string   | no             | UUID for correlation                              |

```jsonc
// POST /v1/cases → 422 Unprocessable Entity
{
  "version": "v1",
  "status": "error",
  "code": "BAD_REQUEST",
  "message": "request validation failed",
  "details": [
    "title: is required",
    "tenant_id: is required"
  ],
  "request_id": "01234567-89ab-cdef-0123-456789abcdef"
}
```

### 3.1 Error Codes

| Code                | HTTP Status | Meaning                             |
|---------------------|-------------|-------------------------------------|
| `BAD_REQUEST`       | 400         | Malformed input or validation error |
| `UNAUTHORIZED`      | 401         | Missing or invalid credentials      |
| `FORBIDDEN`         | 403         | Authenticated but not authorised    |
| `NOT_FOUND`         | 404         | Resource does not exist             |
| `CONFLICT`          | 409         | State conflict (e.g. duplicate key) |
| `TOO_MANY_REQUESTS` | 429         | Rate limit exceeded                 |
| `INTERNAL_ERROR`    | 500         | Unexpected server-side failure      |

---

## 4. Pagination

List endpoints support cursor-free offset pagination via query parameters:

| Parameter  | Default | Max | Description              |
|------------|---------|-----|--------------------------|
| `page`     | 1       | —   | 1-indexed page number    |
| `per_page` | 20      | 100 | Items per page           |

Paginated responses include a `meta` object:

```jsonc
{
  "meta": {
    "page": 2,
    "per_page": 20,
    "total": 142,
    "total_pages": 8
  }
}
```

Use `PaginateSlice[T]` from the gateway package for in-memory slices, or compute the `PaginationMeta` manually when querying the database.

---

## 5. Request ID

Every request should carry an `X-Request-ID` header. The `RequestIDMiddleware` generates a UUID if none is provided. The same ID is:

- Stored in the request context (access via `RequestIDFromContext`).
- Echoed in the `X-Request-ID` response header.
- Included in the `request_id` field of all response envelopes.

Clients should log request IDs to correlate client-side and server-side traces.

---

## 6. Authentication

All endpoints (except `/health`) require a Bearer JWT in the `Authorization` header:

```
Authorization: Bearer <token>
```

Tokens are issued by the Verdex identity service. A missing or invalid token returns `401 UNAUTHORIZED`. An expired token returns `401 UNAUTHORIZED` with message `"token has expired"`.

---

## 7. Rate Limiting

The default policy is **60 requests per minute per IP address**. Clients that exceed this limit receive `429 TOO_MANY_REQUESTS`. The `Retry-After` header is not currently set; clients should implement exponential back-off.

Rate-limit keys can be overridden per route (e.g. keyed by tenant ID) by passing a custom `keyFn` to `RateLimitMiddleware`.

---

## 8. CORS

Allowed origins are configured at service startup. The gateway handles `OPTIONS` preflight requests automatically. Credentials (`Authorization` cookies) require explicit opt-in via `CORSOptions.AllowCredentials`.

Allowed request headers (default): `Content-Type`, `Authorization`, `X-Request-ID`.

---

## 9. Security Headers

All responses include the following security headers (set by `SecurityHeadersMiddleware`):

| Header                    | Value                               |
|---------------------------|-------------------------------------|
| `X-Content-Type-Options`  | `nosniff`                           |
| `X-Frame-Options`         | `DENY`                              |
| `X-XSS-Protection`        | `1; mode=block`                     |
| `Referrer-Policy`         | `strict-origin-when-cross-origin`   |
| `Content-Security-Policy` | `default-src 'none'`                |

---

## 10. Middleware Ordering

The recommended middleware stack (outermost to innermost):

1. `Recovery` — catch panics before anything else runs
2. `RequestIDMiddleware` — ensure every request has an ID early
3. `SecurityHeadersMiddleware` — always set, even for error responses
4. `CORSMiddleware` — must run before auth to handle preflight
5. `Timeout` — bound request processing time
6. `RateLimitMiddleware` — reject excessive clients early
7. `VersionMiddleware` — validate API version
8. Auth middleware (identity package)
9. Route handler
