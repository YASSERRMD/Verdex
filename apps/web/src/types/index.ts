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
