package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestWatchCmdRegistersTUIFlag(t *testing.T) {
	cmd := newWatchCmd()
	f := cmd.Flags().Lookup("tui")
	if f == nil {
		t.Fatal("watch command does not register the --tui flag")
	}
}

// TestWatchCmdInterruptExitsCleanly is an end-to-end smoke: watch discovers a
// real two-worktree repo, opens its watchers, and must exit cleanly (nil
// error, JSONL audit trail created) when interrupted — the same path the
// wired --tui flag rides on after the session ends.
func TestWatchCmdInterruptExitsCleanly(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end smoke")
	}

	tmp := t.TempDir()
	repo := filepath.Join(tmp, "lm")
	gitRunIn := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v (in %s): %v\n%s", args, dir, err, out)
		}
	}

	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRunIn(repo, "init", "-q")
	gitRunIn(repo, "config", "user.email", "dev@leakmap.dev")
	gitRunIn(repo, "config", "user.name", "dev")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("# demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRunIn(repo, "add", "-A")
	gitRunIn(repo, "commit", "-qm", "init")
	gitRunIn(repo, "worktree", "add", "-q", filepath.Join(tmp, "wt-b"), "-b", "feature/b")

	// The repo carries a secret so the watch session has a fingerprint.
	if err := os.WriteFile(filepath.Join(repo, ".env"),
		[]byte("DB_TOKEN=super-secret-token-1234567890\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	jsonl := filepath.Join(tmp, "leakmap.jsonl")
	oldRepo, oldJSONL := globalFlags.repo, globalFlags.jsonl
	globalFlags.repo, globalFlags.jsonl = repo, jsonl
	t.Cleanup(func() { globalFlags.repo, globalFlags.jsonl = oldRepo, oldJSONL })

	cmd := newWatchCmd()
	go func() {
		// Give RunE time to reach the watch loop, then interrupt the session
		// the way a user would (signal.NotifyContext intercepts it).
		time.Sleep(750 * time.Millisecond)
		_ = syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("watch did not exit cleanly on interrupt: %v", err)
	}
	if _, err := os.Stat(jsonl); err != nil {
		t.Fatalf("JSONL audit trail not created: %v", err)
	}
}
