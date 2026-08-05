package leak

import (
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
