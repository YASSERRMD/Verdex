package localization

// SeedCatalog returns this phase's starter translation set (task 2):
// real English, Arabic, Urdu, and Tamil translations for a
// representative slice of the platform's UI surface --
//
//   - case_status.* mirrors apps/web's CASE_STATE_LABELS
//     (apps/web/src/lib/caseLifecycle.ts) key-for-key, so a caller
//     already holding a CaseState string can look up
//     "case_status."+state directly.
//   - action.* mirrors apps/web's CASE_WORKSPACE_ACTION_LABELS the same
//     way.
//   - common.* covers frequently reused UI actions (save, cancel,
//     confirm, etc.) not tied to any one workflow.
//   - disclaimer.non_binding is this package's translated rendering of
//     the same substantive warning packages/guardrail's
//     outputDisclaimer and apps/web's Disclaimer.tsx both already carry
//     in English. The English entry below is copied verbatim from
//     apps/web/src/components/Disclaimer.tsx's two paragraphs (not
//     guardrail's own longer, footer-style outputDisclaimer constant,
//     which is unexported and specifically shaped for appending to
//     rendered analysis text) so the two English surfaces stay
//     word-for-word identical; the ar/ur/ta entries are translations of
//     that same English text, reviewed to preserve its substance
//     exactly (non-binding, requires judge review and sign-off) --
//     this package does not alter, shorten, or soften the warning in
//     translation.
//
// Every locale below has a complete translation for every key: this is
// what lets FallbackLocale (English) work as a genuine safety net for
// keys added to ar/ur/ta later, not a crutch papering over gaps in this
// seed set itself.
func SeedCatalog() []CatalogEntry {
	entries := make([]CatalogEntry, 0, 4*len(seedKeys))
	for _, locale := range []Locale{LocaleEnglish, LocaleArabic, LocaleUrdu, LocaleTamil} {
		for key, byLocale := range seedKeys {
			entries = append(entries, CatalogEntry{Locale: locale, Key: key, Value: byLocale[locale]})
		}
	}
	return entries
}

// seedTranslations is a key -> (locale -> value) map, the natural
// authoring shape for a small hand-seeded catalogue (each key's four
// translations sit together, making it easy to review one concept
// across all locales at once). SeedCatalog transposes it into the
// flat []CatalogEntry Catalog.Merge/Set expect.
type seedTranslations = map[string]map[Locale]string

var seedKeys seedTranslations = mergeSeedGroups(
	caseStatusSeed,
	actionSeed,
	commonSeed,
	disclaimerSeed,
	dateSeed,
)

// mergeSeedGroups combines any number of seedTranslations maps into
// one, so each concern (case status, actions, common UI, disclaimer)
// can be authored and reviewed in its own variable below.
func mergeSeedGroups(groups ...seedTranslations) seedTranslations {
	out := make(seedTranslations)
	for _, g := range groups {
		for k, v := range g {
			out[k] = v
		}
	}
	return out
}

// caseStatusSeed mirrors apps/web's CASE_STATE_LABELS
// (apps/web/src/lib/caseLifecycle.ts) key-for-key: "draft", "active",
// "under_review", "closed", "archived".
var caseStatusSeed = seedTranslations{
	"case_status.draft": {
		LocaleEnglish: "Draft",
		LocaleArabic:  "مسودة",
		LocaleUrdu:    "مسودہ",
		LocaleTamil:   "வரைவு",
	},
	"case_status.active": {
		LocaleEnglish: "Active",
		LocaleArabic:  "نشط",
		LocaleUrdu:    "فعال",
		LocaleTamil:   "செயலில்",
	},
	"case_status.under_review": {
		LocaleEnglish: "Under Review",
		LocaleArabic:  "قيد المراجعة",
		LocaleUrdu:    "زیر جائزہ",
		LocaleTamil:   "மறுஆய்வில்",
	},
	"case_status.closed": {
		LocaleEnglish: "Closed",
		LocaleArabic:  "مغلق",
		LocaleUrdu:    "بند",
		LocaleTamil:   "மூடப்பட்டது",
	},
	"case_status.archived": {
		LocaleEnglish: "Archived",
		LocaleArabic:  "مؤرشف",
		LocaleUrdu:    "محفوظ شدہ",
		LocaleTamil:   "காப்பகப்படுத்தப்பட்டது",
	},
}

