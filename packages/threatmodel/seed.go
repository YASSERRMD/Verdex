package threatmodel

import "github.com/google/uuid"

// SeedThreatModels returns the platform's starter threat catalogue
// (task 1): concrete, named threats and mitigations against three
// components -- the API gateway (packages/gateway, Phase 009), the
// ingestion pipeline (packages/intake / packages/ingestion, Phase
// 019/029), and the reasoning orchestration path
// (packages/reasoningorchestration, Phase 059). Every Mitigation's
// ReferenceTag names a real, existing platform control by string tag;
// this package does not import any of the tagged packages. Callers
// wanting a full ThreatModel per component can also call
// GatewayThreatModel / IngestionThreatModel /
// ReasoningOrchestrationThreatModel directly.
//
// Every returned ThreatModel and its nested Threat/Mitigation values
// leave ID zero where a caller intends to persist/track them --
// AllocateIDs (below) fills every zero ID in, mirroring
// packages/compliance.SeedControls leaving Control.ID zero for
// RegisterControl to fill in.
func SeedThreatModels() []ThreatModel {
	models := []ThreatModel{
		GatewayThreatModel(),
		IngestionThreatModel(),
		ReasoningOrchestrationThreatModel(),
	}
	for i := range models {
		AllocateIDs(&models[i])
	}
	return models
}

// AllocateIDs fills every zero uuid.UUID (ThreatModel.ID, each
// Threat.ID, each Mitigation.ID) in tm with a freshly generated UUID,
// leaving any already-set ID untouched -- so re-running AllocateIDs on
// an already-allocated ThreatModel is a safe no-op, mirroring
// packages/compliance's RegisterControl "fill in ID if zero" idiom
// applied here at catalogue-construction time instead of at a
// repository-write boundary (this package has no repository for
// ThreatModel itself -- see doc.go's persistence discussion).
func AllocateIDs(tm *ThreatModel) {
	if tm.ID == uuid.Nil {
		tm.ID = uuid.New()
	}
	for i := range tm.Threats {
		if tm.Threats[i].ID == uuid.Nil {
			tm.Threats[i].ID = uuid.New()
		}
		for j := range tm.Threats[i].Mitigations {
			if tm.Threats[i].Mitigations[j].ID == uuid.Nil {
				tm.Threats[i].Mitigations[j].ID = uuid.New()
			}
		}
	}
}

// GatewayThreatModel is the starter ThreatModel for the API gateway
// (packages/gateway, Phase 009): the platform's single HTTP entry
// point, and therefore the component most directly exposed to an
// unauthenticated attacker.
func GatewayThreatModel() ThreatModel {
	return ThreatModel{
		Component: Component{
			Name:        "gateway",
			PackageTag:  "packages/gateway",
			Description: "The platform's HTTP API gateway: request routing, authentication, rate limiting, and request/response envelope handling for every external caller.",
		},
		Threats: []Threat{
			{
				Title:       "Unauthenticated actor spoofs a tenant user's identity",
				Description: "An external caller submits a forged or replayed authentication token to impersonate a legitimate tenant user and act with that user's permissions.",
				Category:    StrideSpoofing,
				Severity:    SeverityCritical,
				Mitigations: []Mitigation{
					{
						Title:        "Permission-gated request authorization",
						Description:  "Every gateway-routed operation resolves the authenticated identity.User from request context and checks a required identity.Permission before proceeding.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/identity.RequirePermission",
					},
				},
			},
			{
				Title:       "Oversized or malformed request body exhausts gateway resources",
				Description: "An attacker submits an extremely large or deeply nested JSON body to exhaust memory or CPU before request validation rejects it.",
				Category:    StrideDenialOfService,
				Severity:    SeverityHigh,
				Mitigations: []Mitigation{
					{
						Title:        "Request body decode and structural validation",
						Description:  "ValidateRequest decodes with DisallowUnknownFields and rejects structurally invalid bodies before handler logic runs.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/gateway.ValidateRequest",
					},
					{
						Title:        "Hardening-library size/charset/structure checks",
						Description:  "A lower-level, protocol-agnostic Validator/Sanitize pass bounds input size and rejects control characters before gateway-level decode, composing with (not replacing) ValidateRequest.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/threatmodel.ValidateSize",
					},
				},
			},
			{
				Title:       "Request flood degrades gateway availability for all tenants",
				Description: "A single caller or botnet issues a very high rate of requests, starving legitimate tenants of gateway capacity.",
				Category:    StrideDenialOfService,
				Severity:    SeverityHigh,
				Mitigations: []Mitigation{
					{
						Title:        "Sliding-window rate limiting",
						Description:  "RateLimitMiddleware enforces a per-caller sliding-window request rate limit and returns 429 once exceeded.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/gateway.RateLimitMiddleware",
					},
					{
						Title:        "Request timeout enforcement",
						Description:  "Gateway requests are bounded by a context-based timeout, returning 503 rather than holding a connection indefinitely.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/gateway.Timeout",
					},
				},
			},
			{
				Title:       "Cross-tenant data leakage via a manipulated tenant identifier",
				Description: "An authenticated user of tenant A supplies tenant B's identifier in a request path/body and is served tenant B's data because a handler trusts the caller-supplied tenant ID rather than the authenticated scope.",
				Category:    StrideInformationDisclosure,
				Severity:    SeverityCritical,
				Mitigations: []Mitigation{
					{
						Title:        "Tenant-match enforcement at every operation boundary",
						Description:  "Every tenant-scoped Engine method rejects a request whose target tenant ID does not match the authenticated actor's own TenantID.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/identity.User.TenantID",
					},
					{
						Title:        "Row-Level Security as a database-layer backstop",
						Description:  "Postgres RLS policies enforce tenant isolation independent of any application-layer bug, so a defect in a single handler cannot leak cross-tenant rows.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/tenancy.WithTenantScope",
					},
				},
			},
		},
	}
}

