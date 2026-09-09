# Changelog

All notable changes to LeakMap are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions match the
repo-root VERSION file and the `v<version>` release tags.

## [0.2.0] — 2026-09-09

Correctness and attribution release; no new scope.

### Fixed

- `leakmap watch --tui` was a dead flag: declared and documented but never
  read, so the promised post-session leak-map TUI never ran. It now renders
  the leak-map accumulated during the watch session inline when the session
  ends (Ctrl-C). `leakmap scan` no longer builds an unused worktree-to-PID
  map.
- `LeakEvent.source_agent_pid` was always 0 even when the source worktree's
  agent PID was known. Events now carry the source agent PID (best-effort,
  0 when unknown), completing the source → target attribution the data
  model promises.
- Large writes stalled the watch loop: the sliding-window fuzzy matcher
  burned O(content × value) CPU per fingerprint (a single 512 KiB write
  measured ~600 ms against one fingerprint), and files were read into memory
  unbounded. Reads are now capped at 10 MiB and the fuzzy scan at 128 KiB of
  content; exact substring matching still covers the full capped content, so
  verbatim leaks in large files keep firing. Known tradeoff: a lightly
  mutated secret appearing only beyond 128 KiB of a larger write is no
  longer fuzzy-detected.

### Changed

- Version surfaces bumped to 0.2.0 in lockstep (VERSION file, CLI `--version`)
  with a unit test asserting they can never drift apart.

Network-egress (eBPF) and env-var read interception remain planned; they are
not part of this release.

## [0.1.0] — 2026-08-06

Initial release.

### Added

- `leakmap scan` — discover git worktrees and fingerprint each worktree's
  secret surface (`.env`-style KEY=VALUE entries, key/credential blobs) with
  regex-first classification; in-memory fingerprints are never persisted,
  only truncated hashes.
- `leakmap watch` — fsnotify watch over every worktree root; the moment a
  write to one worktree contains a value fingerprinted in a different
  worktree, an attributed LeakEvent is emitted to the JSONL audit trail.
- `leakmap map` — render the accumulated leak-map as a bubbletea TUI or a
  self-contained local HTML page.
- `leakmap report` — deterministic Markdown summary ranked by severity, with
  optional HTML export.
