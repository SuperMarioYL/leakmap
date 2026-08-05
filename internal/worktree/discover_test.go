package worktree

import "testing"

func TestParsePorcelain(t *testing.T) {
	in := `worktree /repo
HEAD 0000000000000000000000000000000000000000
branch refs/heads/main

worktree /repo-wt-b
HEAD 1111111111111111111111111111111111111111
branch refs/heads/feature/b

worktree /repo-wt-c
HEAD 2222222222222222222222222222222222222222
detached

`
	wts, err := ParsePorcelain(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 3 {
		t.Fatalf("want 3 worktrees, got %d", len(wts))
	}
	wantPaths := []string{"/repo", "/repo-wt-b", "/repo-wt-c"}
	wantBranches := []string{"refs/heads/main", "refs/heads/feature/b", ""}
	for i, w := range wts {
		if w.Path != wantPaths[i] {
			t.Errorf("wt %d path %q want %q", i, w.Path, wantPaths[i])
		}
		if w.Branch != wantBranches[i] {
			t.Errorf("wt %d branch %q want %q", i, w.Branch, wantBranches[i])
		}
	}
	if wts[0].Head != "0000000000000000000000000000000000000000" {
		t.Errorf("wt 0 head %q", wts[0].Head)
	}
}

func TestParsePorcelainTrailingNoBlank(t *testing.T) {
	// A final block with no trailing blank line must still be flushed.
	in := "worktree /repo\nHEAD abc\nbranch refs/heads/main"
	wts, err := ParsePorcelain(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(wts) != 1 || wts[0].Path != "/repo" {
		t.Fatalf("unexpected %+v", wts)
	}
}

func TestParsePorcelainBare(t *testing.T) {
	in := "worktree /repo\nbare\n"
	wts, _ := ParsePorcelain(in)
	if len(wts) != 1 || !wts[0].Bare {
		t.Fatalf("bare flag not set: %+v", wts)
	}
}