// IngestionThreatModel is the starter ThreatModel for the ingestion
// pipeline (packages/intake, packages/ingestion, Phase 019/029): the
// component that accepts untrusted binary artifacts (audio, images,
// documents) from tenant users and extracts text from them.
func IngestionThreatModel() ThreatModel {
	return ThreatModel{
		Component: Component{
			Name:        "ingestion",
			PackageTag:  "packages/ingestion",
			Description: "The document/media ingestion pipeline: intake, STT/OCR extraction, normalization, segmentation, and classification of tenant-submitted binary artifacts.",
		},
		Threats: []Threat{
			{
				Title:       "Retained source binary survives past its required discard point",
				Description: "A bug or malicious modification to the STT/OCR extraction path retains the original audio/image bytes in memory or storage after transcription, defeating the platform's transcribe-and-discard guarantee and creating an unnecessary copy of potentially sensitive content.",
				Category:    StrideInformationDisclosure,
				Severity:    SeverityCritical,
				Mitigations: []Mitigation{
					{
						Title:        "Source-byte zeroing at the STT/OCR boundary",
						Description:  "stt.Discard/ocr.Discard zero the source AudioInput/ImageInput bytes immediately after extraction, recording a provenance hash beforehand.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/stt.Discard",
					},
					{
						Title:        "Independent post-extraction discard verification",
						Description:  "VerifyAudioDiscard/VerifyImageDiscard independently assert, per job, that the source hash was recorded and the input bytes were actually zeroed, failing the pipeline stage otherwise rather than trusting the extraction step's own claim.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/ingestion.VerifyAudioDiscard",
					},
				},
			},
			{
				Title:       "Ingested content is tampered with between upload and extraction, with no way to detect it",
				Description: "An attacker with access to intermediate storage modifies an uploaded artifact after intake but before extraction, and no mechanism exists to prove the extracted content corresponds to what was actually uploaded.",
				Category:    StrideTampering,
				Severity:    SeverityHigh,
				Mitigations: []Mitigation{
					{
						Title:        "Content-hash provenance recorded at upload",
						Description:  "A ProvenanceRecord captures a content hash and chain-of-custody signature at upload time, before any processing, so downstream tampering is detectable by hash mismatch.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/provenance.ProvenanceRecord",
					},
				},
			},
			{
				Title:       "Malicious file masquerading as an allowed MIME type",
				Description: "An attacker uploads a file with a spoofed extension or Content-Type header to bypass intake's file-type allow-list and reach a parser it was never intended to reach.",
				Category:    StrideSpoofing,
				Severity:    SeverityMedium,
				Mitigations: []Mitigation{
					{
						Title:        "MIME-type sniffing and allow-list enforcement",
						Description:  "Intake determines a file's actual MIME type from content rather than trusting the caller-supplied Content-Type header, and rejects anything outside the configured allow-list.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/intake.DetectMIME",
					},
				},
			},
			{
				Title:       "Extracted text carries an embedded prompt-injection payload that reaches an LLM prompt unexamined",
				Description: "A document or transcript contains text engineered to override system instructions once it is later placed into an LLM prompt (e.g. as case evidence), and the ingestion pipeline forwards it without ever screening for injection patterns.",
				Category:    StrideElevationOfPrivilege,
				Severity:    SeverityHigh,
				Mitigations: []Mitigation{
					{
						Title:        "Pre-prompt injection-pattern screening",
						Description:  "DetectInjectionAttempt scans extracted text for role-override phrases, instruction-injection markers, and delimiter-breaking sequences before the text is placed into any prompt template variable.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/threatmodel.DetectInjectionAttempt",
					},
					{
						Title:        "Template-delimiter injection guard at render time",
						Description:  "SanitizeValue independently rejects any template variable value containing Go text/template delimiters, a second layer beneath the screening above.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/prompts.SanitizeValue",
					},
				},
			},
		},
	}
}

