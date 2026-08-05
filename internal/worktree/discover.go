// Package worktree discovers git worktrees of a repository and associates
// running agent processes to them by matching each process's current working
// directory against the worktree paths.
package worktree

import (
	"bytes"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Worktree is one entry from `git worktree list`.
type Worktree struct {
	// Path is the absolute filesystem root of the worktree.
	Path string
	// Head is the commit SHA the worktree is checked out at (may be empty for
	// bare/locked entries).
	Head string
	// Branch is the ref name the worktree is on, if any.
	Branch string
	// Bare is true for the main worktree of a bare repo.
	Bare bool
}

// Discover runs `git worktree list --porcelain` inside repoRoot and parses the
// result. repoRoot must be a directory inside a git repository. If git is not
// installed or the repo has no worktrees beyond the implicit main one, the
// main worktree is still returned.
func Discover(repoRoot string) ([]Worktree, error) {
	out, err := runGit(repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	wts, err := ParsePorcelain(out)
	if err != nil {
		return nil, err
	}
	if len(wts) == 0 {
		// Fall back to the common main worktree path via rev-parse.
		top, terr := runGit(repoRoot, "rev-parse", "--show-toplevel")
		if terr != nil {
			return nil, fmt.Errorf("git rev-parse --show-toplevel: %w", terr)
		}
		top = strings.TrimSpace(top)
		if top == "" {
			return nil, fmt.Errorf("no worktrees and no toplevel")
		}
		wts = []Worktree{{Path: top}}
	}
	// Stable order by path.
	sort.Slice(wts, func(i, j int) bool { return wts[i].Path < wts[j].Path })
	return wts, nil
}

// ParsePorcelain parses the textual output of `git worktree list --porcelain`
// without invoking git. Exposed for testing.
//
// Format (one block per worktree):
//
//	worktree /abs/path
//	HEAD <sha>
//	branch refs/heads/<name>     (optional)
//	detached                    (optional)
//	bare                        (optional)
//	<blank line ends block>
func ParsePorcelain(s string) ([]Worktree, error) {
	var wts []Worktree
	lines := strings.Split(s, "\n")
	var cur Worktree
	flush := func() {
		if cur.Path != "" {
			wts = append(wts, cur)
		}
		cur = Worktree{}
	}
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			flush()
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		case strings.HasPrefix(line, "branch "):
			cur.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch "))
		case line == "bare":
			cur.Bare = true
		default:
			// detached, locked, etc. — ignored for v0.1.
		}
	}
	flush()
	return wts, nil
}

// runGit executes a git command inside dir and returns combined stdout+stderr.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
