package localization

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// FallbackLocale is the locale Translate resolves to when the
// requested Locale has no entry for a key (task 8). Every key seeded
// by SeedCatalog has a complete FallbackLocale translation, so
// fallback can never itself miss -- see Translate's doc comment for
// the exact resolution order and MustTranslate for the stricter
// variant that panics if even the fallback is missing (an authoring
// bug, not a runtime translation gap).
const FallbackLocale = LocaleEnglish

// Catalog is a locale -> key -> translated-string table (task 1's
// core externalization mechanism, and task 7's translation-management
// surface). Catalog is safe for concurrent use: Translate is called on
// every render in a real deployment, and RecordMissing (called
// internally by Translate) must not race with concurrent reads.
//
// A Catalog holds compiled-in seed data (see seed.go) plus whatever a
// caller registers via Set/Merge at startup -- it is not itself
// backed by a database. Per-tenant/per-user *preference* for which
// Locale to use is a separate, durable concern (see preference.go);
// the Catalog itself is process-wide, immutable-after-boot data.
type Catalog struct {
	mu sync.RWMutex

	// entries is keyed [locale][key] -> translated string.
	entries map[Locale]map[string]string

	// missing tracks every (locale, key) pair Translate resolved via
	// fallback because locale had no entry for key, so a translation-
	// management workflow can query MissingKeys(locale) later (task 7).
	// A set (map to struct{}) since only presence, not count, matters.
	missing map[Locale]map[string]struct{}
}

// NewCatalog builds an empty Catalog. Most callers should use
// NewSeededCatalog instead to start with this phase's real en/ar/ur/ta
// translations already loaded.
func NewCatalog() *Catalog {
	return &Catalog{
		entries: make(map[Locale]map[string]string),
		missing: make(map[Locale]map[string]struct{}),
	}
}

// NewSeededCatalog builds a Catalog pre-populated with SeedCatalog's
// entries (see seed.go).
func NewSeededCatalog() *Catalog {
	c := NewCatalog()
	for _, e := range SeedCatalog() {
		c.Set(e.Locale, e.Key, e.Value)
	}
	return c
}

// Set registers (or overwrites) the translation for key in locale.
func (c *Catalog) Set(locale Locale, key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries[locale] == nil {
		c.entries[locale] = make(map[string]string)
	}
	c.entries[locale][key] = value
}

// Merge registers every entry in entries (as Set would, one at a
// time), letting a deployment layer additional translations --
// e.g. a customer-supplied locale, or corrections to a seeded one --
// on top of NewSeededCatalog's starter set without needing package-
// level mutable state or a second Catalog type.
func (c *Catalog) Merge(entries []CatalogEntry) {
	for _, e := range entries {
		c.Set(e.Locale, e.Key, e.Value)
	}
}

// CatalogEntry is one (Locale, Key, Value) translation triple, the
// shape SeedCatalog returns and Merge consumes.
type CatalogEntry struct {
	Locale Locale
	Key    string
	Value  string
}

// has reports whether locale has a raw (non-fallback) entry for key,
// without recording a miss and without formatting args. Caller must
// hold c.mu for reading.
func (c *Catalog) has(locale Locale, key string) (string, bool) {
	byKey, ok := c.entries[locale]
	if !ok {
		return "", false
	}
	v, ok := byKey[key]
	return v, ok
}

// recordMissing notes that locale had no entry for key and Translate
// had to fall back, for later MissingKeys reporting. Caller must NOT
// hold c.mu (recordMissing takes its own write lock).
func (c *Catalog) recordMissing(locale Locale, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.missing[locale] == nil {
		c.missing[locale] = make(map[string]struct{})
	}
	c.missing[locale][key] = struct{}{}
}

// Translate resolves key for locale, applying fmt.Sprintf-style args if
// the resolved template contains verbs, with fallback-to-English (task
// 8) when locale has no entry for key:
//
//  1. If locale itself has an entry for key, use it.
//  2. Otherwise, if locale != FallbackLocale and FallbackLocale has an
//     entry for key, use that, and record the gap (locale, key) via
//     recordMissing so MissingKeys(locale) reports it (task 7) --
//     this is what makes fallback "real logic", not a silent no-op:
//     the gap is durably observable afterward.
//  3. Otherwise (key is missing even from FallbackLocale, an authoring
//     bug -- every seeded key must have an English translation),
//     return key itself, wrapped in "!(key)!" so a missing translation
//     is visually obvious in a rendered UI rather than silently
//     blank, mirroring the "missing translation" convention common to
//     i18n libraries (e.g. gettext's msgid passthrough,
//     react-i18next's default key passthrough).
//
// Translate never panics and never returns an error -- see
// MustTranslate for the stricter variant used in tests/tooling that
// should fail loudly on case 3.
func Translate(c *Catalog, locale Locale, key string, args ...any) string {
	if c == nil {
		return fmt.Sprintf("!(%s)!", key)
	}

	c.mu.RLock()
	v, ok := c.has(locale, key)
	c.mu.RUnlock()
	if ok {
		return applyArgs(v, args)
	}

	if locale != FallbackLocale {
		c.mu.RLock()
		fv, fok := c.has(FallbackLocale, key)
		c.mu.RUnlock()
		if fok {
			c.recordMissing(locale, key)
			return applyArgs(fv, args)
		}
	}

	return fmt.Sprintf("!(%s)!", key)
}

