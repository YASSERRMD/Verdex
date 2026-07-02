package irac

import "testing"

func TestSourceSpan_Len(t *testing.T) {
	tests := []struct {
		name string
		s    SourceSpan
		want int
	}{
		{"normal", SourceSpan{Start: 5, End: 15}, 10},
		{"zero-length", SourceSpan{Start: 5, End: 5}, 0},
		{"inverted", SourceSpan{Start: 15, End: 5}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Len(); got != tt.want {
				t.Errorf("Len() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSourceSpan_IsValid(t *testing.T) {
	tests := []struct {
		name string
		s    SourceSpan
		want bool
	}{
		{"valid", SourceSpan{Start: 0, End: 10}, true},
		{"zero-length valid", SourceSpan{Start: 5, End: 5}, true},
		{"negative start", SourceSpan{Start: -1, End: 5}, false},
		{"end before start", SourceSpan{Start: 10, End: 5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSourceSpan_OCRAndSTTFields(t *testing.T) {
	s := SourceSpan{Start: 0, End: 5, Page: 3, StartMS: 1000, EndMS: 2000}
	if s.Page != 3 {
		t.Errorf("Page = %d, want 3", s.Page)
	}
	if s.StartMS != 1000 || s.EndMS != 2000 {
		t.Errorf("StartMS/EndMS = %d/%d, want 1000/2000", s.StartMS, s.EndMS)
	}
}
