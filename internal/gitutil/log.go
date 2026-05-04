// Package gitutil wraps the git CLI for commit listing, worktree management,
// and file-content retrieval. It does not use a Go git library because the
// CLI is faster for large repos and handles all edge cases (submodules, LFS,
// etc.).
package gitutil

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Commit holds the metadata we extract from `git log` for a single commit.
type Commit struct {
	Hash         string
	ShortHash    string
	Message      string
	AuthorName   string
	AuthorEmail  string
	AuthorTime   time.Time
	ParentHashes []string
	TreeHash     string
	// Sequence is an explicit range-order value assigned by review indexing.
	// Higher values are considered later in the indexed review range.
	Sequence int
}

// LogCommits lists commits in the given range. The rangeSpec is passed
// directly to `git log`, so it supports:
//
//	"HEAD~10..HEAD"         — last 10 commits
//	"main..feature-branch"  — commits on feature-branch not on main
//	"--all"                 — all reachable commits
//	""                      — defaults to HEAD
func LogCommits(ctx context.Context, repoRoot, rangeSpec string) ([]Commit, error) {
	args := []string{
		"log",
		"--format=" + logFormat,
	}
	if rangeSpec != "" {
		args = append(args, rangeSpec)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseLogOutput(out)
}

// ChangedFiles returns the list of file paths changed in a commit,
// relative to the repo root.
func ChangedFiles(ctx context.Context, repoRoot, commitHash string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff-tree", "--no-commit-id", "-r", "--name-only", commitHash)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff-tree %s: %w", commitHash, err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// IsAncestor checks if parent is an ancestor of child.
func IsAncestor(ctx context.Context, repoRoot, parent, child string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "merge-base", "--is-ancestor", parent, child)
	cmd.Dir = repoRoot
	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("git merge-base --is-ancestor: %w", err)
	}
	return true, nil
}

// ResolveRef resolves a ref (like "HEAD" or "main") to a full commit hash.
func ResolveRef(ctx context.Context, repoRoot, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", ref)
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s: %w", ref, err)
	}
	return strings.TrimSpace(string(out)), nil
}

const logFormat = "%H%n%h%n%s%n%an%n%ae%n%at%n%T%n<PARENTS>%P%n<END>"

func parseLogOutput(data []byte) ([]Commit, error) {
	var commits []Commit
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var current *Commit

	for scanner.Scan() {
		line := scanner.Text()
		if current == nil {
			current = &Commit{}
		}

		if line == "<END>" {
			if current.Hash != "" {
				commits = append(commits, *current)
			}
			current = nil
			continue
		}

		if strings.HasPrefix(line, "<PARENTS>") {
			parentsStr := strings.TrimPrefix(line, "<PARENTS>")
			parentsStr = strings.TrimSpace(parentsStr)
			if parentsStr != "" {
				current.ParentHashes = strings.Fields(parentsStr)
			}
			continue
		}

		// Fill current commit fields in order matching logFormat.
		// We track which field we're on based on what's empty.
		if current.Hash == "" {
			current.Hash = line
			continue
		}
		if current.ShortHash == "" {
			current.ShortHash = line
			continue
		}
		if current.Message == "" {
			current.Message = line
			continue
		}
		if current.AuthorName == "" {
			current.AuthorName = line
			continue
		}
		if current.AuthorEmail == "" {
			current.AuthorEmail = line
			continue
		}
		if current.AuthorTime.IsZero() {
			unixSec, err := strconv.ParseInt(line, 10, 64)
			if err == nil {
				current.AuthorTime = time.Unix(unixSec, 0)
			}
			continue
		}
		if current.TreeHash == "" {
			current.TreeHash = line
			continue
		}
	}

	return commits, nil
}
