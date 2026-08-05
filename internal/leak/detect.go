// Package leak — detection. The Detector watches a set of worktree roots
// with fsnotify and, on each write, matches the new content against the
// in-memory secret index of the *other* worktrees. A cross-boundary match
// emits a LeakEvent (MatchKind exact or fuzzy).
//
// Detection is single-process and in-memory for v0.1; the JSONL writer makes
// the audit trail durable. Network/env eBPF detection is explicitly out of
// scope for v0.1 (see the plan).
package leak

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/SuperMarioYL/leakmap/internal/secret"
)

// minMatchLen is the shortest value length worth matching as a substring.
// Very short values (e.g. "true", a 4-char flag) produce false positives.
const minMatchLen = 8

// Detector holds the watch roots, the secret index, and dispatches events.
type Detector struct {
	mu       sync.Mutex
	index    *secret.Index
	pidByWt  map[string]int // worktree path -> agent pid (best-effort)
	roots    []string       // watched worktree roots
	emit     func(Event)
	minDelay time.Duration
}

// NewDetector builds a detector that calls emit for each LeakEvent. pidByWt
// maps worktree paths to agent PIDs (0 when unknown).
func NewDetector(idx *secret.Index, pidByWt map[string]int, emit func(Event)) *Detector {
	if pidByWt == nil {
		pidByWt = map[string]int{}
	}
	if emit == nil {
		emit = func(Event) {}
	}
	return &Detector{index: idx, pidByWt: pidByWt, emit: emit, minDelay: 5 * time.Millisecond}
}

// Match inspects written content and returns LeakEvents for any fingerprint
// sourced from a *different* worktree whose value appears in the content.
// targetWt is the worktree receiving the write; targetPath the file written;
// targetPID the agent owning that worktree (0 if unknown).
//
// Match is pure and has no side effects — it is the unit of attribution logic
// and is tested directly.
func Match(content string, targetWt, targetPath string, targetPID int, idx *secret.Index) []Event {
	var events []Event
	if idx == nil || len(content) == 0 {
		return events
	}
	now := time.Now().UTC()
	for _, p := range idx.ForSource(targetWt) {
		if len(p.Value) < minMatchLen {
			continue
		}
		switch {
		case strings.Contains(content, p.Value):
			events = append(events, Event{
				Timestamp:      now,
				SourceWorktree: p.Worktree,
				SourceAgentPID: 0,
				SourcePath:     p.Path,
				SecretField:    p.Field,
				TargetWorktree: targetWt,
				TargetAgentPID: targetPID,
				TargetPath:     targetPath,
				MatchKind:      MatchExact,
				Severity:       severityOfClass(p.Classification),
			})
		case fuzzyContains(content, p.Value):
			events = append(events, Event{
				Timestamp:      now,
				SourceWorktree: p.Worktree,
				SourceAgentPID: 0,
				SourcePath:     p.Path,
				SecretField:    p.Field,
				TargetWorktree: targetWt,
				TargetAgentPID: targetPID,
				TargetPath:     targetPath,
				MatchKind:      MatchFuzzy,
				Severity:       severityOfClass(p.Classification),
			})
		}
	}
	return events
}

// fuzzyContains reports whether content contains a near-duplicate of value
// (>= 0.9 normalized similarity against any equal-length run). This catches
// a value that was lightly mutated (e.g. trailing newline removed, one char
// changed) without matching benign short tokens.
func fuzzyContains(content, value string) bool {
	vlen := len(value)
	if vlen < minMatchLen {
		return false
	}
	// Slide a window of len(value) across content; cheap Jaccard on byte
	// trigrams. Keep it O(n) per value: cap the scan.
	maxScan := len(content) - vlen
	if maxScan < 0 {
		// content shorter than value: compare whole content to value.
		return similarity(content, value) >= 0.9
	}
	trigramsV := trigrams(value)
	for i := 0; i <= maxScan; i++ {
		window := content[i : i+vlen]
		if similarityTrigram(window, value, trigramsV) >= 0.9 {
			return true
		}
	}
	return false
}

func trigrams(s string) map[string]struct{} {
	m := make(map[string]struct{}, len(s))
	for i := 0; i+3 <= len(s); i++ {
		m[s[i:i+3]] = struct{}{}
	}
	return m
}

func similarityTrigram(a, b string, bTri map[string]struct{}) float64 {
	aTri := trigrams(a)
	if len(aTri) == 0 && len(bTri) == 0 {
		return 1
	}
	inter := 0
	for t := range aTri {
		if _, ok := bTri[t]; ok {
			inter++
		}
	}
	union := len(aTri) + len(bTri) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// similarity is a small normalized similarity (0..1) used when content is
// shorter than the fingerprinted value.
func similarity(a, b string) float64 {
	if a == b {
		return 1
	}
	// Sorensen-dice on trigrams handles short strings well.
	return similarityTrigram(a, b, trigrams(b))
}

// severityOfClass maps a secret classification onto a LeakEvent severity. An
// unknown classification is conservatively treated as a secret leak so that
// ambiguous material is surfaced rather than silently dropped.
func severityOfClass(class string) string {
	switch class {
	case secret.ClassSecret, secret.ClassUnknown:
		return SeveritySecret
	case secret.ClassConfig:
		return SeverityConfig
	case secret.ClassState:
		return SeverityState
	default:
		return SeveritySecret
	}
}

// Watch starts an fsnotify watcher over the configured roots (recursively,
// pruning .git and heavy build dirs) and dispatches write events through
// Match + emit. It blocks until the watcher is closed or ctx signals done.
// Errors are reported through onErr (non-fatal: a single failed AddWatch
// must not abort the whole session).
func (d *Detector) Watch(roots []string, done <-chan struct{}, onErr func(error)) error {
	d.mu.Lock()
	d.roots = append([]string(nil), roots...)
	d.mu.Unlock()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("fsnotify: %w", err)
	}
	defer w.Close()

	for _, root := range roots {
		if err := addWatchRecursive(w, root); err != nil && onErr != nil {
			onErr(err)
		}
	}

	for {
		select {
		case <-done:
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if !ev.Has(fsnotify.Write) && !ev.Has(fsnotify.Create) {
				continue
			}
			d.handle(ev.Name, onErr)
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			if onErr != nil {
				onErr(err)
			}
		}
	}
}

// handle reads the written file and runs Match against its content.
func (d *Detector) handle(path string, onErr func(error)) {
	// Throttle tiny back-to-back events for the same path.
	time.Sleep(d.minDelay)

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if onErr != nil {
			onErr(err)
		}
		return
	}
	content := string(data)
	targetWt, ok := d.worktreeForPath(path)
	if !ok {
		return // write outside any watched worktree root
	}
	targetPID := d.pidByWt[targetWt]
	events := Match(content, targetWt, path, targetPID, d.index)
	for _, e := range events {
		d.emit(e)
	}
}

// worktreeForPath returns the most specific watched root containing path.
func (d *Detector) worktreeForPath(path string) (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	best, bestLen := "", -1
	for _, root := range d.roots {
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			if len(root) > bestLen {
				best, bestLen = root, len(root)
			}
		}
	}
	return best, best != ""
}

// addWatchRecursive adds root and all its non-ignored subdirectories to w.
func addWatchRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			return nil
		}
		switch info.Name() {
		case ".git", "node_modules", ".next", "dist", "build", "target", ".venv", "venv":
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}
