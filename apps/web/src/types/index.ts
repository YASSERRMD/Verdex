// ─── User & Auth ────────────────────────────────────────────────────────────

export type Role = 'admin' | 'judge' | 'clerk' | 'viewer';

export interface User {
  id: string;
  name: string;
  email: string;
  roles: Role[];
  createdAt?: string;
  updatedAt?: string;
}

// ─── Jurisdiction ────────────────────────────────────────────────────────────

export type CourtLevel =
  | 'supreme'
  | 'appellate'
  | 'high'
  | 'district'
  | 'magistrate'
  | 'family'
  | 'commercial'
  | 'other';

export interface Jurisdiction {
  id: string;
  country: string;
  countryCode: string;
  courtLevel: CourtLevel;
  name: string;
  languages: string[];
}

// ─── Setup Wizard ────────────────────────────────────────────────────────────

export type SupportedLanguage = 'ar' | 'ur' | 'ta' | 'en';

export interface JurisdictionSetup {
  country: string;
  courtLevel: string;
}

export interface LanguageSetup {
  selected: SupportedLanguage[];
}

export interface ProviderSetup {
  type: string;
  endpoint: string;
  modelId: string;
}

export interface SetupState {
  jurisdiction: JurisdictionSetup;
  language: LanguageSetup;
  provider: ProviderSetup;
}

export interface SetupWizard {
  currentStep: number;
  totalSteps: number;
  completed: boolean;
  state: SetupState;
}

// ─── Cases ───────────────────────────────────────────────────────────────────

export type CaseStatus = 'open' | 'pending' | 'closed' | 'archived';

export interface Case {
  id: string;
  caseNumber: string;
  title: string;
  status: CaseStatus;
  jurisdictionId: string;
  assignedJudgeId?: string;
  createdAt: string;
  updatedAt: string;
}

// ─── API Responses ───────────────────────────────────────────────────────────

export interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface ApiError {
  message: string;
  code?: string;
  details?: Record<string, string[]>;
}

// ─── Ingestion ───────────────────────────────────────────────────────────────

/**
 * Case category used at intake. Mirrors the categories a clerk chooses
 * when opening a new matter — kept intentionally small and generic so it
 * maps cleanly onto jurisdiction-specific taxonomies later.
 */
export type CaseCategory =
  | 'civil'
  | 'criminal'
  | 'family'
  | 'commercial'
  | 'administrative'
  | 'other';

/** Form state for creating a new case at the start of ingestion. */
export interface CaseCreationInput {
  category: CaseCategory | '';
  firstPartyName: string;
  secondPartyName: string;
}

/** Per-file upload status shown in the queued-file list. */
export type UploadStatus = 'queued' | 'uploading' | 'uploaded' | 'failed';

/** A single file (document or audio) queued or in-flight for upload. */
export interface UploadedFile {
  id: string;
  name: string;
  size: number;
  mimeType: string;
  status: UploadStatus;
  progress: number;
  error?: string;
}

/**
 * Pipeline stage for an ingestion job. Mirrors packages/ingestion's Stage
 * constants (intake, extraction, normalize, segment, classify, complete,
 * failed) so the UI vocabulary stays consistent with the backend.
 */
export type IngestionStage =
  | 'intake'
  | 'extraction'
  | 'normalize'
  | 'segment'
  | 'classify'
  | 'complete'
  | 'failed';

/** Point-in-time status of an ingestion job, as polled from the API. */
export interface IngestionStatus {
  jobId: string;
  stage: IngestionStage;
  percentComplete: number;
  message?: string;
  error?: string;
}

/**
 * A reviewable extracted/transcribed segment with a reference back to its
 * position in the source artifact. This is draft material only — it has
 * not been reviewed or signed off, and must never be presented as a
 * finding or verdict.
 */
export interface SegmentReview {
  id: string;
  text: string;
  sourceSpan: {
    start: number;
    end: number;
  };
  sourceFileName?: string;
}

/**
 * Evidence classification for a single segment, mirroring
 * packages/evidence's Classification/EvidenceType/PartyRole concepts.
 * Editable in the UI via a ManualOverride-style correction.
 */
export type EvidenceType =
  | 'testimony'
  | 'documentary'
  | 'statute_citation'
  | 'argument'
  | 'other';

