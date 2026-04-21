package testutil

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestEnv holds all paths for an isolated test environment.
type TestEnv struct {
	T            *testing.T
	RootDir      string // temp root for everything
	ConfigDir    string // ~/.wso2-se-agent equivalent
	WorkspaceDir string // workspace root
	MocksDir     string // directory with mock binaries
	FixturesDir  string // directory with test fixtures
	RepoDir      string // directory with fake git repos
	OrigPath     string // original PATH to restore
	OrigHome     string // original HOME to restore
}

// NewTestEnv creates an isolated test environment with mock binaries on PATH.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	rootDir, err := os.MkdirTemp("", "wse-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(rootDir)
	})

	// Find project root (where tests/ directory lives)
	projectRoot := findProjectRoot(t)

	env := &TestEnv{
		T:            t,
		RootDir:      rootDir,
		ConfigDir:    filepath.Join(rootDir, ".wso2-se-agent"),
		WorkspaceDir: filepath.Join(rootDir, "workspaces"),
		MocksDir:     filepath.Join(projectRoot, "tests", "mocks"),
		FixturesDir:  filepath.Join(projectRoot, "tests", "fixtures"),
		RepoDir:      filepath.Join(rootDir, "repos"),
		OrigPath:     os.Getenv("PATH"),
		OrigHome:     os.Getenv("HOME"),
	}

	// Create directories
	for _, dir := range []string{env.ConfigDir, env.WorkspaceDir, env.RepoDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Put mocks first on PATH so they shadow real binaries
	os.Setenv("PATH", env.MocksDir+":"+env.OrigPath)
	os.Setenv("HOME", rootDir)

	return env
}

// Cleanup restores the original PATH and HOME.
func (e *TestEnv) Cleanup() {
	os.Setenv("PATH", e.OrigPath)
	os.Setenv("HOME", e.OrigHome)
}

// FixturePath returns the absolute path to a test fixture file.
func (e *TestEnv) FixturePath(name string) string {
	return filepath.Join(e.FixturesDir, name)
}

// CreateFakeRepo creates a minimal git repo at the given path with origin and upstream remotes.
func (e *TestEnv) CreateFakeRepo(name, branch string) string {
	e.T.Helper()

	repoPath := filepath.Join(e.RepoDir, name)
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		e.T.Fatal(err)
	}

	cmds := [][]string{
		{"git", "init", "-q", "-b", branch},
		{"git", "commit", "--allow-empty", "-m", "init", "-q"},
		{"git", "remote", "add", "origin", "https://github.com/testuser/" + name + ".git"},
		{"git", "remote", "add", "upstream", repoPath}, // point upstream to self so upstream/<branch> exists
		{"git", "fetch", "upstream"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoPath
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			e.T.Fatalf("git command %v failed: %v\n%s", args, err, string(out))
		}
	}

	return repoPath
}

// CreateSkillsTarball creates a minimal tarball that mimics the skills repo structure.
func (e *TestEnv) CreateSkillsTarball() string {
	e.T.Helper()

	tarballPath := filepath.Join(e.RootDir, "skills.tar.gz")

	f, err := os.Create(tarballPath)
	if err != nil {
		e.T.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	// GitHub tarballs have a top-level prefix like "owner-repo-sha/"
	prefix := "Tharsanan1-wso2-se-agent-skills-abc1234/"

	files := map[string]string{
		prefix + "skills/risk-assessment/SKILL.md":                  "---\nname: risk-assessment\n---\n# Risk Assessment Skill\n",
		prefix + "skills/reproduce/SKILL.md":                        "---\nname: reproduce\n---\n# Generic Reproduce Skill\n",
		prefix + "api-manager-specific/v4/skills/reproduce/SKILL.md": "---\nname: reproduce\n---\n# APIM Reproduce Skill\n",
		prefix + "api-manager-specific/v4/skills/plan-fix/SKILL.md":  "---\nname: plan-fix\n---\n# Plan Fix Skill\n",
		prefix + "api-manager-specific/v4/CLAUDE.md":                 "# WSO2 API Manager\n\nTest CLAUDE.md\n",
	}

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			e.T.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			e.T.Fatal(err)
		}
	}

	return tarballPath
}

// WriteConfig writes a global config file to the test config dir.
func (e *TestEnv) WriteConfig(content string) {
	e.T.Helper()
	path := filepath.Join(e.ConfigDir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.T.Fatal(err)
	}
}

// WriteRepos writes a repos.yaml to the test config dir.
func (e *TestEnv) WriteRepos(content string) {
	e.T.Helper()
	path := filepath.Join(e.ConfigDir, "repos.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		e.T.Fatal(err)
	}
}

// WriteProductConfig writes a product config to the test config dir.
func (e *TestEnv) WriteProductConfig(product, version, content string) {
	e.T.Helper()
	dir := filepath.Join(e.ConfigDir, "products", product, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.T.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "product-config.yaml"), []byte(content), 0644); err != nil {
		e.T.Fatal(err)
	}
}

// WriteScript writes a phase script to the product config scripts dir.
func (e *TestEnv) WriteScript(product, version, scriptName, content string) {
	e.T.Helper()
	dir := filepath.Join(e.ConfigDir, "products", product, version, "scripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		e.T.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, scriptName), []byte(content), 0755); err != nil {
		e.T.Fatal(err)
	}
}

func findProjectRoot(t *testing.T) string {
	t.Helper()
	// Walk up from the current working directory looking for go.mod
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find project root (go.mod)")
		}
		dir = parent
	}
}
