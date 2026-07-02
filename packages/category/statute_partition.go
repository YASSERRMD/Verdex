package category

// StatutePartitionRef is a reference to a partition of a jurisdiction's
// statute corpus that is applicable to cases of a given category.
//
// This is a forward-looking hook: packages/statute (Phase 035) will later
// own the actual statute corpus and populate richer partition metadata.
// For now this stays a simple string-keyed reference with no dependency on
// packages/statute, so packages/category can be built and tested
// independently ahead of that phase landing.
type StatutePartitionRef struct {
	// PartitionID is a short machine-readable identifier for the statute
	// partition (e.g. "IN-IPC", "AE-PENAL-CODE", "US-UCC-ART2"). The exact
	// namespacing scheme is owned by packages/statute; this package only
	// stores and looks up the identifier string.
	PartitionID string `json:"partition_id"`

	// Description provides a brief human-readable overview of what this
	// statute partition covers.
	Description string `json:"description,omitempty"`
}

// statutePartitionTable maps jurisdictionCode -> CategoryCode -> the set of
// StatutePartitionRefs applicable to that category in that jurisdiction.
type statutePartitionTable map[string]map[CategoryCode][]StatutePartitionRef

// StatutePartitions is a lookup table mapping a case Category to the
// statute partitions applicable to it, per jurisdiction. The zero value is
// ready to use (an empty table).
type StatutePartitions struct {
	table statutePartitionTable
}

// NewStatutePartitions constructs an empty StatutePartitions lookup table.
func NewStatutePartitions() *StatutePartitions {
	return &StatutePartitions{table: make(statutePartitionTable)}
}

// Register associates refs with categoryCode for jurisdictionCode,
// appending to (not replacing) any refs already registered for that pair.
func (s *StatutePartitions) Register(jurisdictionCode string, categoryCode CategoryCode, refs ...StatutePartitionRef) {
	if s.table == nil {
		s.table = make(statutePartitionTable)
	}
	byCategory, ok := s.table[jurisdictionCode]
	if !ok {
		byCategory = make(map[CategoryCode][]StatutePartitionRef)
		s.table[jurisdictionCode] = byCategory
	}
	byCategory[categoryCode] = append(byCategory[categoryCode], refs...)
}

// Lookup returns every StatutePartitionRef registered for categoryCode
// within jurisdictionCode. Returns nil if nothing is registered for that
// pair.
func (s *StatutePartitions) Lookup(jurisdictionCode string, categoryCode CategoryCode) []StatutePartitionRef {
	if s.table == nil {
		return nil
	}
	byCategory, ok := s.table[jurisdictionCode]
	if !ok {
		return nil
	}
	return byCategory[categoryCode]
}

// LookupCategory is a convenience wrapper around Lookup that takes a full
// Category value rather than a bare CategoryCode.
func (s *StatutePartitions) LookupCategory(jurisdictionCode string, cat Category) []StatutePartitionRef {
	return s.Lookup(jurisdictionCode, cat.Code)
}
