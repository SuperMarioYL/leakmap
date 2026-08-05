// Package secret fingerprints the secret-bearing material of a worktree.
//
// A Fingerprint is {worktree, path, field, value_hash, classification}. The
// raw value is kept in-memory only for cross-boundary matching during a
// `watch` session; it is never persisted to disk — the value_hash is the
// auditable identifier. This package does regex-based classification first;
// ambiguous values can be promoted by the optional model classifier
// (ClassifyWithModel), which is a no-op stub when no model endpoint is
// configured (see classify.go).
package secret

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Fingerprint is one fingerprinted secret surface entry. Value is the raw
// secret material kept in-memory only for matching; ValueHash is its
// truncated SHA-256 hex digest, safe to persist and display.
type Fingerprint struct {
	Worktree       string `json:"worktree"`
	Path           string `json:"path"`
	Field          string `json:"field"`
	ValueHash      string `json:"value_hash"`
	Classification string `json:"classification"`
	// Value is the raw material, NEVER serialized to disk. Kept volatile for
	// the in-memory match index during `watch`. Omitted from JSON on purpose.
	Value string `json:"-"`
}

// HashValue returns a truncated SHA-256 hex digest of value. Truncation to 16
// hex chars (8 bytes) is sufficient as a display fingerprint while avoiding a
// full digest that could be rainbow-tabled against short secrets.
func HashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

// Mask returns the value masked for display, e.g. "sk-abcd1234…".
func Mask(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	if len(value) <= 12 {
		return value[:2] + strings.Repeat("*", len(value)-2)
	}
	return value[:6] + "…" + value[len(value)-4:]
}

// scanFiles is the set of glob patterns describing secret-bearing files. Keys
// are relative to the worktree root.
var scanFiles = []string{
	".env",
	"*.env",
	".env.*",
	"*.key",
	"*.pem",
	"*.p12",
	"*.pfx",
	"id_rsa",
	"id_ed25519",
	"id_ecdsa",
	"credentials",
	"credentials.json",
	"*.credentials",
}

// isIgnoredDir reports whether a directory should be pruned during the walk.
// We never fingerprint inside .git, node_modules, or heavy build dirs.
func isIgnoredDir(name string) bool {
	switch name {
	case ".git", "node_modules", ".next", "dist", "build", "target", ".venv", "venv":
		return true
	}
	return false
}

// ScanWorktree walks wtRoot for secret-bearing files and returns one
// fingerprint per discovered KEY=VALUE (for .env-style files) or one per file
// (for key/credential blobs). Worktree labels each fingerprint with wtLabel
// (the worktree path). Errors on individual files are skipped, not fatal.
func ScanWorktree(wtLabel, wtRoot string) ([]Fingerprint, error) {
	var prints []Fingerprint
	err := filepath.Walk(wtRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if info.IsDir() {
			if isIgnoredDir(info.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !shouldScan(info.Name()) {
			return nil
		}
		fps, err := fingerprintFile(wtLabel, path)
		if err != nil {
			return nil // skip unreadable files
		}
		prints = append(prints, fps...)
		return nil
	})
	if err != nil {
		return prints, err
	}
	return prints, nil
}

// shouldScan reports whether a file name matches the scan globs.
func shouldScan(name string) bool {
	for _, pat := range scanFiles {
		ok, err := filepath.Match(pat, name)
		if err == nil && ok {
			return true
		}
	}
	return false
}

// fingerprintFile reads one file and produces fingerprints. For .env-style
// files, each KEY=VALUE line is a fingerprint (field=KEY). For blob files
// (keys, credentials), the whole content is one fingerprint (field=filename).
func fingerprintFile(wtLabel, path string) ([]Fingerprint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)
	base := filepath.Base(path)

	// .env-style: parse KEY=VALUE lines.
	if isEnvFile(base) {
		return parseEnv(wtLabel, path, content), nil
	}
	// Blob files: whole content is the value.
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, nil
	}
	return []Fingerprint{{
		Worktree:       wtLabel,
		Path:           path,
		Field:          base,
		ValueHash:      HashValue(trimmed),
		Classification: Classify(trimmed, base),
		Value:          trimmed,
	}}, nil
}

func isEnvFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == ".env" ||
		strings.HasSuffix(lower, ".env") ||
		strings.HasPrefix(lower, ".env.")
}

// parseEnv parses KEY=VALUE lines, ignoring comments and blanks. Values may
// be single- or double-quoted; surrounding quotes are stripped.
func parseEnv(wtLabel, path, content string) []Fingerprint {
	var prints []Fingerprint
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		val = unquote(val)
		if val == "" {
			continue
		}
		prints = append(prints, Fingerprint{
			Worktree:       wtLabel,
			Path:           path,
			Field:          key,
			ValueHash:      HashValue(val),
			Classification: Classify(val, key),
			Value:          val,
		})
	}
	return prints
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// Index groups fingerprints by worktree for fast cross-boundary lookup.
type Index struct {
	byWorktree map[string][]Fingerprint
	all        []Fingerprint
}

// NewIndex builds an in-memory index over fingerprints.
func NewIndex(prints []Fingerprint) *Index {
	idx := &Index{byWorktree: map[string][]Fingerprint{}}
	for _, p := range prints {
		idx.byWorktree[p.Worktree] = append(idx.byWorktree[p.Worktree], p)
		idx.all = append(idx.all, p)
	}
	return idx
}

// ForSource returns fingerprints belonging to worktrees other than target.
func (i *Index) ForSource(target string) []Fingerprint {
	var out []Fingerprint
	for wt, ps := range i.byWorktree {
		if wt == target {
			continue
		}
		out = append(out, ps...)
	}
	return out
}

// All returns every fingerprint.
func (i *Index) All() []Fingerprint { return i.all }

// String formats a fingerprint for the `scan` inventory display.
func (p Fingerprint) String() string {
	return fmt.Sprintf("%-22s %-10s %s", p.Field, p.Classification, p.ValueHash)
}
