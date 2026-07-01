# Routing Policy

The `RoutingPolicy` struct in `packages/router` controls how the Verdex router
selects which LLM provider to call for a given request.

## Structure

```go
type RoutingPolicy struct {
    TaskRoutes        map[TaskType][]string
    FallbackChain     []string
    TenantOverrides   map[string]map[TaskType][]string
    AirGappedOnly     bool
    MaxCostPerRequest *float64
}
```

### TaskRoutes

Maps a `TaskType` (`"chat"`, `"embed"`, `"reason"`, `"extract"`) to an
**ordered** list of provider IDs.  The router tries providers left to right,
skipping any whose circuit breaker is open.

```go
policy.TaskRoutes = map[router.TaskType][]string{
    router.TaskChat:    {"anthropic-claude", "openai-gpt4o"},
    router.TaskEmbed:   {"openai-ada002"},
    router.TaskReason:  {"anthropic-claude"},
    router.TaskExtract: {"anthropic-claude", "openai-gpt4o"},
}
```

### FallbackChain

An ordered list of providers tried when no task-specific route exists or
every task-specific provider has been exhausted.

```go
policy.FallbackChain = []string{"openai-gpt4o", "local:llama3"}
```

### TenantOverrides

Allows per-tenant customisation of task routes.  Overrides take priority
over `TaskRoutes`.

```go
policy.TenantOverrides = map[string]map[router.TaskType][]string{
    "tenant-eu": {
        router.TaskChat: {"azure-eu-openai"},
    },
}
```

### AirGappedOnly

When `true`, the router filters all candidate provider lists to only those
whose ID begins with `"local:"`.  If no local provider is available
`ErrAirGappedViolation` is returned.

```go
policy.AirGappedOnly = true
policy.TaskRoutes[router.TaskChat] = []string{"local:llama3"}
```

### MaxCostPerRequest

An optional USD budget cap per request.  Currently informational; enforcement
is left to the caller.

```go
budget := 0.05 // USD
policy.MaxCostPerRequest = &budget
```

## Provider Selection Order

For a given `(taskType, tenantID)` pair the resolution order is:

1. `TenantOverrides[tenantID][taskType]`
2. `TaskRoutes[taskType]`
3. `FallbackChain`

The list is then filtered by `AirGappedOnly` if set.

## Circuit Breakers

Each provider has a `CircuitBreaker` managed by `CircuitBreakerRegistry`.

| State     | Behaviour                                                 |
|-----------|-----------------------------------------------------------|
| Closed    | Requests forwarded normally.                              |
| Open      | Requests rejected immediately (provider skipped).         |
| Half-open | One probe allowed; success closes, failure reopens.       |

Default thresholds:

- **Failure threshold**: 5 consecutive failures → open.
- **Recovery timeout**: 30 seconds → half-open.

## Telemetry

Every provider attempt emits a `RouterEvent` to the configured
`TelemetrySink`.  Use `LoggingTelemetrySink` during development and replace
it with an OpenTelemetry-backed sink in production.

## Example

```go
policy := router.DefaultPolicy()
policy.TaskRoutes[router.TaskChat] = []string{"anthropic-claude", "openai-gpt4o"}
policy.FallbackChain = []string{"local:llama3"}

r, err := router.NewRouter(router.RouterConfig{
    Registry:  providerRegistry,
    Policy:    policy,
    Telemetry: &router.LoggingTelemetrySink{},
})
if err != nil {
    log.Fatal(err)
}

resp, err := r.Chat(ctx, tenantID, chatReq)
```
