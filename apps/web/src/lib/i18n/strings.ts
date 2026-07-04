import type { SupportedLanguage } from '@/types';

/**
 * Externalized UI strings catalog (Phase 090, task 1), mirroring
 * packages/localization's Go Catalog shape (locale -> key -> string)
 * key-for-key: case_status.*, action.*, and common.* below are the
 * exact same keys packages/localization/seed.go seeds, and the
 * English values are copied verbatim from apps/web's own pre-existing
 * CASE_STATE_LABELS/CASE_WORKSPACE_ACTION_LABELS
 * (src/lib/caseLifecycle.ts) so this catalog is additive, not a
 * competing source of truth -- caseLifecycle.ts's exported label maps
 * are re-derived from STRINGS below (see the bottom of this file)
 * rather than duplicated by hand in two places.
 *
 * This is deliberately a minimal in-house catalog, not a new i18n
 * framework dependency: apps/web/package.json has neither next-intl
 * nor react-i18next installed, and a four-locale, few-dozen-key
 * surface does not warrant adding one.
 */
export type TranslationKey =
  | 'case_status.draft'
  | 'case_status.active'
  | 'case_status.under_review'
  | 'case_status.closed'
  | 'case_status.archived'
  | 'action.ingest_evidence'
  | 'action.edit_category'
  | 'action.edit_timeline'
  | 'action.generate_reasoning'
  | 'action.review_opinion'
  | 'action.edit_metadata'
  | 'common.save'
  | 'common.cancel'
  | 'common.confirm'
  | 'common.back'
  | 'common.continue'
  | 'common.sign_out'
  | 'common.language'
  | 'common.status'
  | 'common.no_actions_available';

type StringTable = Record<TranslationKey, string>;

/** FALLBACK_LOCALE mirrors packages/localization's FallbackLocale. */
export const FALLBACK_LOCALE: SupportedLanguage = 'en';

const en: StringTable = {
  'case_status.draft': 'Draft',
  'case_status.active': 'Active',
  'case_status.under_review': 'Under Review',
  'case_status.closed': 'Closed',
  'case_status.archived': 'Archived',
  'action.ingest_evidence': 'Ingest Evidence',
  'action.edit_category': 'Edit Category',
  'action.edit_timeline': 'Edit Timeline',
  'action.generate_reasoning': 'Generate Reasoning',
  'action.review_opinion': 'Review Opinion',
  'action.edit_metadata': 'Edit Metadata',
  'common.save': 'Save',
  'common.cancel': 'Cancel',
  'common.confirm': 'Confirm',
  'common.back': 'Back',
  'common.continue': 'Continue',
  'common.sign_out': 'Sign out',
  'common.language': 'Language',
  'common.status': 'Status',
  'common.no_actions_available': 'No actions available in this state.',
};

const ar: StringTable = {
  'case_status.draft': 'مسودة',
  'case_status.active': 'نشط',
  'case_status.under_review': 'قيد المراجعة',
  'case_status.closed': 'مغلق',
  'case_status.archived': 'مؤرشف',
  'action.ingest_evidence': 'إدخال الأدلة',
  'action.edit_category': 'تعديل الفئة',
  'action.edit_timeline': 'تعديل الجدول الزمني',
  'action.generate_reasoning': 'إنشاء التحليل',
  'action.review_opinion': 'مراجعة الرأي',
  'action.edit_metadata': 'تعديل البيانات الوصفية',
  'common.save': 'حفظ',
  'common.cancel': 'إلغاء',
  'common.confirm': 'تأكيد',
  'common.back': 'رجوع',
  'common.continue': 'متابعة',
  'common.sign_out': 'تسجيل الخروج',
  'common.language': 'اللغة',
  'common.status': 'الحالة',
  'common.no_actions_available': 'لا توجد إجراءات متاحة في هذه الحالة.',
};

const ur: StringTable = {
  'case_status.draft': 'مسودہ',
  'case_status.active': 'فعال',
  'case_status.under_review': 'زیر جائزہ',
  'case_status.closed': 'بند',
  'case_status.archived': 'محفوظ شدہ',
  'action.ingest_evidence': 'شواہد شامل کریں',
  'action.edit_category': 'قسم میں ترمیم کریں',
  'action.edit_timeline': 'ٹائم لائن میں ترمیم کریں',
  'action.generate_reasoning': 'استدلال تیار کریں',
  'action.review_opinion': 'رائے کا جائزہ لیں',
  'action.edit_metadata': 'میٹا ڈیٹا میں ترمیم کریں',
  'common.save': 'محفوظ کریں',
  'common.cancel': 'منسوخ کریں',
  'common.confirm': 'تصدیق کریں',
  'common.back': 'واپس',
  'common.continue': 'جاری رکھیں',
  'common.sign_out': 'سائن آؤٹ',
  'common.language': 'زبان',
  'common.status': 'حیثیت',
  'common.no_actions_available': 'اس حالت میں کوئی کارروائی دستیاب نہیں ہے۔',
};

const ta: StringTable = {
  'case_status.draft': 'வரைவு',
  'case_status.active': 'செயலில்',
  'case_status.under_review': 'மறுஆய்வில்',
  'case_status.closed': 'மூடப்பட்டது',
  'case_status.archived': 'காப்பகப்படுத்தப்பட்டது',
  'action.ingest_evidence': 'ஆதாரத்தைச் சேர்க்க',
  'action.edit_category': 'வகையைத் திருத்து',
  'action.edit_timeline': 'காலவரிசையைத் திருத்து',
  'action.generate_reasoning': 'பகுத்தறிவை உருவாக்கு',
  'action.review_opinion': 'கருத்தை மறுஆய்வு செய்',
  'action.edit_metadata': 'மேலோட்டத் தரவைத் திருத்து',
  'common.save': 'சேமி',
  'common.cancel': 'ரத்துசெய்',
  'common.confirm': 'உறுதிசெய்',
  'common.back': 'பின்செல்',
  'common.continue': 'தொடர்க',
  'common.sign_out': 'வெளியேறு',
  'common.language': 'மொழி',
  'common.status': 'நிலை',
  'common.no_actions_available': 'இந்த நிலையில் எந்த செயல்களும் இல்லை.',
};

const TABLES: Record<SupportedLanguage, StringTable> = { en, ar, ur, ta };

/**
 * translate resolves key for locale, falling back to FALLBACK_LOCALE
 * (English) when the target locale's table is missing the key --
 * mirroring packages/localization.Translate's fallback contract
 * exactly (task 8). Every key above has a complete translation in
 * every locale, so fallback here is a safety net for keys added later
 * without an immediate ar/ur/ta translation, not a crutch for gaps in
 * this seed set itself.
 */
export function translate(locale: SupportedLanguage, key: TranslationKey): string {
  const table = TABLES[locale];
  const value = table?.[key];
  if (value !== undefined) return value;
  return TABLES[FALLBACK_LOCALE][key];
}

/**
 * Re-derives a Record<T, string> label map for every value of a
 * fixed-key union (e.g. CaseState, CaseWorkspaceAction) from this
 * catalog for locale, so a caller with an existing
 * Record<CaseState, string>-shaped label map (like
 * caseLifecycle.ts's CASE_STATE_LABELS) can become locale-aware with a
 * single call instead of hand-duplicating strings.
 */
export function translateAll<K extends string>(
  locale: SupportedLanguage,
  keysByValue: Record<K, TranslationKey>,
): Record<K, string> {
  const out = {} as Record<K, string>;
  for (const value of Object.keys(keysByValue) as K[]) {
    out[value] = translate(locale, keysByValue[value]);
  }
  return out;
}
