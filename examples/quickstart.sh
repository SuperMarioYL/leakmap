#!/usr/bin/env bash
# LeakMap quickstart — from cold clone to first leak event in 3 commands.
set -euo pipefail

# 1) build + install the single binary
go install ./cmd/leakmap

# 2) make a repo with two worktrees; wt-a carries a secret .env
repo="$(mktemp -d)/lm" && git init -q "$repo" && cd "$repo"
git config user.email dev@leakmap.dev && git config user.name dev
printf '.env\n' > .gitignore && echo '# demo' > README.md
printf 'DB_TOKEN=super-secret-token-1234567890\n' > .env
git add -A && git commit -qm init && git worktree add -q ../wt-b -b feature/b

# 3) fingerprint, then watch; write wt-a's token into wt-b to trip a leak
leakmap scan --repo "$repo"
( sleep 2; echo 'TOKEN=super-secret-token-1234567890' >> ../wt-b/notes.md ) &
leakmap watch --repo "$repo"