// actionSeed mirrors apps/web's CASE_WORKSPACE_ACTION_LABELS
// key-for-key.
var actionSeed = seedTranslations{
	"action.ingest_evidence": {
		LocaleEnglish: "Ingest Evidence",
		LocaleArabic:  "إدخال الأدلة",
		LocaleUrdu:    "شواہد شامل کریں",
		LocaleTamil:   "ஆதாரத்தைச் சேர்க்க",
	},
	"action.edit_category": {
		LocaleEnglish: "Edit Category",
		LocaleArabic:  "تعديل الفئة",
		LocaleUrdu:    "قسم میں ترمیم کریں",
		LocaleTamil:   "வகையைத் திருத்து",
	},
	"action.edit_timeline": {
		LocaleEnglish: "Edit Timeline",
		LocaleArabic:  "تعديل الجدول الزمني",
		LocaleUrdu:    "ٹائم لائن میں ترمیم کریں",
		LocaleTamil:   "காலவரிசையைத் திருத்து",
	},
	"action.generate_reasoning": {
		LocaleEnglish: "Generate Reasoning",
		LocaleArabic:  "إنشاء التحليل",
		LocaleUrdu:    "استدلال تیار کریں",
		LocaleTamil:   "பகுத்தறிவை உருவாக்கு",
	},
	"action.review_opinion": {
		LocaleEnglish: "Review Opinion",
		LocaleArabic:  "مراجعة الرأي",
		LocaleUrdu:    "رائے کا جائزہ لیں",
		LocaleTamil:   "கருத்தை மறுஆய்வு செய்",
	},
	"action.edit_metadata": {
		LocaleEnglish: "Edit Metadata",
		LocaleArabic:  "تعديل البيانات الوصفية",
		LocaleUrdu:    "میٹا ڈیٹا میں ترمیم کریں",
		LocaleTamil:   "மேலோட்டத் தரவைத் திருத்து",
	},
}

// commonSeed covers frequently reused UI actions not tied to any one
// case-workspace workflow.
var commonSeed = seedTranslations{
	"common.save": {
		LocaleEnglish: "Save",
		LocaleArabic:  "حفظ",
		LocaleUrdu:    "محفوظ کریں",
		LocaleTamil:   "சேமி",
	},
	"common.cancel": {
		LocaleEnglish: "Cancel",
		LocaleArabic:  "إلغاء",
		LocaleUrdu:    "منسوخ کریں",
		LocaleTamil:   "ரத்துசெய்",
	},
	"common.confirm": {
		LocaleEnglish: "Confirm",
		LocaleArabic:  "تأكيد",
		LocaleUrdu:    "تصدیق کریں",
		LocaleTamil:   "உறுதிசெய்",
	},
	"common.back": {
		LocaleEnglish: "Back",
		LocaleArabic:  "رجوع",
		LocaleUrdu:    "واپس",
		LocaleTamil:   "பின்செல்",
	},
	"common.continue": {
		LocaleEnglish: "Continue",
		LocaleArabic:  "متابعة",
		LocaleUrdu:    "جاری رکھیں",
		LocaleTamil:   "தொடர்க",
	},
	"common.sign_out": {
		LocaleEnglish: "Sign out",
		LocaleArabic:  "تسجيل الخروج",
		LocaleUrdu:    "سائن آؤٹ",
		LocaleTamil:   "வெளியேறு",
	},
	"common.language": {
		LocaleEnglish: "Language",
		LocaleArabic:  "اللغة",
		LocaleUrdu:    "زبان",
		LocaleTamil:   "மொழி",
	},
	"common.status": {
		LocaleEnglish: "Status",
		LocaleArabic:  "الحالة",
		LocaleUrdu:    "حیثیت",
		LocaleTamil:   "நிலை",
	},
	"common.no_actions_available": {
		LocaleEnglish: "No actions available in this state.",
		LocaleArabic:  "لا توجد إجراءات متاحة في هذه الحالة.",
		LocaleUrdu:    "اس حالت میں کوئی کارروائی دستیاب نہیں ہے۔",
		LocaleTamil:   "இந்த நிலையில் எந்த செயல்களும் இல்லை.",
	},
	"report_section.issue": {
		LocaleEnglish: "Issue",
		LocaleArabic:  "المسألة",
		LocaleUrdu:    "مسئلہ",
		LocaleTamil:   "பிரச்சினை",
	},
}

