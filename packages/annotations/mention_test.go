package annotations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
)

func TestExtractMentions(t *testing.T) {
	userA := uuid.New()
	userB := uuid.New()

	tests := []struct {
		name string
		body string
		want []uuid.UUID
	}{
		{
			name: "no mentions",
			body: "Nothing to see here.",
			want: nil,
		},
		{
			name: "single mention",
			body: "cc @" + userA.String(),
			want: []uuid.UUID{userA},
		},
		{
			name: "multiple distinct mentions in order",
			body: "cc @" + userA.String() + " and @" + userB.String(),
			want: []uuid.UUID{userA, userB},
		},
		{
			name: "duplicate mention collapses to one",
			body: "@" + userA.String() + " ping @" + userA.String() + " again",
			want: []uuid.UUID{userA},
		},
		{
			name: "malformed token ignored",
			body: "not-a-uuid @notauuid and a real one @" + userA.String(),
			want: []uuid.UUID{userA},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := annotations.ExtractMentions(tc.body)
			if len(got) != len(tc.want) {
				t.Fatalf("ExtractMentions(%q) = %v, want %v", tc.body, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ExtractMentions(%q)[%d] = %s, want %s", tc.body, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestMultiMentionSink_FansOutToAllSinks(t *testing.T) {
	sinkA := &recordingSink{}
	sinkB := &recordingSink{}
	multi := &annotations.MultiMentionSink{Sinks: []annotations.MentionSink{sinkA, sinkB}}

	mention := annotations.Mention{
		AnnotationID:    uuid.New(),
		MentionedUserID: uuid.New(),
	}
	if err := multi.Notify(context.Background(), mention); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if len(sinkA.received) != 1 || len(sinkB.received) != 1 {
		t.Fatalf("expected both sinks to receive the mention, got %d and %d", len(sinkA.received), len(sinkB.received))
	}
}
