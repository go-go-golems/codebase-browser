//go:build ignore

// generate_build.go is invoked by `go generate ./internal/sourcefs`. It walks
// the repository root, filters out generated and local-only directories, and
// copies the remaining tree into internal/sourcefs/embed/source/.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}
	tmpSrc, err := os.MkdirTemp("", "codebase-browser-source-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmpSrc)

	if err := copyFilteredTree(root, tmpSrc); err != nil {
		log.Fatal(err)
	}

	dst := filepath.Join(root, "internal", "sourcefs", "embed", "source")
	if err := recreate(dst); err != nil {
		log.Fatal(err)
	}
	if err := copyTree(tmpSrc, dst); err != nil {
		log.Fatal(err)
	}
	fmt.Println("generate_build: copied", tmpSrc, "->", dst)
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("go.mod not found from %s", dir)
}

func copyFilteredTree(srcRoot, dstRoot string) error {
	shouldSkip := func(rel string) bool {
		norm := "/" + filepath.ToSlash(rel) + "/"
		skips := []string{
			"/.git/",
			"/.cache/",
			"/.idea/",
			"/.playwright-mcp/",
			"/bin/",
			"/dist/",
			"/node_modules/",
			"/ttmp/",
			"/ui/dist/",
			"/ui/storybook-static/",
			"/vendor/",
			"/internal/indexfs/embed/",
			"/internal/sourcefs/embed/",
			"/internal/staticapp/embed/",
		}
		for _, skip := range skips {
			if strings.Contains(norm, skip) {
				return true
			}
		}
		return false
	}

	return filepath.WalkDir(srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if shouldSkip(rel) || (!d.IsDir() && (filepath.Base(p) == "go.mod" || filepath.Base(p) == "go.sum" || strings.HasSuffix(filepath.Base(p), "_test.go"))) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dstRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func recreate(dir string) error {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() == ".keep" {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return os.MkdirAll(dir, 0o755)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}
