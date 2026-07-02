package hybridretrieval

// isZeroFilter reports whether f restricts nothing (every field empty),
// the common case where graph-expansion candidates never need a metadata
// lookup at all.
func isZeroFilter(f Filter) bool {
	return f.JurisdictionCode == "" && f.CategoryCode == "" && f.PartyID == ""
}

// filterGraphHits removes graphHits whose node fails query.Filter,
// resolved via query.MetadataLookup. Vector-recall hits are never passed
// through this function: vectorindex.VectorStore.Query already applied
// the identical Filter (via MetadataFilter.Matches) during recall, so
// re-filtering them here would be redundant — see doc/hybrid-retrieval.md
// "Filters applied consistently across both paths".
//
// When query.Filter is the zero value, every hit passes without consulting
// MetadataLookup at all (a no-op filter should never require metadata to
// be resolvable). When Filter is non-zero and MetadataLookup is nil, or
// the lookup can't resolve a given node, that node is excluded: a
// caller asking for a jurisdiction/category-restricted hybrid query
// without supplying a way to check graph-only nodes against it gets a
// conservative (exclude, don't guess) result rather than a silently
// unfiltered one.
func filterGraphHits(query HybridQuery, hits []graphHit) []graphHit {
	if isZeroFilter(query.Filter) {
		return hits
	}

	out := make([]graphHit, 0, len(hits))
	for _, h := range hits {
		if query.MetadataLookup == nil {
			continue
		}
		meta, ok := query.MetadataLookup(h.nodeID)
		if !ok || !query.Filter.matches(meta) {
			continue
		}
		out = append(out, h)
	}
	return out
}
