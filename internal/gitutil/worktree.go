package gitutil

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
)

// CreateWorktree creates a temporary git worktree at the given commit.
// It returns the worktree directory path. The caller must call
// RemoveWorktree when done.
func CreateWorktree(ctx context.Context, repoRoot, commitHash string) (string, error) {
	// Create a deterministic temp directory name based on the commit hash.
	tmpDir := filepath.Join(repoRoot, ".git-worktrees", commitHash)

	cmd := exec.CommandContext(ctx, "git", "worktree", "add", "--detach", tmpDir, commitHash)
	cmd.Dir = repoRoot
	cmd.Env = cleanGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add %s: %w\n%s", commitHash, err, out)
	}
	return tmpDir, nil
}

// RemoveWorktree removes a previously created worktree.
func RemoveWorktree(ctx context.Context, repoRoot, worktreeDir string) error {
	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", "--force", worktreeDir)
	cmd.Dir = repoRoot
	cmd.Env = cleanGitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree remove %s: %w\n%s", worktreeDir, err, out)
	}
	return nil
}

// WorktreePool manages a pool of worktrees for parallel indexing.
// It reuses worktree directories by removing and recreating them.
type WorktreePool struct {
	repoRoot string
	maxSize  int
	sem      chan struct{}
	mu       sync.Mutex
	dirs     []string
}

// NewWorktreePool creates a pool that allows up to maxSize concurrent worktrees.
func NewWorktreePool(repoRoot string, maxSize int) *WorktreePool {
	if maxSize < 1 {
		maxSize = 1
	}
	return &WorktreePool{
		repoRoot: repoRoot,
		maxSize:  maxSize,
		sem:      make(chan struct{}, maxSize),
	}
}

// Acquire creates a worktree at the given commit and returns its directory.
// Blocks if the pool is at capacity. The caller must call Release when done.
func (p *WorktreePool) Acquire(ctx context.Context, commitHash string) (string, error) {
	// Acquire a slot (block if at capacity).
	select {
	case p.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	dir, err := CreateWorktree(ctx, p.repoRoot, commitHash)
	if err != nil {
		<-p.sem // release slot on error
		return "", err
	}
	p.mu.Lock()
	p.dirs = append(p.dirs, dir)
	p.mu.Unlock()
	return dir, nil
}

// Release removes the worktree and releases the pool slot.
func (p *WorktreePool) Release(worktreeDir string) error {
	err := RemoveWorktree(context.Background(), p.repoRoot, worktreeDir)
	p.mu.Lock()
	for i, d := range p.dirs {
		if d == worktreeDir {
			p.dirs = append(p.dirs[:i], p.dirs[i+1:]...)
			break
		}
	}
	p.mu.Unlock()
	<-p.sem // release slot
	return err
}

// Close removes all remaining worktrees. Call this when done with the pool.
func (p *WorktreePool) Close() error {
	p.mu.Lock()
	dirs := make([]string, len(p.dirs))
	copy(dirs, p.dirs)
	p.mu.Unlock()
	var firstErr error
	for _, dir := range dirs {
		if err := RemoveWorktree(context.Background(), p.repoRoot, dir); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
