// Package render — Markdown summary. Renders LeakEvents into a deterministic
// Markdown report. The plan specifies GLM-4-Flash for the summary prose; for
// v0.1 the prose is a deterministic template and the model call is left as a
// documented seam (ProseModel is a no-op stub until an operator supplies a
// key) — this keeps the report command runnable offline.
package render

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

// ProseConfig is the optional GLM-4 summary-prose seam. When Endpoint is
// empty, Report produces a deterministic template (the v0.1 happy path).
type ProseConfig struct {
	Endpoint string
	APIKey   string
	Model    string
}

// Report writes a Markdown leak-map summary to w. It groups events by
// source->target edge and ranks secret-severity leaks first.
func Report(wts []worktree.Worktree, events []leak.Event, w io.Writer, _ ProseConfig) error {
	var b strings.Builder
	b.WriteString("# LeakMap — leak-map report\n\n")
	b.WriteString(fmt.Sprintf("_generated %s_  \n", time.Now().UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("**%d** cross-worktree leak event(s) across **%d** worktree(s).\n\n",
		len(events), countWorktrees(wts, events)))

	if len(events) == 0 {
		b.WriteString("> No cross-worktree leaks recorded. Run `leakmap watch` in a repo with 2+ worktrees to start attribution.\n")
		_, err := io.WriteString(w, b.String())
		return err
	}

	// Rank secret leaks first, then config, then state; stable within group.
	ranked := make([]leak.Event, len(events))
	copy(ranked, events)
	sort.SliceStable(ranked, func(i, j int) bool {
		return sevRank(ranked[i].Severity) < sevRank(ranked[j].Severity)
	})

	b.WriteString("## Leak edges\n\n")
	b.WriteString("| # | source | target | field | match | severity |\n")
	b.WriteString("|---|--------|--------|-------|-------|----------|\n")
	for i, e := range ranked {
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s |\n",
			i+1,
			shortPath(e.SourceWorktree),
			shortPath(e.TargetWorktree),
			e.SecretField,
			e.MatchKind,
			e.Severity,
		))
	}

	b.WriteString("\n## Detail\n\n")
	for i, e := range ranked {
		b.WriteString(fmt.Sprintf("### %d. %s → %s\n\n", i+1,
			shortPath(e.SourceWorktree), shortPath(e.TargetWorktree)))
		b.WriteString(fmt.Sprintf("- **time**: %s\n", e.Timestamp.Format(time.RFC3339)))
		b.WriteString(fmt.Sprintf("- **secret field**: `%s`\n", e.SecretField))
		b.WriteString(fmt.Sprintf("- **source path**: `%s`\n", e.SourcePath))
		b.WriteString(fmt.Sprintf("- **target path**: `%s`\n", e.TargetPath))
		b.WriteString(fmt.Sprintf("- **match kind**: %s\n", e.MatchKind))
		b.WriteString(fmt.Sprintf("- **severity**: %s\n\n", e.Severity))
	}

	// Deterministic prose. (When a model is configured, the future iterate
	// replaces this paragraph with GLM-4 generated summary text.)
	secrets := countBySeverity(ranked, leak.SeveritySecret)
	b.WriteString("## Summary\n\n")
	b.WriteString(fmt.Sprintf("Detected %d cross-worktree leak event(s); %d at secret severity. ",
		len(ranked), secrets))
	switch {
	case secrets > 0:
		b.WriteString("Secret-grade material crossed a worktree boundary — review the source worktrees' secret surfaces and the receiving commits before pushing.\n")
	default:
		b.WriteString("No secret-grade leaks detected; remaining events are config/state drift to review at your discretion.\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func sevRank(s string) int {
	switch s {
	case leak.SeveritySecret:
		return 0
	case leak.SeverityConfig:
		return 1
	case leak.SeverityState:
		return 2
	default:
		return 3
	}
}

func countBySeverity(events []leak.Event, sev string) int {
	n := 0
	for _, e := range events {
		if e.Severity == sev {
			n++
		}
	}
	return n
}

func countWorktrees(wts []worktree.Worktree, events []leak.Event) int {
	seen := map[string]bool{}
	for _, wt := range wts {
		seen[wt.Path] = true
	}
	for _, e := range events {
		seen[e.SourceWorktree] = true
		seen[e.TargetWorktree] = true
	}
	return len(seen)
}
