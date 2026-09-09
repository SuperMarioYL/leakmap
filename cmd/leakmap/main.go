// Package main is the LeakMap CLI entrypoint. LeakMap attributes secret and
// file bleeds between parallel coding agents running in git worktrees of one
// repository, and renders them as a leak-map.
//
// v0.2 implements: scan (fingerprint each worktree's secret surface),
// watch (fsnotify cross-worktree writes -> LeakEvent JSONL, optional inline
// leak-map TUI), map (leak-map TUI), report (markdown/html summary). Network
// and env-var eBPF detection is out of scope for v0.2.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is the single source of truth for the CLI version banner. The
// release tag (v<version>) is driven by the repo-root VERSION file, which
// main_test.go asserts stays in lockstep with this constant.
const version = "0.2.0"

// globalFlags shared across subcommands.
var globalFlags struct {
	repo    string
	jsonl   string
	verbose bool
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "leakmap",
		Short: "Cross-worktree leak provenance + leak-map for parallel coding agents",
		Long: `LeakMap attributes secret and file bleeds between parallel coding agents
running in git worktrees of one repository. It fingerprints each worktree's
secret surface, watches cross-worktree writes with fsnotify, and renders a
leak-map (TUI / HTML / Markdown) attributing which agent's material bled into
which worktree.

Quick start:
  git worktree add ../wt-b branch-b
  leakmap scan   --repo .        # fingerprint every worktree's secrets
  leakmap watch                 # watch cross-worktree writes -> leakmap.jsonl
  leakmap map                   # render the accumulated leak-map TUI

v0.2 covers file/content leak attribution. Network egress and env-var
interception (eBPF) are deferred to a later release.`,
		Version: version,
	}
	root.PersistentFlags().StringVar(&globalFlags.repo, "repo", ".",
		"repository root (a directory inside a git repo; defaults to cwd)")
	root.PersistentFlags().StringVar(&globalFlags.jsonl, "jsonl", "leakmap.jsonl",
		"path to the leak-event JSONL audit trail")
	root.PersistentFlags().BoolVarP(&globalFlags.verbose, "verbose", "v", false,
		"print verbose diagnostics")

	root.AddCommand(newScanCmd())
	root.AddCommand(newWatchCmd())
	root.AddCommand(newMapCmd())
	root.AddCommand(newReportCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
