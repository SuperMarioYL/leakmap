// Package render — HTML. Renders the leak-map to a self-contained local HTML
// page (no external assets) the operator can open in a browser. This is the
// local report surface from m3; it reads only from LeakEvents and never
// leaks raw secret values.
package render

import (
	"fmt"
	"html/template"
	"os"

	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

const htmlTpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>LeakMap — leak-map report</title>
<style>
  :root { color-scheme: light dark; }
  body { font: 14px/1.5 -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif;
         margin: 0; padding: 2rem; background: #0b1220; color: #e2e8f0; }
  h1 { color: #38bdf8; margin: 0 0 .25rem; }
  .sub { color: #64748b; margin-bottom: 1.5rem; }
  .nodes { display: flex; flex-wrap: wrap; gap: .5rem; margin-bottom: 1.5rem; }
  .node { border: 1px solid #1e293b; background: #0f172a; border-radius: 8px;
          padding: .4rem .8rem; }
  .node b { color: #0ea5e9; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #1e293b; padding: .4rem .6rem; text-align: left;
           font-size: 13px; }
  th { background: #111827; color: #94a3b8; }
  .sev-secret { color: #f43f5e; font-weight: 600; }
  .sev-config { color: #f59e0b; }
  .sev-state  { color: #38bdf8; }
  .edge { color: #f43f5e; }
  .ok { color: #22c55e; }
</style>
</head>
<body>
  <h1>LeakMap</h1>
  <div class="sub">cross-worktree leak-map report · {{.Count}} leak event(s)</div>
  <div class="nodes">
  {{range .Nodes}}
    <div class="node"><b>{{.Label}}</b></div>
  {{else}}
    <div class="node ok">no leaks</div>
  {{end}}
  </div>
  <table>
    <thead><tr>
      <th>time</th><th>source worktree</th><th>source path</th><th>field</th>
      <th>target worktree</th><th>target path</th><th>match</th><th>severity</th>
    </tr></thead>
    <tbody>
    {{range .Events}}
      <tr>
        <td>{{.Timestamp}}</td>
        <td class="edge">{{.SourceWorktree}}</td>
        <td>{{.SourcePath}}</td>
        <td>{{.SecretField}}</td>
        <td>{{.TargetWorktree}}</td>
        <td>{{.TargetPath}}</td>
        <td>{{.MatchKind}}</td>
        <td class="sev-{{.Severity}}">{{.Severity}}</td>
      </tr>
    {{else}}
      <tr><td colspan="8" class="ok">no cross-worktree leaks recorded</td></tr>
    {{end}}
    </tbody>
  </table>
</body>
</html>`

type htmlNode struct{ Label string }

type htmlData struct {
	Nodes  []htmlNode
	Events []leak.Event
	Count  int
}

// HTML writes a self-contained leak-map HTML page to outPath, derived from
// the given worktrees (node column) and events (edge table).
func HTML(wts []worktree.Worktree, events []leak.Event, outPath string) error {
	seen := map[string]bool{}
	nodes := make([]htmlNode, 0)
	for _, wt := range wts {
		if !seen[wt.Path] {
			nodes = append(nodes, htmlNode{Label: shortPath(wt.Path)})
			seen[wt.Path] = true
		}
	}
	for _, e := range events {
		if !seen[e.SourceWorktree] {
			nodes = append(nodes, htmlNode{Label: shortPath(e.SourceWorktree)})
			seen[e.SourceWorktree] = true
		}
		if !seen[e.TargetWorktree] {
			nodes = append(nodes, htmlNode{Label: shortPath(e.TargetWorktree)})
			seen[e.TargetWorktree] = true
		}
	}
	data := htmlData{Nodes: nodes, Events: events, Count: len(events)}

	tpl, err := template.New("leakmap").Parse(htmlTpl)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", outPath, err)
	}
	defer f.Close()
	if err := tpl.Execute(f, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}
