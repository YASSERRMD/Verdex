package ocr

import "context"

// TableCell holds the extracted text for one cell of a Table, plus its
// position within the grid.
type TableCell struct {
	// Row is the 0-based row index of this cell.
	Row int
	// Col is the 0-based column index of this cell.
	Col int
	// Text is the extracted text content of the cell.
	Text string
	// Confidence is the extraction confidence for this cell's text, in the
	// closed interval [0, 1].
	Confidence float64
}

// Table is a structured grid of cell text extracted from a region
// classified as RegionTypeTable.
type Table struct {
	// Page is the 1-based page number this table was extracted from.
	Page int

	// BoundingBox locates the table on the page, in pixel coordinates.
	BoundingBox BoundingBox

	// Rows is the number of rows in the table grid.
	Rows int

	// Cols is the number of columns in the table grid.
	Cols int

	// Cells holds every cell in the table. A well-formed Table has exactly
	// Rows*Cols entries in Cells, one per (Row, Col) pair, though extractors
	// may omit cells they could not segment (see TableIsConsistent).
	Cells []TableCell
}

// CellAt returns the TableCell at (row, col) and true if present, or the
// zero TableCell and false if no cell occupies that position.
func (t *Table) CellAt(row, col int) (TableCell, bool) {
	if t == nil {
		return TableCell{}, false
	}
	for _, c := range t.Cells {
		if c.Row == row && c.Col == col {
			return c, true
		}
	}
	return TableCell{}, false
}

// TableIsConsistent reports whether every cell's (Row, Col) lies within
// [0, Rows) x [0, Cols) and no two cells share the same position. Extraction
// hooks should produce consistent tables; this helper exists primarily for
// tests and defensive validation.
func TableIsConsistent(t Table) bool {
	seen := make(map[[2]int]bool, len(t.Cells))
	for _, c := range t.Cells {
		if c.Row < 0 || c.Row >= t.Rows || c.Col < 0 || c.Col >= t.Cols {
			return false
		}
		key := [2]int{c.Row, c.Col}
		if seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

// TableExtractor extracts structured Table data from a Region previously
// classified as RegionTypeTable.
//
// Table extraction is deliberately a separate hook from LayoutDetector: a
// LayoutDetector only identifies *that* a region is a table (its bounding
// box), while a TableExtractor performs the finer-grained row/column
// segmentation within that region. Implementations MUST be deterministic
// for a given input so tests can assert exact output.
type TableExtractor interface {
	// ExtractTable returns the Table for region, scoped to input. It must
	// not mutate input.Data. region.Type is not required to be
	// RegionTypeTable; implementations should return an empty Table (or
	// ErrInvalidRequest) for non-table regions.
	ExtractTable(ctx context.Context, input ImageInput, region Region) (*Table, error)
}

// NoOpTableExtractor is a TableExtractor that always returns an empty,
// zero-dimension Table. Use this when no real table-structure-extraction
// backend is configured. This is the default TableExtractor for OCRService.
type NoOpTableExtractor struct{}

// ExtractTable implements TableExtractor. It is a deterministic no-op.
func (NoOpTableExtractor) ExtractTable(_ context.Context, _ ImageInput, region Region) (*Table, error) {
	return &Table{
		Page:        region.Page,
		BoundingBox: region.BoundingBox,
	}, nil
}

// ExtractTablesFromRegions runs extractor against every region in regions
// classified as RegionTypeTable, returning the resulting Tables in the same
// order the qualifying regions appeared in.
func ExtractTablesFromRegions(ctx context.Context, extractor TableExtractor, input ImageInput, regions []Region) ([]Table, error) {
	if extractor == nil {
		extractor = NoOpTableExtractor{}
	}

	var tables []Table
	for _, r := range regions {
		if r.Type != RegionTypeTable {
			continue
		}
		t, err := extractor.ExtractTable(ctx, input, r)
		if err != nil {
			return nil, err
		}
		if t != nil {
			tables = append(tables, *t)
		}
	}
	return tables, nil
}
