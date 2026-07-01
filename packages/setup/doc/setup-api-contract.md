# Setup Wizard API Contract

## Overview

The Setup Wizard API drives a tenant through its first-run deployment
provisioning.  Every request must carry the tenant's UUID in the request
context (injected by upstream authentication middleware).

Base path: `/setup`

All responses are JSON.  Error responses have the shape:

```json
{ "error": "<human-readable message>" }
```

---

## State Machine

A tenant's setup wizard moves through the following states in order:

```
pending
  └─ in_progress
       └─ jurisdiction_selected
            └─ court_selected
                 └─ language_selected
                      └─ provider_configured
                           └─ completed
                                └─ locked  (terminal)
```

Once a wizard reaches `locked` all modification attempts return **409
Conflict**.

---

## Endpoints

### GET /setup/status

Returns the current wizard state for the calling tenant.

**Response 200**

```json
{
  "tenant_id": "uuid",
  "state": "in_progress",
  "jurisdiction_id": "uuid | null",
  "court_level": "string | null",
  "languages": ["en", "ar"],
  "provider_config": {
    "provider_type": "openai",
    "endpoint": "https://api.openai.com/v1",
    "model_id": "gpt-4o",
    "configured_at": "2024-01-01T00:00:00Z"
  },
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "completed_at": "2024-01-01T00:00:00Z | null",
  "locked_at": "2024-01-01T00:00:00Z | null"
}
```

**Response 401** — missing tenant context  
**Response 404** — no wizard found for the tenant

---

### POST /setup/jurisdiction

Starts the wizard (if needed) and records the selected jurisdiction.

**Request body**

```json
{ "jurisdiction_id": "uuid" }
```

**Response 200** — wizard state after the step  
**Response 400** — missing or zero `jurisdiction_id`  
**Response 401** — missing tenant context  
**Response 409** — wizard is locked or already complete  
**Response 422** — invalid state transition (e.g. step applied out of order)

---

### POST /setup/court

Records the selected court level.

**Request body**

```json
{ "court_level": "supreme" }
```

Valid values are not enforced at this layer; the jurisdiction package owns
court-level validation.  Common examples: `supreme`, `appellate`, `trial`,
`magistrate`.

**Response 200** — wizard state after the step  
**Response 400** — empty `court_level`  
**Response 401** — missing tenant context  
**Response 409** — wizard is locked  
**Response 422** — invalid state transition

---

### POST /setup/languages

Records the set of reasoning-language codes (BCP-47).

**Request body**

```json
{ "languages": ["en", "ar", "fr"] }
```

At least one language code is required.

**Response 200** — wizard state after the step  
**Response 400** — empty `languages` array  
**Response 401** — missing tenant context  
**Response 409** — wizard is locked  
**Response 422** — invalid state transition

---

### POST /setup/provider

Records the AI inference provider stub configuration.

**Request body**

```json
{
  "provider_type": "openai",
  "endpoint": "https://api.openai.com/v1",
  "model_id": "gpt-4o"
}
```

`provider_type` is required.  `endpoint` and `model_id` are optional strings.
This step stores a placeholder only; real provider integration is performed
elsewhere.

**Response 200** — wizard state after the step  
**Response 400** — missing `provider_type`  
**Response 401** — missing tenant context  
**Response 409** — wizard is locked  
**Response 422** — invalid state transition

---

### POST /setup/complete

Marks the wizard as completed.  Validates that a jurisdiction and at least one
language have been recorded.

**Request body** — empty body or `{}`

**Response 200** — wizard state (`state: "completed"`, `completed_at` set)  
**Response 400** — missing jurisdiction or languages  
**Response 401** — missing tenant context  
**Response 409** — wizard already locked  
**Response 422** — invalid state transition

---

## Idempotency

- `POST /setup/jurisdiction` calls `StartSetup` internally; calling it when the
  wizard is already past `pending` is safe and does not reset progress.
- `GET /setup/status` is always idempotent.
- Each `POST` step will return **422 Unprocessable Entity** if applied when the
  wizard is not in the immediately preceding state, protecting against
  out-of-order or double application.

## Locking

After `POST /setup/complete` succeeds, callers may lock the wizard via the
service layer (`StepLock`) to prevent any future modification.  There is no
REST endpoint for locking by design — locking is an administrative operation
performed programmatically after confirming the configuration is correct.

## Error Reference

| Error constant         | HTTP status | Meaning |
|------------------------|-------------|---------|
| `ErrSetupNotFound`     | 404         | No wizard record for the tenant |
| `ErrSetupLocked`       | 409         | Wizard is locked — no further changes |
| `ErrSetupAlreadyComplete` | 409      | Wizard already completed |
| `ErrInvalidTransition` | 422         | Step applied out of sequence |
| `ErrMissingJurisdiction` | 400       | Completion attempted without a jurisdiction |
| `ErrMissingLanguages`  | 400         | No languages selected |