export type PartyRole = 'first_party' | 'second_party' | 'third_party' | 'unattributed';

export interface SegmentClassification {
  segmentId: string;
  type: EvidenceType;
  party: PartyRole;
  confidence: number;
  overridden: boolean;
}

/**
 * Timeline party, mirroring packages/timeline's Party concept.
 * UI-only representation used by the party/timeline editor.
 */
export interface TimelineParty {
  id: string;
  role: 'first_party' | 'second_party' | 'third_party';
  name: string;
  counsel?: string;
}

/**
 * Timeline event, mirroring packages/timeline's Event concept.
 * `occurredAt` is an ISO date string or undefined when the date could not
 * be determined from the source text.
 */
export interface TimelineEvent {
  id: string;
  description: string;
  occurredAt?: string;
  partyId?: string;
  confidence?: number;
}

// ─── Case Workspace ─────────────────────────────────────────────────────────

/**
 * Case lifecycle state, mirroring packages/caselifecycle's State constants
 * exactly (draft, active, under_review, closed, archived). Kept distinct
 * from the older, simpler `CaseStatus` above, which predates the
 * caselifecycle package and is not reused here.
 */
export type CaseState = 'draft' | 'active' | 'under_review' | 'closed' | 'archived';

/**
 * Case-scoped action, mirroring packages/caselifecycle's Action constants.
 * Used to drive which buttons the status/actions bar shows for the case's
 * current state.
 */
export type CaseWorkspaceAction =
  | 'ingest_evidence'
  | 'edit_category'
  | 'edit_timeline'
  | 'generate_reasoning'
  | 'review_opinion'
  | 'edit_metadata';

/**
 * One immutable entry in a case's transition audit log, mirroring
 * packages/caselifecycle's TransitionRecord.
 */
export interface CaseTransitionRecord {
  id: string;
  caseId: string;
  fromState: CaseState;
  toState: CaseState;
  actor: string;
  reason?: string;
  occurredAt: string;
}

/**
 * The canonical case record as the workspace expects an API to expose it,
 * mirroring packages/caselifecycle.Case field-for-field (camelCased):
 * id, tenantId, jurisdictionId, categoryId, title, reference, state,
 * metadata/metadataVersion, createdBy/createdAt/updatedAt, archivedAt.
 */
export interface CaseLifecycle {
  id: string;
  tenantId: string;
  jurisdictionId: string;
  jurisdictionName?: string;
  categoryId: string;
  categoryLabel?: string;
  subcategoryLabel?: string;
  title: string;
  reference?: string;
  state: CaseState;
  metadata: Record<string, string>;
  metadataVersion: number;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
  archivedAt?: string;
}

/** A party attached to a case, as shown in the parties/category panel. */
export interface CaseParty {
  id: string;
  role: 'first_party' | 'second_party' | 'third_party';
  name: string;
  counsel?: string;
}

/** An evidence segment shown in the workspace's evidence panel. */
export interface EvidenceSegment {
  id: string;
  text: string;
  type: EvidenceType;
  party: PartyRole;
  confidence: number;
  sourceFileName?: string;
  sourceSpan: {
    start: number;
    end: number;
  };
  /**
   * Whether this segment's classification or attribution is actively
   * contested by a party. Not yet modeled in packages/evidence, so tracked
   * client-side pending a real dispute-flag API (see docs/evidence-review.md).
   */
  disputed?: boolean;
}

// ─── Evidence Review (Phase 066) ───────────────────────────────────────────

/**
 * One entry in a segment's change-audit trail: which field changed, from
 * what to what, by whom, and when. Client-side/mocked pending a real audit
 * API — generalizes packages/evidence's ManualOverride audit intent
 * (override.go: ReviewedBy/ReviewedAt/Previous) to any reviewable field on
 * a segment (type, party, or disputed status), not just type/party
 * classification.
 */
export interface EvidenceAuditEntry {
  id: string;
  segmentId: string;
  field: 'type' | 'party' | 'disputed';
  previousValue: string;
  newValue: string;
  actor: string;
  occurredAt: string;
}

// ─── IRAC Reasoning Tree ─────────────────────────────────────────────────────

/**
 * Position a node occupies in the Issue-Rule-Fact-Application-Conclusion
 * reasoning tree, mirroring packages/irac's NodeType constants exactly
 * (packages/irac/node.go): "issue", "rule", "fact", "application",
 * "conclusion".
 */