// MustTranslate is Translate's stricter counterpart: it returns
// ErrUnknownKey wrapped with key if the key is missing even from
// FallbackLocale (Translate's case 3), instead of silently returning a
// "!(key)!" placeholder. Intended for tests and build-time tooling that
// should fail loudly on a genuinely unknown key, as opposed to a
// merely-untranslated-in-this-locale gap (which is not an error -- see
// Translate).
func MustTranslate(c *Catalog, locale Locale, key string, args ...any) (string, error) {
	if c == nil {
		return "", ErrNilCatalog
	}
	c.mu.RLock()
	_, fok := c.has(FallbackLocale, key)
	c.mu.RUnlock()
	if !fok {
		return "", fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	return Translate(c, locale, key, args...), nil
}

// applyArgs runs fmt.Sprintf(template, args...) when args is non-empty,
// otherwise returns template unchanged (so a plain translated string
// with a literal "%" in it -- e.g. a percentage sign in translated
// prose -- is never accidentally treated as a format verb when no args
// were supplied).
func applyArgs(template string, args []any) string {
	if len(args) == 0 {
		return template
	}
	return fmt.Sprintf(template, args...)
}

// MissingKeys returns every key that has been requested for locale (via
// Translate) but resolved only via fallback to FallbackLocale, in
// sorted order, for a translation-management workflow to act on (task
// 7). It reflects gaps *observed so far in this process* -- keys never
// requested for locale are not reported, since Catalog does not
// maintain a registry of "every key that should exist" independent of
// FallbackLocale's own key set (AllKeys provides that superset when a
// caller wants to proactively diff rather than wait for a miss to be
// observed).
func (c *Catalog) MissingKeys(locale Locale) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	byKey := c.missing[locale]
	if len(byKey) == 0 {
		return nil
	}
	out := make([]string, 0, len(byKey))
	for k := range byKey {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// AllKeys returns every translation key registered for FallbackLocale,
// sorted -- the full key inventory a translation-management workflow
// can diff a target locale against proactively (see
// UntranslatedKeys), rather than waiting for a runtime Translate call
// to observe a gap.
func (c *Catalog) AllKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	byKey := c.entries[FallbackLocale]
	out := make([]string, 0, len(byKey))
	for k := range byKey {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// UntranslatedKeys returns every key present in FallbackLocale but
// absent from locale, sorted -- a proactive translation-management gap
// report (task 7) that does not depend on Translate having been called
// yet, complementing MissingKeys's "observed at runtime" report.
func (c *Catalog) UntranslatedKeys(locale Locale) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	fallback := c.entries[FallbackLocale]
	target := c.entries[locale]
	out := make([]string, 0)
	for k := range fallback {
		if target == nil {
			out = append(out, k)
			continue
		}
		if _, ok := target[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// CoveragePercent returns the fraction (0-100) of FallbackLocale's keys
// that locale also has a translation for, rounded down to the nearest
// integer -- a compact translation-management summary metric (task 7)
// alongside the fuller UntranslatedKeys/MissingKeys key lists.
// CoveragePercent(FallbackLocale) is always 100.
func (c *Catalog) CoveragePercent(locale Locale) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	total := len(c.entries[FallbackLocale])
	if total == 0 {
		return 100
	}
	target := c.entries[locale]
	have := 0
	for k := range c.entries[FallbackLocale] {
		if _, ok := target[k]; ok {
			have++
		}
	}
	return (have * 100) / total
}

// Locales returns every Locale this Catalog has at least one entry
// for, sorted for deterministic output.
func (c *Catalog) Locales() []Locale {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Locale, 0, len(c.entries))
	for l := range c.entries {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// normalizeKey is a small helper other files use to build consistent
// dotted translation keys (e.g. "case_status.draft",
// "action.ingest_evidence") from a namespace and a name.
func normalizeKey(namespace, name string) string {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "" {
		return name
	}
	return namespace + "." + name
}