// ReasoningOrchestrationThreatModel is the starter ThreatModel for the
// reasoning orchestration path (packages/reasoningorchestration, Phase
// 059): the pipeline that coordinates issue framing, argument
// construction, evidence weighing, law application, synthesis, and the
// final guardrail check for a single case.
func ReasoningOrchestrationThreatModel() ThreatModel {
	return ThreatModel{
		Component: Component{
			Name:        "reasoning-orchestration",
			PackageTag:  "packages/reasoningorchestration",
			Description: "Coordinates the full agent sequence -- issue framing, first/second-party argument construction, evidence weighing, law application, synthesis, uncertainty surfacing, and the non-binding guardrail check -- end to end for one case.",
		},
		Threats: []Threat{
			{
				Title:       "Synthesized output omits or loses the mandatory non-binding guardrail label",
				Description: "A defect in the reasoning pipeline, an intermediate serialization round-trip, or a downstream consumer strips the draft_analysis label before the output reaches a human reader, who then mistakes AI-generated draft analysis for an authoritative verdict.",
				Category:    StrideTampering,
				Severity:    SeverityCritical,
				Mitigations: []Mitigation{
					{
						Title:        "Guardrail label required at pipeline completion",
						Description:  "The pipeline's final guardrail-check stage rejects any case run whose synthesized conclusion does not carry the mandatory non-binding label before the run is allowed to reach StageComplete.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/guardrail.RequireLabel",
					},
					{
						Title:        "Defensive re-verification at output boundaries",
						Description:  "VerifyGuardrailIntact re-checks the label immediately before any output is surfaced or persisted, independent of whether the pipeline's own internal check already passed, so a post-pipeline mutation cannot silently drop the label.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/threatmodel.VerifyGuardrailIntact",
					},
				},
			},
			{
				Title:       "Synthesized conclusion contains verdict or directive language despite the label being present",
				Description: "The pipeline attaches the correct non-binding label, but the underlying text itself uses verdict/directive phrasing (e.g. \"the defendant is guilty\") that a reader could still mistake for an authoritative ruling regardless of the disclaimer.",
				Category:    StrideElevationOfPrivilege,
				Severity:    SeverityCritical,
				Mitigations: []Mitigation{
					{
						Title:        "Verdict-language content check",
						Description:  "CheckText rejects any reasoning-output text containing verdict or directive language, independent of and in addition to the label check.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/guardrail.CheckText",
					},
				},
			},
			{
				Title:       "A mid-pipeline failure is silently treated as a completed, trustworthy run",
				Description: "A stage in the orchestration pipeline fails (e.g. an agent call errors or times out) but the failure is not recorded, so a partial or degraded case run is later surfaced as if it had completed the full agent sequence.",
				Category:    StrideRepudiation,
				Severity:    SeverityHigh,
				Mitigations: []Mitigation{
					{
						Title:        "Explicit run-state and termination-reason tracking",
						Description:  "RunState records CurrentStage, CompletedStages, and a TerminationReason for every run, so a failed or partial run is structurally distinguishable from a completed one, not inferred from absence of an error.",
						Status:       MitigationVerified,
						ReferenceTag: "packages/reasoningorchestration.RunState",
					},
					{
						Title:        "Durable audit trail of pipeline outcomes",
						Description:  "Every run's outcome is recorded to the platform's durable, hash-chained audit trail, giving a tamper-evident record of what actually completed versus failed.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/auditlog.Store",
					},
				},
			},
			{
				Title:       "A single tenant's reasoning run exhausts shared LLM/router capacity, degrading other tenants",
				Description: "A tenant submits an unusually large or complex case that consumes disproportionate reasoning-pipeline budget, starving other tenants of timely reasoning results.",
				Category:    StrideDenialOfService,
				Severity:    SeverityMedium,
				Mitigations: []Mitigation{
					{
						Title:        "Per-run pipeline budget enforcement",
						Description:  "RunConfig.Budget bounds the resources (e.g. agent calls, tokens) a single case run may consume, failing the run rather than allowing unbounded consumption.",
						Status:       MitigationImplemented,
						ReferenceTag: "packages/reasoningorchestration.PipelineBudget",
					},
				},
			},
		},
	}
}
