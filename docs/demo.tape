Output assets/demo.gif

Set FontSize 14
Set Width 1200
Set Height 560
Set Padding 16
Set Theme "Dracula"

# --- scaffold a demo repo with two worktrees (hidden) ---
Hide
Type "rm -rf /tmp/lm /tmp/lm-wt-b && mkdir -p /tmp/lm && cd /tmp/lm && git init -q && git config user.email demo@leakmap.dev && git config user.name demo && printf '.env\n' > .gitignore && echo '# LeakMap demo repo' > README.md && printf 'DB_TOKEN=super-secret-token-1234567890\nAPI_KEY=ak_live_abcdefghijklmnopqrstuv\n' > .env && git add -A && git commit -q -m init && git worktree add -q ../lm-wt-b -b feature/b"
Enter
Sleep 2s
Show

Type "cd /tmp/lm"
Enter
Sleep 300ms

# 1) scan: per-worktree secret fingerprint inventory (m1)
Type "leakmap scan --repo /tmp/lm"
Enter
Sleep 1800ms

# 2) watch: schedule a cross-worktree leak write, then watch in foreground
Type "(sleep 2; echo 'TOKEN=super-secret-token-1234567890' >> /tmp/lm-wt-b/notes.md) &"
Enter
Sleep 300ms
Type "leakmap watch --repo /tmp/lm"
Enter
Sleep 4500ms
Ctrl+C
Sleep 600ms

# 3) report: markdown leak-map summary of the accumulated audit trail
Type "leakmap report --repo /tmp/lm | head -16"
Enter
Sleep 1800ms
