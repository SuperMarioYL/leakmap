package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/render"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

func newMapCmd() *cobra.Command {
	var htmlOut string
	cmd := &cobra.Command{
		Use:   "map",
		Short: "Render the accumulated leak-map (TUI or local HTML)",
		Long: `Reads the LeakEvent audit trail (leakmap.jsonl) and renders the
leak-map: worktree nodes plus leak edges. With no flags it runs the bubbletea
TUI in the terminal; --html writes a self-contained local HTML page instead.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			events, err := leak.ReadEvents(globalFlags.jsonl)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(os.Stderr, "leakmap: no audit trail found — run `leakmap watch` first")
					events = nil
				} else {
					return err
				}
			}
			// Worktree nodes: try to re-discover for labels; events alone are
			// sufficient if discovery fails.
			var wts []worktree.Worktree
			if d, derr := worktree.Discover(globalFlags.repo); derr == nil {
				wts = d
			}

			if htmlOut != "" {
				if err := render.HTML(wts, events, htmlOut); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "leakmap: wrote %s (%d event(s))\n", htmlOut, len(events))
				return nil
			}
			return render.RunTUI(wts, events)
		},
	}
	cmd.Flags().StringVar(&htmlOut, "html", "", "write a self-contained HTML leak-map to this path instead of running the TUI")
	return cmd
}