export type TreeNodeType = 'issue' | 'rule' | 'fact' | 'application' | 'conclusion';

/**
 * How two nodes in the reasoning tree relate to one another, mirroring
 * packages/irac's EdgeType constants exactly (packages/irac/edge.go):
 *   - "governs": Rule -> Issue
 *   - "applies_to": Application -> Fact | Application -> Rule
 *   - "supports": Fact -> Application
 *   - "concludes_from": Conclusion -> Application
 */
export type TreeEdgeType = 'governs' | 'applies_to' | 'supports' | 'concludes_from';

/**
 * Locates a node's claim within its original ingested source text, mirroring
 * packages/irac's SourceSpan (packages/irac/span.go). `page` is set for
 * OCR-derived text, `startMs`/`endMs` for STT-derived text; both are omitted
 * when the origin does not apply.
 */
export interface TreeSourceSpan {
  start: number;
  end: number;
  page?: number;
  startMs?: number;
  endMs?: number;
}

/**
 * Records how a tree node came to exist, mirroring packages/irac's
 * Provenance (packages/irac/provenance.go).
 */
export interface TreeNodeProvenance {
  generatedBy: string;
  generatedAt: string;
  upstreamNodeIds?: string[];
}

/**
 * One node in a case's IRAC reasoning tree, mirroring packages/irac's Node
 * plus its typed wrappers (IssueNode, RuleNode, FactNode, ApplicationNode,
 * ConclusionNode). Type-specific fields (`jurisdictionCode`, `legalFamily`
 * for rules; `label` for conclusions) are optional here since a single flat
 * shape covers every node type, matching how knowledgeapi's NodeDTO is
 * transported over the wire (packages/knowledgeapi/dto.go) extended with the
 * span/provenance/jurisdiction detail this panel needs to render.
 */
export interface TreeNode {
  id: string;
  type: TreeNodeType;
  caseId: string;
  text: string;
  confidence: number;
  createdAt: string;
  spans?: TreeSourceSpan[];
  provenance?: TreeNodeProvenance;
  /** RuleNode only: the jurisdiction this rule derives its authority from. */
  jurisdictionCode?: string;
  /** RuleNode only: the legal tradition this rule derives from. */
  legalFamily?: string;
  /**
   * ConclusionNode only: the mandatory non-binding guardrail label. Always
   * "draft_analysis" per packages/irac's guardrail (packages/irac/
   * guardrail.go) — reasoning output is never presented as a verdict.
   */
  label?: string;
}

/** A directed relationship between two nodes, mirroring packages/irac's Edge. */
export interface TreeEdge {
  fromId: string;
  toId: string;
  type: TreeEdgeType;
}

/**
 * A case's full IRAC reasoning tree as served by the (not-yet-implemented)
 * `/api/v1/cases/:caseId/tree` endpoint, matching packages/knowledgeapi's
 * GetTreeResponse shape (packages/knowledgeapi/dto.go) camelCased.
 */
export interface ReasoningTree {
  caseId: string;
  nodes: TreeNode[];
  edges: TreeEdge[];
}

// ─── Draft Reasoned Opinion (Phase 067) ─────────────────────────────────────

/**
 * Which side of a case an argument or conclusion favors, mirroring the
 * plain-string PartyID convention shared by firstpartyagent.PartyID /
 * secondpartyagent.PartyID / evidenceweighing.CitingArgument.PartyID: an
 * opaque caller-defined label, not a hard dependency on any one party
 * package's own type.
 */
export type OpinionPartyRole = 'first_party' | 'second_party';

/**
 * A resolved, verified citation attached to one of an OpinionArgument's
 * supporting rules, mirroring firstpartyagent.CitationRef /
 * secondpartyagent.CitationRef field-for-field.
 */
export interface OpinionCitation {
  nodeId: string;
  citation: string;
  verificationStatus: string;
  verified: boolean;
  confidenceScore: number;
}

/**
 * One party's line of reasoning for a single issue, mirroring
 * firstpartyagent.Argument / secondpartyagent.Argument (structurally
 * identical, independent types upstream) camelCased: a claim, the fact/rule
 * node IDs from the case's tree that support it, resolved citations,
 * anticipated counterarguments, and a [0,1] strength score.
 */
