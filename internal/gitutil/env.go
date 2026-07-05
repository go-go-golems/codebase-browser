package gitutil

import "os"

// cleanGitEnv removes repository-specific Git environment variables inherited
// from hooks or parent Git commands. Helpers in this package set cmd.Dir
// explicitly; carrying variables such as GIT_INDEX_FILE into nested git
// commands can make temporary repos and worktrees operate on the caller's index.
func cleanGitEnv() []string {
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		name := kv
		for i, r := range kv {
			if r == '=' {
				name = kv[:i]
				break
			}
		}
		switch name {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX":
			continue
		}
		env = append(env, kv)
	}
	return env
}
