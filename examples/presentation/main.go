package main
import (
 "fmt"
 "os"
 "path/filepath"
 "github.com/SuperMarioYL/leakmap/internal/secret"
 "github.com/SuperMarioYL/leakmap/internal/leak"
)
func main() {
 dir, err := os.MkdirTemp("", "leakmap-demo-"); if err != nil { panic(err) }; defer os.RemoveAll(dir)
 value := "demo-only-token-1234567890"
 if err := os.WriteFile(filepath.Join(dir,".env"), []byte("DB_TOKEN="+value+"\n"), 0600); err != nil { panic(err) }
 prints, err := secret.ScanWorktree("worktree-a",dir); if err != nil { panic(err) }
 index := secret.NewIndex(prints)
 events := leak.Match("TOKEN="+value,"worktree-b","notes.md",0,index)
 fmt.Printf("fingerprints: %d\n",len(prints))
 for _, event := range events { fmt.Printf("%s -> %s: field=%s match=%s severity=%s\n",event.SourceWorktree,event.TargetWorktree,event.SecretField,event.MatchKind,event.Severity) }
 fmt.Printf("same-worktree matches: %d\n",len(leak.Match("TOKEN="+value,"worktree-a","notes.md",0,index)))
}
