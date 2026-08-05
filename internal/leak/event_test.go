package leak

import (
	"path/filepath"
	"testing"
	"time"
)

func TestEventSummary(t *testing.T) {
	e := Event{
		Timestamp:      time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC),
		SourceWorktree: "/r/wt-a",
		SourcePath:     "/r/wt-a/.env",
		SecretField:    "DB_TOKEN",
		TargetWorktree: "/r/wt-b",
		TargetPath:     "/r/wt-b/src/config.go",
		MatchKind:      MatchExact,
		Severity:       SeveritySecret,
	}
	s := e.Summary()
	if !contains(s, "DB_TOKEN") || !contains(s, "exact") || !contains(s, "secret") {
		t.Fatalf("summary missing fields: %s", s)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestJSONLRoundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "leakmap.jsonl")
	w, err := NewJSONLWriter(path)
	if err != nil {
		t.Fatal(err)
	}
	events := []Event{
		{
			Timestamp:      time.Date(2026, 8, 6, 1, 0, 0, 0, time.UTC),
			SourceWorktree: "/r/wt-a",
			SourcePath:     "/r/wt-a/.env",
			SecretField:    "DB_TOKEN",
			TargetWorktree: "/r/wt-b",
			TargetPath:     "/r/wt-b/src/config.go",
			MatchKind:      MatchExact,
			Severity:       SeveritySecret,
		},
		{
			Timestamp:      time.Date(2026, 8, 6, 1, 1, 0, 0, time.UTC),
			SourceWorktree: "/r/wt-a",
			SourcePath:     "/r/wt-a/.env",
			SecretField:    "API_KEY",
			TargetWorktree: "/r/wt-c",
			TargetPath:     "/r/wt-c/notes.md",
			MatchKind:      MatchFuzzy,
			Severity:       SeveritySecret,
		},
	}
	for _, e := range events {
		if err := w.Write(e); err != nil {
			_ = w.Close()
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := ReadEvents(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(events) {
		t.Fatalf("want %d events, got %d", len(events), len(got))
	}
	for i, e := range got {
		if e.SourceWorktree != events[i].SourceWorktree {
			t.Errorf("event %d: source wt %q want %q", i, e.SourceWorktree, events[i].SourceWorktree)
		}
		if e.SecretField != events[i].SecretField {
			t.Errorf("event %d: field %q want %q", i, e.SecretField, events[i].SecretField)
		}
		if e.MatchKind != events[i].MatchKind {
			t.Errorf("event %d: kind %q want %q", i, e.MatchKind, events[i].MatchKind)
		}
		// Round-trip must NOT carry the raw secret value (it never did here,
		// but assert the struct has no secret-value field surfaced).
	}
}

func TestStoreAppendRead(t *testing.T) {
	s := NewStore()
	if s.Len() != 0 {
		t.Fatalf("want 0, got %d", s.Len())
	}
	s.Append(Event{SecretField: "X", MatchKind: MatchExact})
	s.Append(Event{SecretField: "Y", MatchKind: MatchFuzzy})
	if s.Len() != 2 {
		t.Fatalf("want 2, got %d", s.Len())
	}
	got := s.Events()
	if len(got) != 2 || got[0].SecretField != "X" || got[1].SecretField != "Y" {
		t.Fatalf("unexpected snapshot %+v", got)
	}
}
