package bulkimport

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// ComputeDedupKey derives a stable deduplication key for an
// ImportRecord from its case number, jurisdiction, and party names
// (task 5). The key is a SHA-256 hex digest of a normalized
// (lower-cased, whitespace-trimmed, party-names-sorted) composite of
// those three fields, so:
//
//   - Two records referring to the same case (same case number,
//     jurisdiction, and party set) hash identically regardless of
//     source formatting differences in case (e.g. "Doe, Jane" vs
//     "doe, jane") or party-name ordering.
//   - A blank case number never collides with another blank case
//     number across different jurisdictions/parties, since
//     jurisdiction and party names still contribute to the hash.
//
// Returns "" if both caseNumber and jurisdiction are blank and
// partyNames is empty -- there is nothing meaningful to dedup on, so
// this package never treats such a record as a duplicate of another
// equally-empty one.
func ComputeDedupKey(caseNumber, jurisdiction string, partyNames []string) string {
	normCase := strings.ToLower(strings.TrimSpace(caseNumber))
	normJurisdiction := strings.ToLower(strings.TrimSpace(jurisdiction))

	normParties := make([]string, 0, len(partyNames))
	for _, p := range partyNames {
		trimmed := strings.ToLower(strings.TrimSpace(p))
		if trimmed != "" {
			normParties = append(normParties, trimmed)
		}
	}
	sort.Strings(normParties)

	if normCase == "" && normJurisdiction == "" && len(normParties) == 0 {
		return ""
	}

	composite := normCase + "|" + normJurisdiction + "|" + strings.Join(normParties, ",")
	sum := sha256.Sum256([]byte(composite))
	return hex.EncodeToString(sum[:])
}