export interface OpinionArgument {
  id: string;
  issueNodeId: string;
  partyId: OpinionPartyRole;
  claim: string;
  supportingFactIds: string[];
  supportingRuleIds: string[];
  citations?: OpinionCitation[];
  counterarguments?: string[];
  strength: number;
  grounded: boolean;
}

/**
 * Per-fact evidentiary weight backing an issue's arguments, mirroring
 * evidenceweighing.FactWeight camelCased: a [0,1] weight, whether the fact
 * is contradicted by the opposing party, how many arguments corroborate it,
 * and a human-readable rationale for the score.
 */
export interface OpinionEvidenceWeight {
  factNodeId: string;
  weight: number;
  kind: string;
  contradicted: boolean;
  corroborationCount: number;
  rationale: string;
}

/**
 * Which upstream pipeline stage an uncertainty finding was derived from,
 * mirroring packages/uncertainty's Source constants exactly:
 * "issue_framing" | "evidence" | "law_application" | "conclusion".
 */
export type OpinionUncertaintySource =
  | 'issue_framing'
  | 'evidence'
  | 'law_application'
  | 'conclusion';

/**
 * A single ranked reason to doubt part of an issue's draft analysis,
 * mirroring packages/uncertainty.Uncertainty camelCased: which upstream
 * Source raised it, how severe the signal is on its own, its impact rank
 * relative to every other uncertainty for the case, and a human-readable
 * caveat suitable for direct display to a reviewing judge.
 */
export interface OpinionUncertainty {
  issueNodeId: string;
  source: OpinionUncertaintySource;
  severity: number;
  impactRank: number;
  impactScore: number;
  caveat: string;
  detail?: string;
}

/**
 * The tentative, non-binding draft resolution for a single issue, mirroring
 * synthesisagent.TentativeConclusion camelCased: which party (if any) this
 * draft currently favors, the conclusion's own confidence, its single
 * weakest supporting element, and every fact/rule node ID it traces back
 * to. Text here is guaranteed, by the time it reaches this panel, to have
 * passed packages/guardrail's CheckText verdict-language gate — but the
 * panel's own tests re-assert that independently rather than trusting the
 * upstream guarantee blindly (see ReasoningOpinionPanel.test.tsx).
 */
export interface OpinionConclusion {
  issueNodeId: string;
  text: string;
  favoredParty?: OpinionPartyRole;
  confidence: number;
  weakestLink?: string;
  supportingFactIds: string[];
  supportingRuleIds: string[];
  grounded: boolean;
}

/**
 * One issue's full draft-analysis section: the issue framing itself, both
 * parties' argument chains, the evidence weights those arguments rely on,
 * the tentative conclusion, and any uncertainty findings scoped to this
 * issue. This is a UI-side aggregate — no single backend package returns
 * this exact shape yet — that joins synthesisagent.TentativeConclusion,
 * firstpartyagent/secondpartyagent.Argument, evidenceweighing.FactWeight,
 * and uncertainty.Uncertainty by their shared IssueNodeID, the same way a
 * real /api/v1/cases/:caseId/opinion endpoint is expected to when built
 * (see packages/reasoningtrace, which assembles the equivalent trace
 * server-side).
 */
export interface IssueOpinion {
  issueNodeId: string;
  issueText: string;
  firstPartyArguments: OpinionArgument[];
  secondPartyArguments: OpinionArgument[];
  evidenceWeights: OpinionEvidenceWeight[];
  conclusion: OpinionConclusion;
  uncertainties: OpinionUncertainty[];
}

/**
 * A judge's comment/annotation on one issue's draft analysis. Client-side/
 * mocked pending a real annotation API — mirrors EvidenceAuditEntry's
 * "actor + occurredAt" attribution convention, since no annotation backend
 * exists yet (same rationale as EvidenceAuditEntry in Phase 066).
 */
export interface OpinionComment {
  id: string;
  issueNodeId: string;
  text: string;
  author: string;
  occurredAt: string;
}

/**
 * A case's full draft reasoned opinion as the workspace expects an API to
 * expose it: one IssueOpinion per issue addressed, mirroring
 * synthesisagent.Opinion's case-level envelope (caseId/generatedAt) plus
 * this panel's per-issue aggregation.
 */
