package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/SuperMarioYL/leakmap/internal/proc"
	"github.com/SuperMarioYL/leakmap/internal/secret"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

func newScanCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Discover worktrees and fingerprint each worktree's secret surface",
		Long: `Runs "git worktree list", fingerprints the secret-bearing files
(.env, *.key, credentials, ...) of every worktree, classifies each value
(regex-first; the optional model classifier is inert until configured), and
prints a per-worktree secret inventory. Fingerprints are kept in-memory; the
raw secret values are never persisted — only their hashes and classifications.`,
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
			mappings := proc.MapWorktrees(wts)
			pidByWt := map[string]int{}
			for _, m := range mappings {
				pidByWt[m.Worktree.Path] = m.AgentPID
			}

			var allPrints []secret.Fingerprint
			for _, m := range mappings {
				wt := m.Worktree
				prints, err := secret.ScanWorktree(wt.Path, wt.Path)
				if err != nil && globalFlags.verbose {
					fmt.Fprintln(os.Stderr, "warn:", wt.Path, err)
				}
				allPrints = append(allPrints, prints...)
				if !jsonOut {
					printWorktreeHeader(wt, m.AgentPID, len(prints))
					printInventory(prints)
				}
			}
			if jsonOut {
				printJSONInventory(allPrints)
			}
			if !jsonOut {
				fmt.Printf("\n%d worktree(s), %d fingerprint(s)\n", len(mappings), len(allPrints))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the inventory as JSON")
	return cmd
}

func printWorktreeHeader(wt worktree.Worktree, pid int, n int) {
	fmt.Printf("\n== %s\n", rel(wt.Path))
	if pid != 0 {
		fmt.Printf("   agent pid: %d\n", pid)
	}
	fmt.Printf("   %d secret surface entr(y|ies)\n", n)
}

func printInventory(prints []secret.Fingerprint) {
	if len(prints) == 0 {
		fmt.Println("   (none)")
		return
	}
	for _, p := range prints {
		fmt.Printf("   %-22s %-10s %s  %s\n",
			p.Field, p.Classification, p.ValueHash, secret.Mask(p.Value))
	}
}

func printJSONInventory(prints []secret.Fingerprint) {
	// Redact raw values in JSON output: only path/field/hash/classification.
	type safeFP struct {
		Path           string `json:"path"`
		Field          string `json:"field"`
		ValueHash      string `json:"value_hash"`
		Classification string `json:"classification"`
	}
	out := make([]safeFP, len(prints))
	for i, p := range prints {
		out[i] = safeFP{
			Path:           p.Path,
			Field:          p.Field,
			ValueHash:      p.ValueHash,
			Classification: p.Classification,
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(b))
}

// rel shortens an absolute path to something readable from cwd.
func rel(p string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return p
	}
	r, err := filepath.Rel(cwd, p)
	if err != nil {
		return p
	}
	if !strings.HasPrefix(r, ".") && !strings.HasPrefix(r, "/") {
		r = "./" + r
	}
	return r
}
