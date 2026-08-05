// Package proc maps running processes to git worktrees by current working
// directory. It is best-effort: on systems where reading another process's
// cwd requires elevated privileges, the association is simply skipped and
// the agent PID for that worktree is reported as 0 (unknown).
package proc

import (
	"strings"

	"github.com/shirou/gopsutil/v3/process"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

// Mapping associates one worktree with the first process whose cwd is inside
// it. AgentPID is 0 when no process could be matched.
type Mapping struct {
	Worktree worktree.Worktree
	AgentPID int
}

// MapWorktrees inspects all running processes and, for each worktree, records
// the first process whose cwd lives under that worktree's path. Processes
// whose cwd cannot be read are skipped. Worktrees with no matching process
// get AgentPID=0.
func MapWorktrees(wts []worktree.Worktree) []Mapping {
	out := make([]Mapping, len(wts))
	for i, wt := range wts {
		out[i] = Mapping{Worktree: wt, AgentPID: 0}
	}

	procs, err := process.Processes()
	if err != nil {
		// No process listing available — return zeroed mappings.
		return out
	}

	for _, p := range procs {
		cwd, err := p.Cwd()
		if err != nil || cwd == "" {
			continue
		}
		// Prefer the most specific (longest-path) matching worktree.
		bestIdx, bestLen := -1, -1
		for i, wt := range wts {
			if isUnder(cwd, wt.Path) && len(wt.Path) > bestLen {
				bestIdx, bestLen = i, len(wt.Path)
			}
		}
		if bestIdx >= 0 && out[bestIdx].AgentPID == 0 {
			out[bestIdx].AgentPID = int(p.Pid)
		}
	}
	return out
}

// isUnder reports whether path is equal to or nested under base.
func isUnder(path, base string) bool {
	if path == base {
		return true
	}
	return strings.HasPrefix(path, base+string("/"))
}
