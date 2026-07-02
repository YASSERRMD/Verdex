package ocr_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestTable_CellAt(t *testing.T) {
	table := ocr.Table{
		Rows: 2,
		Cols: 2,
		Cells: []ocr.TableCell{
			{Row: 0, Col: 0, Text: "a1", Confidence: 0.9},
			{Row: 0, Col: 1, Text: "b1", Confidence: 0.9},
			{Row: 1, Col: 0, Text: "a2", Confidence: 0.9},
			{Row: 1, Col: 1, Text: "b2", Confidence: 0.9},
		},
	}

	cell, ok := table.CellAt(1, 1)
	if !ok {
		t.Fatal("CellAt(1, 1) expected found, got not-found")
	}
	if cell.Text != "b2" {
		t.Errorf("CellAt(1, 1).Text = %q, want %q", cell.Text, "b2")
	}

	_, ok = table.CellAt(5, 5)
	if ok {
		t.Error("CellAt(5, 5) expected not-found, got found")
	}
}

func TestTableIsConsistent(t *testing.T) {
	tests := []struct {
		name string
		t    ocr.Table
		want bool
	}{
		{
			name: "consistent_full_grid",
			t: ocr.Table{
				Rows: 2, Cols: 2,
				Cells: []ocr.TableCell{
					{Row: 0, Col: 0}, {Row: 0, Col: 1},
					{Row: 1, Col: 0}, {Row: 1, Col: 1},
				},
			},
			want: true,
		},
		{
			name: "consistent_partial_grid",
			t: ocr.Table{
				Rows: 2, Cols: 2,
				Cells: []ocr.TableCell{
					{Row: 0, Col: 0},
				},
			},
			want: true,
		},
		{
			name: "out_of_bounds_row",
			t: ocr.Table{
				Rows: 1, Cols: 2,
				Cells: []ocr.TableCell{
					{Row: 1, Col: 0},
				},
			},
			want: false,
		},
		{
			name: "out_of_bounds_col",
			t: ocr.Table{
				Rows: 2, Cols: 1,
				Cells: []ocr.TableCell{
					{Row: 0, Col: 1},
				},
			},
			want: false,
		},
		{
			name: "duplicate_position",
			t: ocr.Table{
				Rows: 2, Cols: 2,
				Cells: []ocr.TableCell{
					{Row: 0, Col: 0, Text: "first"},
					{Row: 0, Col: 0, Text: "dup"},
				},
			},
			want: false,
		},
		{
			name: "empty_table",
			t:    ocr.Table{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ocr.TableIsConsistent(tt.t); got != tt.want {
				t.Errorf("TableIsConsistent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNoOpTableExtractor_ReturnsEmptyTable(t *testing.T) {
	e := ocr.NoOpTableExtractor{}
	input := ocr.ImageInput{Data: []byte("synthetic page bytes")}
	region := ocr.Region{
		Page:        1,
		BoundingBox: ocr.BoundingBox{X: 5, Y: 5, Width: 100, Height: 50},
		Type:        ocr.RegionTypeTable,
	}

	table, err := e.ExtractTable(context.Background(), input, region)
	if err != nil {
		t.Fatalf("ExtractTable() unexpected error: %v", err)
	}
	if table.Rows != 0 || table.Cols != 0 {
		t.Errorf("ExtractTable() = %+v, want zero-dimension table", table)
	}
	if table.BoundingBox != region.BoundingBox {
		t.Errorf("BoundingBox = %+v, want %+v", table.BoundingBox, region.BoundingBox)
	}
	if !ocr.TableIsConsistent(*table) {
		t.Error("NoOpTableExtractor produced an inconsistent table")
	}
}

func TestExtractTablesFromRegions_OnlyScopesTableRegions(t *testing.T) {
	input := ocr.ImageInput{Data: []byte("synthetic page bytes")}
	regions := []ocr.Region{
		{Page: 1, Type: ocr.RegionTypeParagraph, BoundingBox: ocr.BoundingBox{Width: 100, Height: 20}},
		{Page: 1, Type: ocr.RegionTypeTable, BoundingBox: ocr.BoundingBox{Width: 200, Height: 80}},
		{Page: 1, Type: ocr.RegionTypeFigure, BoundingBox: ocr.BoundingBox{Width: 50, Height: 50}},
		{Page: 2, Type: ocr.RegionTypeTable, BoundingBox: ocr.BoundingBox{Width: 150, Height: 60}},
	}

	tables, err := ocr.ExtractTablesFromRegions(context.Background(), ocr.NoOpTableExtractor{}, input, regions)
	if err != nil {
		t.Fatalf("ExtractTablesFromRegions() unexpected error: %v", err)
	}
	if len(tables) != 2 {
		t.Fatalf("ExtractTablesFromRegions() returned %d tables, want 2", len(tables))
	}
	if tables[0].Page != 1 || tables[1].Page != 2 {
		t.Errorf("tables not in region order: got pages %d, %d, want 1, 2", tables[0].Page, tables[1].Page)
	}
}

func TestExtractTablesFromRegions_NilExtractorUsesNoOp(t *testing.T) {
	input := ocr.ImageInput{Data: []byte("synthetic page bytes")}
	regions := []ocr.Region{
		{Page: 1, Type: ocr.RegionTypeTable, BoundingBox: ocr.BoundingBox{Width: 100, Height: 40}},
	}

	tables, err := ocr.ExtractTablesFromRegions(context.Background(), nil, input, regions)
	if err != nil {
		t.Fatalf("ExtractTablesFromRegions() unexpected error: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("ExtractTablesFromRegions() returned %d tables, want 1", len(tables))
	}
}

func TestExtractTablesFromRegions_NoTableRegions_ReturnsEmpty(t *testing.T) {
	input := ocr.ImageInput{Data: []byte("synthetic page bytes")}
	regions := []ocr.Region{
		{Page: 1, Type: ocr.RegionTypeParagraph},
		{Page: 1, Type: ocr.RegionTypeHeading},
	}

	tables, err := ocr.ExtractTablesFromRegions(context.Background(), ocr.NoOpTableExtractor{}, input, regions)
	if err != nil {
		t.Fatalf("ExtractTablesFromRegions() unexpected error: %v", err)
	}
	if len(tables) != 0 {
		t.Errorf("ExtractTablesFromRegions() returned %d tables, want 0", len(tables))
	}
}
