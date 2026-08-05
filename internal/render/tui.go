// Package render renders accumulated LeakEvents into the leak-map surfaces:
// a bubbletea TUI (tui.go), a local HTML page (html.go), and a Markdown
// summary (report.go). For v0.1 the TUI is real and minimal (worktree nodes
// + leak edges); the report prose is a deterministic template (the GLM-4
// model prose seam is left as a documented stub — see report.go).
package render

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/SuperMarioYL/leakmap/internal/leak"
	"github.com/SuperMarioYL/leakmap/internal/worktree"
)

// Palette — leak-map: dev-tooling family (blue + slate) per the house style,
// with a red accent for leak edges so attribution pops on a terminal.
var (
	nodeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#0EA5E9")).
			Padding(0, 1)
	edgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F43F5E"))
	sevStyle = lipgloss.NewStyle().Bold(true)
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#38BDF8"))
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
)

// Model is the bubbletea leak-map model. It holds the worktree nodes and the
// leak edges derived from a set of LeakEvents.
type Model struct {
	worktrees []worktree.Worktree
	events    []leak.Event
	width     int
	height    int
}

// NewModel builds a leak-map model from discovered worktrees and recorded
// events. The TUI renders whatever slice it is given — empty is fine.
func NewModel(wts []worktree.Worktree, events []leak.Event) Model {
	return Model{worktrees: wts, events: events}
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update satisfies tea.Model. It reacts to window size and quits on q/Ctrl+C.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View satisfies tea.Model. It draws the leak-map: nodes (worktrees) on the
// left, edges (leaks) on the right.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("LeakMap — leak-map attribution view"))
	b.WriteString("\n\n")

	if len(m.events) == 0 {
		b.WriteString(dimStyle.Render("no cross-worktree leaks recorded yet — run `leakmap watch` in a repo with 2+ worktrees"))
		b.WriteString("\n")
		return b.String()
	}

	// Node column: one line per worktree.
	b.WriteString(nodeStyle.Render("worktrees"))
	b.WriteString("\n")
	known := map[string]bool{}
	for _, wt := range m.worktrees {
		label := shortPath(wt.Path)
		b.WriteString(fmt.Sprintf("  • %s\n", label))
		known[wt.Path] = true
	}
	for _, e := range m.events {
		if !known[e.SourceWorktree] {
			b.WriteString(fmt.Sprintf("  • %s\n", shortPath(e.SourceWorktree)))
			known[e.SourceWorktree] = true
		}
		if !known[e.TargetWorktree] {
			b.WriteString(fmt.Sprintf("  • %s\n", shortPath(e.TargetWorktree)))
			known[e.TargetWorktree] = true
		}
	}

	// Edge column: source -> target per leak.
	b.WriteString("\n")
	b.WriteString(edgeStyle.Render("leak edges"))
	b.WriteString("\n")
	for _, e := range m.events {
		src := shortPath(e.SourceWorktree)
		tgt := shortPath(e.TargetWorktree)
		line := fmt.Sprintf("  %s ──%s──▶ %s   %s=%s",
			src, e.MatchKind, tgt,
			e.SecretField, e.Severity)
		b.WriteString(edgeStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d leak(s) · press q to quit", len(m.events))))
	b.WriteString("\n")
	return b.String()
}

// shortPath reduces an absolute path to its last two segments for compact
// node labels.
func shortPath(p string) string {
	if p == "" {
		return "?"
	}
	parts := strings.Split(strings.TrimRight(p, "/"), "/")
	if len(parts) <= 2 {
		return strings.Join(parts, "/")
	}
	return strings.Join(parts[len(parts)-2:], "/")
}

// RunTUI launches the bubbletea program for the given worktrees + events.
// It blocks until the user quits.
func RunTUI(wts []worktree.Worktree, events []leak.Event) error {
	p := tea.NewProgram(NewModel(wts, events), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
