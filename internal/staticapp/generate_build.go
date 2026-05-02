//go:build ignore

// generate_build.go is invoked by `go generate ./internal/staticapp`. It copies
// the already-built Vite static export assets from ui/dist/public into the
// staticapp embed directory used by -tags embed builds.
package main

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}
	src := filepath.Join(root, "ui", "dist", "public")
	if _, err := os.Stat(filepath.Join(src, "index.html")); err != nil {
		log.Fatalf("SPA assets missing at %s; run `pnpm -C ui run build` first: %v", src, err)
	}
	dst := filepath.Join(root, "internal", "staticapp", "embed", "public")
	if err := recreate(dst); err != nil {
		log.Fatal(err)
	}
	if err := copyTree(src, dst); err != nil {
		log.Fatal(err)
	}
	fmt.Println("generate_staticapp: copied", src, "->", dst)
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

func recreate(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return os.MkdirAll(path, 0o755)
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
