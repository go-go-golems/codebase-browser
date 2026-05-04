package gitutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a temporary git repo with 3 commits.
func cleanGitTestEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name := kv
		if i := len(kv); i > 0 {
			for j, r := range kv {
				if r == '=' {
					name = kv[:j]
					break
				}
			}
		}
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX",
			"GIT_AUTHOR_NAME", "GIT_AUTHOR_EMAIL", "GIT_AUTHOR_DATE",
			"GIT_COMMITTER_NAME", "GIT_COMMITTER_EMAIL", "GIT_COMMITTER_DATE":
			continue
		}
		env = append(env, kv)
	}
	return env
}

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		cmd.Env = cleanGitTestEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %s", name, args, out)
		}
	}
	writeFile := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	run("git", "init")
	run("git", "config", "user.email", "test@test.com")
	run("git", "config", "user.name", "Test")

	writeFile("main.go", `package main
func main() { println("v1") }
`)
	run("git", "add", ".")
	run("git", "commit", "-m", "first commit")

	writeFile("main.go", `package main
func main() { println("v2") }
func greet() { println("hello") }
`)
	run("git", "add", ".")
	run("git", "commit", "-m", "second commit")

	writeFile("main.go", `package main
func main() { println("v3") }
func greet() { println("hello world") }
func farewell() { println("bye") }
`)
	run("git", "add", ".")
	run("git", "commit", "-m", "third commit")

	return dir
}
