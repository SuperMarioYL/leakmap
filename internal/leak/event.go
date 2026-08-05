// Package leak defines the LeakEvent primitive: an attributed, timestamped
// record proving that bytes sourced from one worktree's secret surface
// appeared in a write to a different worktree.
//
// A LeakEvent is the unit of cross-worktree leak provenance. It carries the
// source (which worktree/agent/path/field the byte came from) and the target
// (which worktree/agent/path received it), plus a match kind and severity. It
// never carries the raw secret value itself — only field names and paths — so
// the persisted JSONL audit trail is safe to share.
package leak

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Match kinds describe how a fingerprinted value was matched inside a write.
const (
	// MatchExact means the fingerprinted value appeared verbatim (substring)
	// in the written content.
	MatchExact = "exact"
	// MatchFuzzy means a token in the written content was a near-duplicate of
	// the fingerprinted value (high similarity but not byte-equal).
	MatchFuzzy = "fuzzy"
	// MatchLLMClassified means an ambiguous byte sequence was promoted to a
	// leak by the classification model. Reserved for the optional model path.
	MatchLLMClassified = "llm_classified"
)

// Severities classify the sensitivity of the leaked material.
const (
	// SeveritySecret — credentials, tokens, private keys.
	SeveritySecret = "secret"
	// SeverityConfig — non-secret but cross-boundary config drift.
	SeverityConfig = "config"
	// SeverityState — runtime state (session ids, cache keys).
	SeverityState = "state"
)

// Event is a single attributed cross-worktree leak.
type Event struct {
	Timestamp      time.Time `json:"timestamp"`
	SourceWorktree string    `json:"source_worktree"`
	SourceAgentPID int       `json:"source_agent_pid"`
	SourcePath     string    `json:"source_path"`
	SecretField    string    `json:"secret_field"`
	TargetWorktree string    `json:"target_worktree"`
	TargetAgentPID int       `json:"target_agent_pid"`
	TargetPath     string    `json:"target_path"`
	MatchKind      string    `json:"match_kind"`
	Severity       string    `json:"severity"`
}

// Summary returns a one-line human description of the event.
func (e Event) Summary() string {
	return fmt.Sprintf(
		"[%s] %s:%s -> %s:%s  field=%s  kind=%s  severity=%s",
		e.Timestamp.Format(time.RFC3339),
		e.SourceWorktree,
		e.SourcePath,
		e.TargetWorktree,
		e.TargetPath,
		e.SecretField,
		e.MatchKind,
		e.Severity,
	)
}

// JSONLWriter appends LeakEvents as newline-delimited JSON to a file.
type JSONLWriter struct {
	mu   sync.Mutex
	f    *os.File
	path string
}

// NewJSONLWriter opens (creating if needed) a JSONL file for appending.
func NewJSONLWriter(path string) (*JSONLWriter, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open jsonl %s: %w", path, err)
	}
	return &JSONLWriter{f: f, path: path}, nil
}

// Write encodes one event and appends it.
func (w *JSONLWriter) Write(e Event) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.f.Write(b)
	return err
}

// Close flushes and closes the underlying file.
func (w *JSONLWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil {
		err := w.f.Close()
		w.f = nil
		return err
	}
	return nil
}

// Path returns the JSONL file path.
func (w *JSONLWriter) Path() string { return w.path }

// ReadEvents reads all LeakEvents from a JSONL file produced by JSONLWriter.
func ReadEvents(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var events []Event
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			return events, fmt.Errorf("decode line: %w", err)
		}
		events = append(events, e)
	}
	return events, sc.Err()
}

// Store is an in-memory collection of LeakEvents with thread-safe append/read.
type Store struct {
	mu     sync.Mutex
	events []Event
}

// NewStore returns an empty event store.
func NewStore() *Store { return &Store{} }

// Append records an event.
func (s *Store) Append(e Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, e)
}

// Events returns a snapshot copy of all recorded events.
func (s *Store) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out
}

// Len returns the number of stored events.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.events)
}
