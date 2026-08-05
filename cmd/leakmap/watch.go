package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/proc"
	"github.com/SuperMarioYL/leakmap/internal/secret"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

func newWatchCmd() *cobra.Command {
	var showTUI bool
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch cross-worktree writes and emit attributed LeakEvents",
		Long: `Discovers the repository's worktrees, fingerprints each worktree's
secret surface (in-memory), and opens an fsnotify watch over every worktree
root. The moment a write to worktree Y contains a value fingerprinted in a
*different* worktree X, a LeakEvent is emitted to the JSONL audit trail and
printed to the terminal. The raw secret value is never persisted — only field
names, paths, and hashes.

Ctrl-C stops the watch. See "leakmap map" to render the accumulated leaks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := globalFlags.repo
			abs, err := filepath.Abs(repo)
			if err != nil {
				return fmt.Errorf("resolve repo path: %w", err)
			}
			wts, err := worktree.Discover(abs)
			if err != nil {
				return err
			}
			if len(wts) < 1 {
				return fmt.Errorf("no worktrees discovered under %s", abs)
			}

			mappings := proc.MapWorktrees(wts)
			pidByWt := map[string]int{}
			for _, m := range mappings {
				pidByWt[m.Worktree.Path] = m.AgentPID
			}

			// Fingerprint every worktree (in-memory; raw values kept volatile).
			var allPrints []secret.Fingerprint
			for _, m := range mappings {
				prints, ferr := secret.ScanWorktree(m.Worktree.Path, m.Worktree.Path)
				if ferr != nil && globalFlags.verbose {
					fmt.Fprintln(os.Stderr, "warn:", m.Worktree.Path, ferr)
				}
				allPrints = append(allPrints, prints...)
			}
			idx := secret.NewIndex(allPrints)

			fmt.Fprintf(os.Stderr, "leakmap: %d worktree(s), %d fingerprint(s) loaded\n",
				len(mappings), len(allPrints))
			for _, m := range mappings {
				fmt.Fprintf(os.Stderr, "  watch %s", rel(m.Worktree.Path))
				if m.AgentPID != 0 {
					fmt.Fprintf(os.Stderr, " (agent pid %d)", m.AgentPID)
				}
				fmt.Fprintln(os.Stderr)
			}

			writer, err := leak.NewJSONLWriter(globalFlags.jsonl)
			if err != nil {
				return err
			}
			defer writer.Close()

			store := leak.NewStore()
			emit := func(e leak.Event) {
				store.Append(e)
				_ = writer.Write(e)
				fmt.Fprintln(os.Stderr, "  leak:", e.Summary())
			}

			roots := make([]string, 0, len(mappings))
			for _, m := range mappings {
				roots = append(roots, m.Worktree.Path)
			}
			det := leak.NewDetector(idx, pidByWt, emit)

			ctx, stop := signal.NotifyContext(context.Background(),
				os.Interrupt, syscall.SIGTERM)
			defer stop()

			onErr := func(err error) {
				if globalFlags.verbose {
					fmt.Fprintln(os.Stderr, "  watch err:", err)
				}
			}
			fmt.Fprintln(os.Stderr, "leakmap: watching — Ctrl-C to stop")
			return det.Watch(roots, ctx.Done(), onErr)
		},
	}
	cmd.Flags().BoolVar(&showTUI, "tui", false,
		"run the leak-map TUI inline after the watch session (m3 surface)")
	return cmd
}
