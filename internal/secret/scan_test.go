package secret

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHashAndMask(t *testing.T) {
	h := HashValue("super-secret-token")
	if len(h) != 16 {
		t.Fatalf("hash len %d want 16", len(h))
	}
	if h != HashValue("super-secret-token") {
		t.Fatal("hash not deterministic")
	}
	if HashValue("a") == HashValue("b") {
		t.Fatal("distinct values collided")
	}
	m := Mask("sk-abcd1234efgh5678")
	if m == "sk-abcd1234efgh5678" {
		t.Fatal("mask leaked full value")
	}
	if Mask("ab") == "ab" {
		t.Fatal("short value not masked")
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		value, field, want string
	}{
		{"AKIAIOSFODNN7EXAMPLE", "AWS_ACCESS_KEY_ID", ClassSecret},
		{"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", "AWS_SECRET_ACCESS_KEY", ClassSecret},
		{"ghp_abcdefghijklmnopqrstuvwxyz0123456789AB", "GITHUB_TOKEN", ClassSecret},
		{"xoxb-1234567890-abcdef", "SLACK_TOKEN", ClassSecret},
		{"-----BEGIN RSA PRIVATE KEY-----", "id_rsa", ClassSecret},
		{"postgres://user:pass@host:5432/db", "DSN", ClassConfig},
		{"https://example.com", "ENDPOINT", ClassConfig},
		{"5432", "PORT", ClassConfig},
		{"true", "DEBUG", ClassConfig},
		{"abc-12345", "SESSION_ID", ClassState},
		{"some-random-value", "PLAIN", ClassUnknown},
	}
	for _, c := range cases {
		got := Classify(c.value, c.field)
		if got != c.want {
			t.Errorf("Classify(%q,%q) = %q, want %q", c.value, c.field, got, c.want)
		}
	}
}

func TestClassifyWithModelStub(t *testing.T) {
	// No model configured → stub returns the regex verdict unchanged.
	got := ClassifyWithModel("opaque-token-with-no-field-signal", "PLAIN", ModelConfig{})
	if got != ClassUnknown {
		t.Errorf("stub model should be inert, got %q", got)
	}
}

func TestScanWorktreeEnv(t *testing.T) {
	root := t.TempDir()
	env := []byte("DB_TOKEN=super-secret-token-1234567890\n" +
		"# a comment\n" +
		"DEBUG=true\n" +
		"EMPTY=\n" +
		`QUOTED="quoted-value-abcdefgh"` + "\n")
	if err := os.WriteFile(filepath.Join(root, ".env"), env, 0o600); err != nil {
		t.Fatal(err)
	}
	prints, err := ScanWorktree("wt-a", root)
	if err != nil {
		t.Fatal(err)
	}
	// DB_TOKEN, DEBUG, QUOTED → 3 fingerprints (EMPTY skipped, comment skipped).
	if len(prints) != 3 {
		t.Fatalf("want 3 fingerprints, got %d (%+v)", len(prints), prints)
	}
	byField := map[string]Fingerprint{}
	for _, p := range prints {
		byField[p.Field] = p
	}
	if _, ok := byField["DB_TOKEN"]; !ok {
		t.Error("DB_TOKEN not fingerprinted")
	}
	if byField["DB_TOKEN"].Classification != ClassSecret {
		t.Errorf("DB_TOKEN classified %q, want secret", byField["DB_TOKEN"].Classification)
	}
	if byField["DB_TOKEN"].Value != "super-secret-token-1234567890" {
		t.Errorf("raw value mismatch: %q", byField["DB_TOKEN"].Value)
	}
	if byField["DB_TOKEN"].ValueHash != HashValue("super-secret-token-1234567890") {
		t.Error("value hash mismatch")
	}
	// Quoted value must be unquoted.
	if byField["QUOTED"].Value != "quoted-value-abcdefgh" {
		t.Errorf("quoted value not unquoted: %q", byField["QUOTED"].Value)
	}
}

