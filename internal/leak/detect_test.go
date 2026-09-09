package leak

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SuperMarioYL/leakmap/internal/secret"
)

func idxFrom(prints []secret.Fingerprint) *secret.Index { return secret.NewIndex(prints) }

func fp(wt, field, value string) secret.Fingerprint {
	return secret.Fingerprint{
		Worktree:       wt,
		Path:           "/r/" + wt + "/.env",
		Field:          field,
		ValueHash:      secret.HashValue(value),
		Classification: secret.Classify(value, field),
		Value:          value,
	}
}

func TestMatchExactCrossBoundary(t *testing.T) {
	// wt-a holds DB_TOKEN; a write to wt-b that contains it must produce one
	// exact leak event attributed to wt-a -> wt-b.
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DB_TOKEN", "super-secret-token-1234567890"),
	})
	content := "const db = connect(\"super-secret-token-1234567890\")\n"
	got := Match(content, "wt-b", "/r/wt-b/src/db.go", 4242, idx)
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d (%+v)", len(got), got)
	}
	e := got[0]
	if e.SourceWorktree != "wt-a" || e.TargetWorktree != "wt-b" {
		t.Errorf("attribution wrong: %+v", e)
	}
	if e.MatchKind != MatchExact {
		t.Errorf("kind %q want exact", e.MatchKind)
	}
	if e.TargetAgentPID != 4242 {
		t.Errorf("target pid %d want 4242", e.TargetAgentPID)
	}
	if e.SecretField != "DB_TOKEN" {
		t.Errorf("field %q want DB_TOKEN", e.SecretField)
	}
}

func TestMatchNoSameWorktree(t *testing.T) {
	// A write to the same worktree that holds the fingerprint is NOT a leak.
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DB_TOKEN", "super-secret-token-1234567890"),
	})
	content := "echo super-secret-token-1234567890\n"
	got := Match(content, "wt-a", "/r/wt-a/x.txt", 0, idx)
	if len(got) != 0 {
		t.Fatalf("self-leak must not fire, got %d (%+v)", len(got), got)
	}
}

func TestMatchNoFalsePositiveShort(t *testing.T) {
	// Very short values (< minMatchLen) must not match.
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DEBUG", "true"),
	})
	content := "DEBUG=true\n"
	got := Match(content, "wt-b", "/r/wt-b/x", 0, idx)
	if len(got) != 0 {
		t.Fatalf("short-value false positive fired, got %d", len(got))
	}
}

func TestMatchFuzzyNearDuplicate(t *testing.T) {
	// A lightly mutated token (one char changed) should be caught by the
	// fuzzy path, not exact.
	value := "ghp_abcdefghijklmnopqrstuvwxyz0123456789AB" // 40 chars
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "GH_TOKEN", value),
	})
	// Change last char: still very similar but not equal.
	mutated := value[:len(value)-1] + "C"
	content := "token := " + mutated + "\n"
	got := Match(content, "wt-b", "/r/wt-b/y", 0, idx)
	if len(got) != 1 {
		t.Fatalf("want 1 fuzzy event, got %d (%+v)", len(got), got)
	}
	if got[0].MatchKind != MatchFuzzy {
		t.Errorf("kind %q want fuzzy", got[0].MatchKind)
	}
}

func TestMatchNilIndex(t *testing.T) {
	got := Match("anything", "wt-b", "/r/wt-b/x", 0, nil)
	if len(got) != 0 {
		t.Fatalf("nil index should yield no events, got %d", len(got))
	}
}

func TestMatchMultipleFingerprints(t *testing.T) {
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DB_TOKEN", "tok-aaaaaaaaaaaaaa"),
		fp("wt-a", "API_KEY", "key-bbbbbbbbbbbbbb"),
		fp("wt-c", "SECRET", "sec-cccccccccccccc"),
	})
	content := "tok-aaaaaaaaaaaaaa and key-bbbbbbbbbbbbbb and unrelated\n"
	got := Match(content, "wt-b", "/r/wt-b/z", 0, idx)
	if len(got) != 2 {
		t.Fatalf("want 2 events (wt-a both), got %d (%+v)", len(got), got)
	}
	for _, e := range got {
		if e.SourceWorktree != "wt-a" {
			t.Errorf("source %q want wt-a", e.SourceWorktree)
		}
	}
}

func TestDetectorHandleFillsSourceAgentPID(t *testing.T) {
	// The detector knows both worktrees' agent PIDs; the emitted event must
	// attribute the leak to the source agent, not just the target.
	root := t.TempDir() // stands in for the watched "wt-b" root
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DB_TOKEN", "super-secret-token-1234567890"),
	})
	var got []Event
	d := &Detector{
		index:    idx,
		pidByWt:  map[string]int{"wt-a": 4242, root: 2718},
		roots:    []string{root},
		emit:     func(e Event) { got = append(got, e) },
		minDelay: 0,
	}
	target := filepath.Join(root, "notes.md")
	if err := os.WriteFile(target, []byte("TOKEN=super-secret-token-1234567890\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d.handle(target, nil)
	if len(got) != 1 {
		t.Fatalf("want 1 event, got %d (%+v)", len(got), got)
	}
	if got[0].SourceAgentPID != 4242 {
		t.Errorf("source agent pid %d, want 4242", got[0].SourceAgentPID)
	}
	if got[0].TargetAgentPID != 2718 {
		t.Errorf("target agent pid %d, want 2718", got[0].TargetAgentPID)
	}
}

func TestMatchExactSurvivesFuzzyCap(t *testing.T) {
	// A verbatim secret at the very end of content larger than fuzzyScanCap
	// must still fire: exact matching covers the whole read-capped content.
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "DB_TOKEN", "super-secret-token-1234567890"),
	})
	content := strings.Repeat("x", fuzzyScanCap+4096) + "TOKEN=super-secret-token-1234567890"
	got := Match(content, "wt-b", "/r/wt-b/bundle.js", 0, idx)
	if len(got) != 1 || got[0].MatchKind != MatchExact {
		t.Fatalf("want 1 exact event above fuzzyScanCap, got %d (%+v)", len(got), got)
	}
}

func TestMatchFuzzySkippedAboveCap(t *testing.T) {
	// Documented cap behavior: only a near-duplicate (non-verbatim) mutation
	// inside content larger than fuzzyScanCap produces no fuzzy event — the
	// quadratic sliding-window scan is skipped to keep the watch loop alive.
	value := "ghp_abcdefghijklmnopqrstuvwxyz0123456789AB"
	idx := idxFrom([]secret.Fingerprint{
		fp("wt-a", "GH_TOKEN", value),
	})
	mutated := value[:len(value)-1] + "C"
	content := strings.Repeat("pad ", 40*1024) + "token := " + mutated + "\n"
	if len(content) <= fuzzyScanCap {
		t.Fatalf("test content must exceed fuzzyScanCap (%d bytes)", len(content))
	}
	got := Match(content, "wt-b", "/r/wt-b/y", 0, idx)
	if len(got) != 0 {
		t.Fatalf("fuzzy scan must be skipped above fuzzyScanCap, got %d (%+v)", len(got), got)
	}
}

func TestReadCapped(t *testing.T) {
	// Files larger than maxReadBytes are read up to the cap only.
	path := filepath.Join(t.TempDir(), "big.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, maxReadBytes+1024)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	data, err := readCapped(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != maxReadBytes {
		t.Fatalf("read %d bytes, want capped at %d", len(data), maxReadBytes)
	}
}
