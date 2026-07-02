package timeline

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

// Event is a single occurrence in the chronological case timeline: what
// happened, when (if known), and how confident the extraction is.
type Event struct {
	// ID uniquely identifies this event within its case.
	ID string

	// Description is a short human-readable description of what occurred,
	// typically derived from the source segment's text.
	Description string

	// OccurredAt is the date this event occurred, when it could be
	// determined. Nil means the date is unknown or too approximate to
	// resolve to a concrete calendar date — many events in a legal record
	// have imprecise or missing dates, and Timeline assembly (see
	// assemble.go) handles that case explicitly rather than guessing.
	OccurredAt *time.Time

	// Confidence is this event's extraction confidence score, in the
	// closed interval [0, 1].
	Confidence float64

	// SegmentID identifies the segmentation.Segment this event was
	// extracted from, when applicable. Empty when the event was
	// constructed without a source segment.
	SegmentID string

	// PartyID optionally identifies the Party this event is attributed
	// to. Empty when no party attribution applies or is known.
	PartyID string
}

// datePatterns are deterministic, regex-based date extractors, checked in
// order. Each pattern's capture groups are consumed by its paired parse
// function in dateParsers. No ML models are used, mirroring
// packages/segmentation and packages/evidence's "no ML models, rule based"
// design principle.
var (
	// isoDatePattern matches ISO-8601-shaped dates: 2024-03-15.
	isoDatePattern = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

	// longDatePattern matches "March 15, 2024" / "March 15 2024" style
	// dates.
	longDatePattern = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),?\s+(\d{4})\b`)

	// slashDatePattern matches "03/15/2024" (month/day/year) style dates.
	slashDatePattern = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4})\b`)
)

var monthNames = map[string]time.Month{
	"january": time.January, "february": time.February, "march": time.March,
	"april": time.April, "may": time.May, "june": time.June,
	"july": time.July, "august": time.August, "september": time.September,
	"october": time.October, "november": time.November, "december": time.December,
}

// ExtractDate attempts to find the first recognizable calendar date within
// text using deterministic regex patterns (ISO, long-form, and
// slash-separated), in that priority order. Returns the parsed date (UTC,
// midnight) and a confidence score, or ok=false if no pattern matched.
//
// ISO dates are checked first because their shape is unambiguous; long-form
// dates second because a spelled-out month name is a strong, low-ambiguity
// signal; slash dates last because "03/15/2024" is ambiguous with
// day/month/year conventions outside the US and so scores lower confidence.
func ExtractDate(text string) (t time.Time, confidence float64, ok bool) {
	if m := isoDatePattern.FindStringSubmatch(text); m != nil {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		if d, valid := buildDate(year, time.Month(month), day); valid {
			return d, 0.95, true
		}
	}
	if m := longDatePattern.FindStringSubmatch(text); m != nil {
		month, known := monthNames[strings.ToLower(m[1])]
		day, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if known {
			if d, valid := buildDate(year, month, day); valid {
				return d, 0.9, true
			}
		}
	}
	if m := slashDatePattern.FindStringSubmatch(text); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		if d, valid := buildDate(year, time.Month(month), day); valid {
			return d, 0.6, true
		}
	}
	return time.Time{}, 0, false
}

// buildDate constructs a UTC midnight time.Time for year/month/day and
// reports whether it round-trips to the same calendar date (rejecting
// out-of-range values like month 13 or day 32, which time.Date would
// otherwise silently normalize into a different date).
func buildDate(year int, month time.Month, day int) (time.Time, bool) {
	if year < 1000 || year > 9999 {
		return time.Time{}, false
	}
	if month < time.January || month > time.December {
		return time.Time{}, false
	}
	if day < 1 || day > 31 {
		return time.Time{}, false
	}
	d := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if d.Year() != year || d.Month() != month || d.Day() != day {
		return time.Time{}, false
	}
	return d, true
}

// ExtractEvent builds an Event from seg: its Description is seg.Text, its
// ID is id, and its OccurredAt/Confidence are populated from the first
// date ExtractDate finds in seg.Text (nil/0 when none is found, per
// Event.OccurredAt's documented "unknown date" contract).
func ExtractEvent(id string, seg segmentation.Segment, partyID string) Event {
	ev := Event{
		ID:          id,
		Description: seg.Text,
		SegmentID:   seg.ID,
		PartyID:     partyID,
	}
	if d, conf, ok := ExtractDate(seg.Text); ok {
		ev.OccurredAt = &d
		ev.Confidence = conf
	}
	return ev
}
