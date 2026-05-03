//go:build ignore

// generate_build.go is invoked by `go generate ./internal/staticapp`. It builds
// the frontend through cmd/build-web (Dagger by default, local pnpm fallback)
// and writes internal/staticapp/embed/public for -tags embed builds.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root, err := findRepoRoot()
	if err != nil {
		log.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/build-web")
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("generate_staticapp: wrote internal/staticapp/embed/public")
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
