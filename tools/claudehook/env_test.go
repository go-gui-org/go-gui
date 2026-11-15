package main

import (
	"os"
	"testing"
)

// gitLocalEnv is the list `git rev-parse --local-env-vars` prints: the
// variables that point git at a repository other than the one around its
// working directory.
var gitLocalEnv = []string{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS",
	"GIT_CONFIG_COUNT", "GIT_OBJECT_DIRECTORY", "GIT_DIR", "GIT_WORK_TREE",
	"GIT_IMPLICIT_WORK_TREE", "GIT_GRAFT_FILE", "GIT_INDEX_FILE",
	"GIT_NO_REPLACE_OBJECTS", "GIT_REPLACE_REF_BASE", "GIT_PREFIX",
	"GIT_SHALLOW_FILE", "GIT_COMMON_DIR",
}

// TestMain clears gitLocalEnv before any test runs. These tests build
// throwaway repos in t.TempDir() and run git there (gitRepo, and the gates
// through gitOutput). git exports GIT_DIR to hooks, and make prepush runs
// these tests from the pre-push hook, so with the variables inherited
// every "temp" git command acted on the repo being pushed: git init set
// core.bare=true, git config wrote user.email=test@example.com into its
// config, and git commit added "init" commits to the pushed branch.
func TestMain(m *testing.M) {
	for _, k := range gitLocalEnv {
		_ = os.Unsetenv(k)
	}
	os.Exit(m.Run())
}