func TestScanWorktreeKeyBlob(t *testing.T) {
	root := t.TempDir()
	blob := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----\n"
	if err := os.WriteFile(filepath.Join(root, "id_rsa"), []byte(blob), 0o600); err != nil {
		t.Fatal(err)
	}
	prints, err := ScanWorktree("wt-a", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(prints) != 1 {
		t.Fatalf("want 1 fingerprint, got %d", len(prints))
	}
	if prints[0].Field != "id_rsa" {
		t.Errorf("field %q want id_rsa", prints[0].Field)
	}
	if prints[0].Classification != ClassSecret {
		t.Errorf("classified %q, want secret", prints[0].Classification)
	}
}

func TestScanWorktreeIgnoresDotGit(t *testing.T) {
	root := t.TempDir()
	// A secret-looking file inside .git must be ignored.
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config.env"),
		[]byte("LEAK=should-not-be-scanned-12345678"), 0o600); err != nil {
		t.Fatal(err)
	}
	prints, err := ScanWorktree("wt-a", root)
	if err != nil {
		t.Fatal(err)
	}
	if len(prints) != 0 {
		t.Fatalf(".git must be pruned, got %+v", prints)
	}
}

func TestIndexForSource(t *testing.T) {
	prints := []Fingerprint{
		{Worktree: "wt-a", Field: "A"},
		{Worktree: "wt-a", Field: "B"},
		{Worktree: "wt-b", Field: "C"},
	}
	idx := NewIndex(prints)
	got := idx.ForSource("wt-b")
	if len(got) != 2 {
		t.Fatalf("ForSource(wt-b) want 2 (only wt-a), got %d", len(got))
	}
	for _, p := range got {
		if p.Worktree != "wt-a" {
			t.Errorf("ForSource leaked same-wt entry: %+v", p)
		}
	}
}

func TestScanWorktreeCapsBlobRead(t *testing.T) {
	// A blob file larger than maxScanBytes is fingerprinted on its first
	// maxScanBytes bytes only — the documented scan-path read cap mirroring
	// the watch-path maxReadBytes.
	dir := t.TempDir()
	big := filepath.Join(dir, "big.key")
	content := make([]byte, maxScanBytes+1024)
	for i := range content[:maxScanBytes] {
		content[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(big, content, 0o600); err != nil {
		t.Fatal(err)
	}
	prints, err := ScanWorktree(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(prints) != 1 {
		t.Fatalf("want 1 blob fingerprint, got %d (%+v)", len(prints), prints)
	}
	want := HashValue(string(content[:maxScanBytes]))
	if prints[0].ValueHash != want {
		t.Fatalf("blob hash = %s, want hash of the first %d bytes", prints[0].ValueHash, maxScanBytes)
	}
}

func TestScanWorktreeEnvReadsCappedContent(t *testing.T) {
	// A .env larger than maxScanBytes still yields the KEY=VALUE fingerprints
	// that live within the capped read; entries beyond the cap are not
	// fingerprinted (the documented truncation point).
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")
	var b strings.Builder
	b.WriteString("# padding so the tail lands beyond maxScanBytes\n")
	for b.Len() < maxScanBytes-256 {
		b.WriteString("# filler line to grow the file past the read cap\n")
	}
	b.WriteString("REAL_TOKEN=super-secret-token-1234567890\n")
	b.WriteString(strings.Repeat("# beyond-cap filler\n", 64))
	b.WriteString("LOST_TOKEN=another-secret-token-0987654321\n")
	if err := os.WriteFile(env, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	prints, err := ScanWorktree(dir, dir)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]bool{}
	for _, p := range prints {
		fields[p.Field] = true
	}
	if !fields["REAL_TOKEN"] {
		t.Fatal("REAL_TOKEN within the capped read must be fingerprinted")
	}
	if fields["LOST_TOKEN"] {
		t.Fatal("LOST_TOKEN beyond the read cap must not be fingerprinted")
	}
}
