// Command build-web builds the Vite/React frontend and copies the production
// assets into internal/staticapp/embed/public for Go embed builds.
//
// The default path uses Dagger with a pnpm CacheVolume, matching the repo's
// TypeScript indexer build pattern. Set BUILD_WEB_LOCAL=1 to force local pnpm.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
)

const (
	defaultBuilderImage = "node:22-bookworm"
	defaultPNPMVersion  = "10.13.1"
)

type packageJSON struct {
	PackageManager string `json:"packageManager"`
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "build-web: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	repoRoot, err := findRepoRoot()
	if err != nil {
		return err
	}
	moduleRoot, err := filepath.Abs(envDefault("WEB_MODULE_ROOT", filepath.Join(repoRoot, "ui")))
	if err != nil {
		return fmt.Errorf("abs module root: %w", err)
	}
	outDir, err := filepath.Abs(envDefault("WEB_EMBED_OUT", filepath.Join(repoRoot, "internal", "staticapp", "embed", "public")))
	if err != nil {
		return fmt.Errorf("abs embed out: %w", err)
	}
	pnpmVersion := envDefault("WEB_PNPM_VERSION", packageManagerPNPMVersion(moduleRoot))

	if os.Getenv("BUILD_WEB_LOCAL") == "1" {
		return runLocal(ctx, moduleRoot, outDir, pnpmVersion)
	}
	if err := runDagger(ctx, moduleRoot, outDir, pnpmVersion); err != nil {
		if errors.Is(err, errDaggerUnavailable) {
			fmt.Fprintln(os.Stderr, "dagger unavailable, falling back to local pnpm")
			return runLocal(ctx, moduleRoot, outDir, pnpmVersion)
		}
		return err
	}
	return nil
}

var errDaggerUnavailable = errors.New("dagger: engine not reachable")

func runDagger(ctx context.Context, moduleRoot, outDir, pnpmVersion string) error {
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return fmt.Errorf("%w: %v", errDaggerUnavailable, err)
	}
	defer func() { _ = client.Close() }()

	moduleSrc := client.Host().Directory(moduleRoot, dagger.HostDirectoryOpts{
		Exclude: []string{"node_modules", "dist", "storybook-static", ".next", "*.tsbuildinfo"},
	})
	pnpmStore := client.CacheVolume("codebase-browser-ui-pnpm-store")
	pathEnv := "/pnpm:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	image := envDefault("WEB_BUILDER_IMAGE", defaultBuilderImage)

	container := client.Container().
		From(image).
		WithEnvVariable("PNPM_HOME", "/pnpm").
		WithEnvVariable("PATH", pathEnv).
		WithMountedCache("/pnpm/store", pnpmStore).
		WithDirectory("/module", moduleSrc).
		WithWorkdir("/module").
		WithExec([]string{"sh", "-lc", "corepack enable && corepack prepare pnpm@" + pnpmVersion + " --activate"}).
		WithExec([]string{"pnpm", "install", "--frozen-lockfile", "--prefer-offline"}).
		WithExec([]string{"pnpm", "run", "build"})

	if err := recreate(outDir); err != nil {
		return err
	}
	if _, err := container.Directory("/module/dist/public").Export(ctx, outDir); err != nil {
		return fmt.Errorf("export frontend dist: %w", err)
	}
	fmt.Fprintf(os.Stderr, "build-web: wrote %s via Dagger\n", outDir)
	return nil
}

func runLocal(ctx context.Context, moduleRoot, outDir, pnpmVersion string) error {
	if _, err := exec.LookPath("corepack"); err == nil {
		cmd := exec.CommandContext(ctx, "corepack", "enable")
		cmd.Dir = moduleRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()

		cmd = exec.CommandContext(ctx, "corepack", "prepare", "pnpm@"+pnpmVersion, "--activate")
		cmd.Dir = moduleRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
	}
	for _, args := range [][]string{
		{"install", "--frozen-lockfile", "--prefer-offline"},
		{"run", "build"},
	} {
		cmd := exec.CommandContext(ctx, "pnpm", args...)
		cmd.Dir = moduleRoot
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("pnpm %s: %w", strings.Join(args, " "), err)
		}
	}
	src := filepath.Join(moduleRoot, "dist", "public")
	if _, err := os.Stat(filepath.Join(src, "index.html")); err != nil {
		return fmt.Errorf("frontend build did not produce %s: %w", filepath.Join(src, "index.html"), err)
	}
	if err := recreate(outDir); err != nil {
		return err
	}
	if err := copyTree(src, outDir); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "build-web: wrote %s via local pnpm\n", outDir)
	return nil
}

func packageManagerPNPMVersion(moduleRoot string) string {
	data, err := os.ReadFile(filepath.Join(moduleRoot, "package.json"))
	if err != nil {
		return defaultPNPMVersion
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return defaultPNPMVersion
	}
	pm := strings.TrimSpace(pkg.PackageManager)
	if strings.HasPrefix(pm, "pnpm@") {
		return strings.TrimPrefix(pm, "pnpm@")
	}
	return defaultPNPMVersion
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

func envDefault(k, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return fallback
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

func copyFile(src, dst string, mode fs.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, in.Close())
	}()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, out.Close())
	}()
	_, err = io.Copy(out, in)
	return err
}
