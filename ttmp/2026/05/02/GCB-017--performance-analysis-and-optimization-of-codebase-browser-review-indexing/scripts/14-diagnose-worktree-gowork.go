// 14-diagnose-worktree-gowork.go
// Reproduces the worktree extraction bug: packages.Load fails inside worktrees
// when a parent go.work file exists. Fix: set GOWORK=off in packages.Config.Env.
//
// Usage: cd codebase-browser && go run scripts/14-diagnose-worktree-gowork.go
//+build ignore

package main

import (
    "fmt"
    "log"
    "os"
    "os/exec"
    "path/filepath"

    "golang.org/x/tools/go/packages"
)

func main() {
    repoRoot := "."
    commitHash := "HEAD~1"

    tmpDir := filepath.Join(repoRoot, ".git-worktrees", "diag-test")
    cmd := exec.Command("git", "worktree", "add", "--detach", tmpDir, commitHash)
    cmd.Dir = repoRoot
    out, err := cmd.CombinedOutput()
    if err != nil {
        log.Fatalf("worktree add failed: %v\n%s", err, out)
    }
    defer exec.Command("git", "worktree", "remove", "--force", tmpDir).Run()

    absRoot, _ := filepath.Abs(tmpDir)

    // --- Without GOWORK=off (broken) ---
    fmt.Println("=== Without GOWORK=off ===")
    cfg1 := &packages.Config{
        Mode:  packages.NeedName | packages.NeedModule,
        Dir:   absRoot,
        Tests: true,
    }
    pkgs1, _ := packages.Load(cfg1, "./cmd/...", "./internal/...")
    for _, p := range pkgs1 {
        fmt.Printf("  PkgPath=%q Name=%q Errors=%d\n", p.PkgPath, p.Name, len(p.Errors))
        for _, e := range p.Errors {
            fmt.Printf("    ERROR: %s\n", e.Msg)
        }
    }

    // --- With GOWORK=off (fixed) ---
    fmt.Println("\n=== With GOWORK=off ===")
    cfg2 := &packages.Config{
        Mode:  packages.NeedName | packages.NeedModule,
        Dir:   absRoot,
        Tests: true,
        Env:   append(os.Environ(), "GOWORK=off"),
    }
    pkgs2, _ := packages.Load(cfg2, "./cmd/...", "./internal/...")
    fmt.Printf("  Loaded %d packages\n", len(pkgs2))
    for i, p := range pkgs2 {
        fmt.Printf("  [%d] PkgPath=%q Name=%q\n", i, p.PkgPath, p.Name)
    }
}
