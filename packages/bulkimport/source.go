package bulkimport

import "context"

// RecordSource is the pluggable read side of a bulk-import job: the
// source corpus being onboarded (a CSV export, a DMS export API, a
// stream of rows from a legacy database). Engine.RunBatch reads from a
// RecordSource rather than requiring an entire corpus to be loaded
// into memory up front, so a multi-million-row historical archive can
// be processed in bounded-size batches.
type RecordSource interface {
	// ReadAt returns up to count records starting at the zero-based
	// index, in the source corpus's stable order. It returns fewer
	// than count records (possibly zero) with done=true once the
	// source is exhausted -- this is not an error, it is how RunBatch
	// detects a job has reached StatusCompleted.
	//
	// A RecordSource must return records in a stable, deterministic
	// order across calls for the same index -- this is what makes
	// Cursor-based resumability correct (task 4): RunBatch must be
	// able to call ReadAt(cursor, batchSize) after a crash and get
	// exactly the records it would have gotten had it not crashed.
	ReadAt(ctx context.Context, index, count int) (records []SourceRecord, done bool, err error)
}

// SourceRecord is one raw row read from a RecordSource, before
// validation/dedup/import processing has run. It carries just enough
// structure for Engine.RunBatch to build an ImportRecord; a real
// deployment's RecordSource implementation is responsible for parsing
// whatever wire format (CSV, JSON, a DMS API response) the source
// corpus actually uses into this shape.
type SourceRecord struct {
	// PayloadRef is copied onto ImportRecord.PayloadRef unchanged.
	PayloadRef string

	// CaseNumber, Jurisdiction, and PartyNames are copied onto the
	// corresponding ImportRecord fields unchanged.
	CaseNumber   string
	Jurisdiction string
	PartyNames   []string
}

// InMemoryRecordSource is a RecordSource backed by a fixed in-memory
// slice, intended for tests and small corpora. Never for production
// use with a real multi-million-row archive -- see doc/bulk-import.md
// for why a real deployment implements RecordSource against its own
// storage instead.
type InMemoryRecordSource struct {
	records []SourceRecord
}

// NewInMemoryRecordSource builds an InMemoryRecordSource over records.
func NewInMemoryRecordSource(records []SourceRecord) *InMemoryRecordSource {
	return &InMemoryRecordSource{records: append([]SourceRecord(nil), records...)}
}

// ReadAt implements RecordSource.
func (s *InMemoryRecordSource) ReadAt(_ context.Context, index, count int) ([]SourceRecord, bool, error) {
	if index < 0 || count <= 0 {
		return nil, true, nil
	}
	if index >= len(s.records) {
		return nil, true, nil
	}
	end := index + count
	done := false
	if end >= len(s.records) {
		end = len(s.records)
		done = true
	}
	out := append([]SourceRecord(nil), s.records[index:end]...)
	return out, done, nil
}

// Len returns the total number of records in s.
func (s *InMemoryRecordSource) Len() int { return len(s.records) }

var _ RecordSource = (*InMemoryRecordSource)(nil)
