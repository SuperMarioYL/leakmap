package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/render"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

func newReportCmd() *cobra.Command {
	var mdOut string
	var htmlOut string
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Write a Markdown (and optional HTML) leak-map summary",
		Long: `Reads the LeakEvent audit trail (leakmap.jsonl) and writes a
deterministic Markdown summary ranking secret-severity leaks first. With
--html, also writes a self-contained HTML leak-map page. The optional GLM-4
prose seam is inert until configured (see render.Report).`,
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
			var wts []worktree.Worktree
			if d, derr := worktree.Discover(globalFlags.repo); derr == nil {
				wts = d
			}

			var sink *os.File
			if mdOut == "" || mdOut == "-" {
				sink = os.Stdout
			} else {
				f, ferr := os.Create(mdOut)
				if ferr != nil {
					return ferr
				}
				defer f.Close()
				sink = f
			}
			if err := render.Report(wts, events, sink, render.ProseConfig{}); err != nil {
				return err
			}
			if mdOut != "" && mdOut != "-" {
				fmt.Fprintf(os.Stderr, "leakmap: wrote %s (%d event(s))\n", mdOut, len(events))
			}
			if htmlOut != "" {
				if err := render.HTML(wts, events, htmlOut); err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "leakmap: wrote %s (%d event(s))\n", htmlOut, len(events))
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&mdOut, "markdown", "m", "", "write Markdown to this path (default: stdout)")
	cmd.Flags().StringVar(&htmlOut, "html", "", "also write a self-contained HTML leak-map to this path")
	return cmd
}