// disclaimerSeed is the translated non-binding disclaimer (see this
// file's doc comment for the exact English-source-of-truth
// coordination with apps/web/src/components/Disclaimer.tsx and
// packages/guardrail).
var disclaimerSeed = seedTranslations{
	"disclaimer.non_binding_title": {
		LocaleEnglish: "Non-Binding Draft Analysis",
		LocaleArabic:  "تحليل مسودة غير ملزم",
		LocaleUrdu:    "غیر پابند مسودہ تجزیہ",
		LocaleTamil:   "பிணைப்பில்லாத வரைவு பகுப்பாய்வு",
	},
	"disclaimer.non_binding_body": {
		LocaleEnglish: "This system produces non-binding draft analyses only. All outputs require review and sign-off by a qualified judge before any legal use or publication.",
		LocaleArabic:  "ينتج هذا النظام تحليلات مسودة غير ملزمة فقط. تتطلب جميع المخرجات مراجعة وتوقيعًا من قاضٍ مؤهل قبل أي استخدام قانوني أو نشر.",
		LocaleUrdu:    "یہ نظام صرف غیر پابند مسودہ تجزیے تیار کرتا ہے۔ کسی بھی قانونی استعمال یا اشاعت سے پہلے تمام نتائج کا ایک اہل جج کی جانب سے جائزہ اور دستخط ضروری ہے۔",
		LocaleTamil:   "இந்த அமைப்பு பிணைப்பில்லாத வரைவு பகுப்பாய்வுகளை மட்டுமே உருவாக்குகிறது. எந்தவொரு சட்டப் பயன்பாடு அல்லது வெளியீட்டிற்கும் முன், தகுதிவாய்ந்த நீதிபதியின் மறுஆய்வு மற்றும் ஒப்புதல் அனைத்து வெளியீடுகளுக்கும் தேவை.",
	},
}

// dateSeed provides translated month names ("date.month.1".."date.month.12",
// January-December) that format.go's FormatDate composes with locale-aware
// numeral formatting to render a full localized date, so a month name is
// never left in English inside an otherwise-localized Arabic/Urdu/Tamil
// date string.
var dateSeed = seedTranslations{
	"date.month.1": {
		LocaleEnglish: "January", LocaleArabic: "يناير", LocaleUrdu: "جنوری", LocaleTamil: "ஜனவரி",
	},
	"date.month.2": {
		LocaleEnglish: "February", LocaleArabic: "فبراير", LocaleUrdu: "فروری", LocaleTamil: "பிப்ரவரி",
	},
	"date.month.3": {
		LocaleEnglish: "March", LocaleArabic: "مارس", LocaleUrdu: "مارچ", LocaleTamil: "மார்ச்",
	},
	"date.month.4": {
		LocaleEnglish: "April", LocaleArabic: "أبريل", LocaleUrdu: "اپریل", LocaleTamil: "ஏப்ரல்",
	},
	"date.month.5": {
		LocaleEnglish: "May", LocaleArabic: "مايو", LocaleUrdu: "مئی", LocaleTamil: "மே",
	},
	"date.month.6": {
		LocaleEnglish: "June", LocaleArabic: "يونيو", LocaleUrdu: "جون", LocaleTamil: "ஜூன்",
	},
	"date.month.7": {
		LocaleEnglish: "July", LocaleArabic: "يوليو", LocaleUrdu: "جولائی", LocaleTamil: "ஜூலை",
	},
	"date.month.8": {
		LocaleEnglish: "August", LocaleArabic: "أغسطس", LocaleUrdu: "اگست", LocaleTamil: "ஆகஸ்ட்",
	},
	"date.month.9": {
		LocaleEnglish: "September", LocaleArabic: "سبتمبر", LocaleUrdu: "ستمبر", LocaleTamil: "செப்டம்பர்",
	},
	"date.month.10": {
		LocaleEnglish: "October", LocaleArabic: "أكتوبر", LocaleUrdu: "اکتوبر", LocaleTamil: "அக்டோபர்",
	},
	"date.month.11": {
		LocaleEnglish: "November", LocaleArabic: "نوفمبر", LocaleUrdu: "نومبر", LocaleTamil: "நவம்பர்",
	},
	"date.month.12": {
		LocaleEnglish: "December", LocaleArabic: "ديسمبر", LocaleUrdu: "دسمبر", LocaleTamil: "டிசம்பர்",
	},
}