export interface CaseOpinion {
  caseId: string;
  issues: IssueOpinion[];
  generatedAt: string;
}

// ─── Case search ────────────────────────────────────────────────────────────

/**
 * Search mode, mirroring packages/casesearch.Mode's string constants
 * exactly ('' for auto-detection, 'keyword', 'semantic', 'issue_rule').
 */
export type SearchMode = '' | 'keyword' | 'semantic' | 'issue_rule';

/**
 * Structured filters narrowing a case search, mirroring
 * packages/casesearch.Filter's field set (minus PartyName's backend-only
 * PartyLookup wiring caveat, which the UI surfaces as a plain text field
 * regardless).
 */
export interface SearchFilters {
  categoryCode?: string;
  jurisdictionId?: string;
  partyName?: string;
  state?: string;
  dateFrom?: string;
  dateTo?: string;
}

/** One content match within a case, mirroring packages/casesearch.Hit. */
export interface SearchHit {
  nodeId: string;
  nodeType: string;
  text: string;
  score: number;
  explanation: string;
}

/** One ranked case-search result, mirroring packages/casesearch.Result. */
export interface SearchResultItem {
  caseId: string;
  title: string;
  reference: string;
  categoryId: string;
  jurisdictionId: string;
  state: string;
  createdAt: string;
  mode: SearchMode;
  score: number;
  snippet: string;
  hits: SearchHit[];
}

/** The full search response, mirroring packages/casesearch.Results. */
export interface SearchResults {
  items: SearchResultItem[];
  totalMatches: number;
  page: { number: number; size: number };
  mode: SearchMode;
  skippedCases: number;
}

/** A persisted search, mirroring packages/casesearch.SavedSearch. */
export interface SavedSearchEntry {
  id: string;
  name: string;
  query: {
    text: string;
    mode: SearchMode;
    issueOrRuleId?: string;
    filter?: SearchFilters;
  };
  createdAt: string;
}

// ─── Annotations & collaboration ────────────────────────────────────────────

/**
 * What an annotation is attached to, mirroring
 * packages/annotations.AnchorType's string constants exactly.
 */
export type AnnotationAnchorType = 'case' | 'tree_node' | 'evidence_segment';

/**
 * A single note, highlight, or discussion comment, mirroring
 * packages/annotations.Annotation. `anchorId` is empty for
 * anchorType 'case', an irac tree node ID for 'tree_node', and an
 * evidence segment ID (the same ID space EvidenceReviewPanel uses)
 * for 'evidence_segment'.
 */
export interface AnnotationEntry {
  id: string;
  caseId: string;
  authorId: string;
  authorName?: string;
  body: string;
  anchorType: AnnotationAnchorType;
  anchorId: string;
  parentId?: string;
  resolved: boolean;
  resolvedBy?: string;
  resolvedAt?: string;
  createdAt: string;
  updatedAt: string;
}

// ─── Case versioning & history ──────────────────────────────────────────────

/**
 * Which case artifact a version-history entry captures, mirroring
 * packages/caseversioning.ArtifactKind's string constants exactly.
 */
export type SnapshotArtifactKind = 'case-metadata' | 'tree' | 'evidence' | 'opinion';

/**
 * One immutable, point-in-time record of a case artifact's state,
 * mirroring packages/caseversioning.Snapshot. `payload` is present only
 * for 'case-metadata' and 'opinion' snapshots (a compact copy); 'tree'
 * and 'evidence' snapshots carry only `artifactRevisionRef`, a pointer
 * into packages/irac's/packages/treeassembly's own revision store or
 * packages/annotations's audit trail, never a duplicated copy.
 */
export interface SnapshotEntry {
  id: string;
  caseId: string;
  artifactKind: SnapshotArtifactKind;
  artifactRevisionRef?: string;
  payload?: Record<string, unknown>;
  createdBy: string;
  createdByName?: string;
  reason?: string;
  label?: string;
  restoredFromId?: string;
  createdAt: string;
}

/** One field-level change reported by a case-metadata Diff. */
export interface SnapshotFieldChange {
  field: string;
  before: string;
  after: string;
}

