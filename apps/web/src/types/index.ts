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