/**
 * The structured comparison between two snapshots, mirroring
 * packages/caseversioning.Diff. `fieldChanges` is populated for
 * 'case-metadata' snapshot pairs; 'tree'/'evidence'/'opinion' pairs only
 * ever populate the revisionRef* fields (a reference-level diff).
 */
export interface SnapshotDiff {
  caseId: string;
  artifactKind: SnapshotArtifactKind;
  snapshotAId: string;
  snapshotBId: string;
  fieldChanges?: SnapshotFieldChange[];
  revisionRefChanged: boolean;
  revisionRefBefore?: string;
  revisionRefAfter?: string;
  identical: boolean;
}

// ─── Notifications ──────────────────────────────────────────────────────────

/**
 * What kind of event a notification represents, mirroring
 * packages/notifications.Kind's string constants exactly.
 */
export type NotificationKind =
  | 'ingestion_complete'
  | 'pending_signoff'
  | 'mention'
  | 'quality_alert'
  | 'budget_alert'
  | 'task_assignment';

/**
 * A single persisted, user-facing notice, mirroring
 * packages/notifications.Notification. `readAt` is undefined while
 * unread.
 */
export interface NotificationEntry {
  id: string;
  tenantId: string;
  recipientId: string;
  kind: NotificationKind;
  title: string;
  body?: string;
  caseId?: string;
  relatedEntityId?: string;
  createdAt: string;
  readAt?: string;
}

// ─── Analytics ───────────────────────────────────────────────────────────────

/** Mirrors packages/analytics.StateCount. */
export interface AnalyticsStateCount {
  state: CaseState;
  count: number;
}

/** Mirrors packages/analytics.CategoryCount. */
export interface AnalyticsCategoryCount {
  categoryId: string;
  count: number;
}

/** Mirrors packages/analytics.JurisdictionBreakdown. */
export interface AnalyticsJurisdictionBreakdown {
  jurisdictionId: string;
  count: number;
  byState: AnalyticsStateCount[];
}

/** Mirrors packages/analytics.DailyCaseCount. */
export interface AnalyticsDailyCaseCount {
  date: string; // YYYY-MM-DD
  count: number;
}

/**
 * Mirrors packages/analytics.Metrics: the aggregated caseload view
 * returned by GET /api/v1/analytics/caseload.
 */
export interface AnalyticsMetrics {
  tenantId: string;
  generatedAt: string;
  totalCases: number;
  byState: AnalyticsStateCount[];
  byCategory: AnalyticsCategoryCount[];
  byJurisdiction: AnalyticsJurisdictionBreakdown[];
  createdTrend: AnalyticsDailyCaseCount[];
}

/**
 * Mirrors packages/analytics.QualityTrendPoint, one jurisdiction's
 * reasoning-quality summary within a QualityTrend.
 */
export interface QualityTrendPoint {
  jurisdictionCode: string;
  legalFamily?: string;
  count: number;
  avgOverall: number;
  avgPerDimension: Record<string, number>;
}

/**
 * Mirrors packages/analytics.QualityTrend, returned by
 * GET /api/v1/analytics/quality-trend.
 */
export interface QualityTrend {
  points: QualityTrendPoint[];
}

/** Mirrors packages/accounting.ProviderSummary. */
export interface UsageProviderSummary {
  providerId: string;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  requestCount: number;
}

/** Mirrors packages/accounting.TaskSummary. */
export interface UsageTaskSummary {
  taskType: string;
  totalInputTokens: number;
  totalOutputTokens: number;
  totalTokens: number;
  estimatedCostUsd: number;
  requestCount: number;
}

/** Mirrors packages/accounting.DailyTrend. */
export interface UsageDailyTrend {
  date: string; // YYYY-MM-DD
  totalTokens: number;
  estimatedCostUsd: number;
  requestCount: number;
}

/**
 * Mirrors packages/accounting.TenantDashboard, returned by
 * GET /api/v1/analytics/usage. Only rendered client-side for
 * admin/judge roles — see UsageCostPanel — matching the server-side
 * identity.PermAuditRead gate on packages/analytics.UsageComposer.
 */
export interface UsageDashboard {
  tenantId: string;
  generatedAt: string;
  byProvider: UsageProviderSummary[];
  byTaskType: UsageTaskSummary[];
  last7DaysTrend: UsageDailyTrend[];
  totalTokens: number;
  estimatedCostUsd: number;
  requestCount: number;
}
