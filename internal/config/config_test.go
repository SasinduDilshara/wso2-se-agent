package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoRef_WorktreeName(t *testing.T) {
	cases := []struct {
		name string
		ref  RepoRef
		want string
	}{
		{
			name: "falls back to Name when WorktreeDir is empty",
			ref:  RepoRef{Name: "carbon-apimgt"},
			want: "carbon-apimgt",
		},
		{
			name: "uses WorktreeDir override when set",
			ref:  RepoRef{Name: "carbon-apimgt-support", WorktreeDir: "carbon-apimgt"},
			want: "carbon-apimgt",
		},
		{
			name: "WorktreeDir may differ from Name in arbitrary ways",
			ref:  RepoRef{Name: "internal-key", WorktreeDir: "display-name"},
			want: "display-name",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.ref.WorktreeName(); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDefaultGlobalConfig(t *testing.T) {
	cfg := DefaultGlobalConfig()

	if cfg.MaxBudgetUSD != 15.0 {
		t.Errorf("MaxBudgetUSD: got %f, want 15.0", cfg.MaxBudgetUSD)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want %q", cfg.LogLevel, "info")
	}
}

func TestSaveAndLoadGlobalConfig(t *testing.T) {
	// Override HOME so config dir goes to temp
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	cfg := &GlobalConfig{
		GitHubUsername: "testuser",
		MaxBudgetUSD:   20.0,
		LogLevel:       "debug",
		ClaudeModel:    "opus",
		WorkspaceRoot:  "/tmp/test-workspaces",
	}

	if err := SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("SaveGlobalConfig failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(tmpDir, ConfigDirName, "config.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not found: %v", err)
	}

	loaded, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	if loaded.GitHubUsername != "testuser" {
		t.Errorf("GitHubUsername: got %q, want %q", loaded.GitHubUsername, "testuser")
	}
	if loaded.MaxBudgetUSD != 20.0 {
		t.Errorf("MaxBudgetUSD: got %f, want 20.0", loaded.MaxBudgetUSD)
	}
}

func TestLoadGlobalConfigDefaults(t *testing.T) {
	// Override HOME to a dir with no config
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	defer os.Setenv("HOME", origHome)

	cfg, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig failed: %v", err)
	}

	// Should return defaults
	if cfg.MaxBudgetUSD != 15.0 {
		t.Errorf("expected default MaxBudgetUSD 15.0, got %f", cfg.MaxBudgetUSD)
	}
}

func TestGetConfigDirEnvOverride(t *testing.T) {
	origHome := os.Getenv("HOME")
	origEnv := os.Getenv(ConfigDirEnvVar)
	defer os.Setenv("HOME", origHome)
	defer os.Setenv(ConfigDirEnvVar, origEnv)

	homeDir := t.TempDir()
	customDir := t.TempDir()
	os.Setenv("HOME", homeDir)

	// Without the env var, config dir is derived from HOME.
	os.Unsetenv(ConfigDirEnvVar)
	dir, err := GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir: %v", err)
	}
	if want := filepath.Join(homeDir, ConfigDirName); dir != want {
		t.Errorf("default config dir: got %q, want %q", dir, want)
	}

	// With the env var, it wins over HOME.
	os.Setenv(ConfigDirEnvVar, customDir)
	dir, err = GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir: %v", err)
	}
	if dir != customDir {
		t.Errorf("overridden config dir: got %q, want %q", dir, customDir)
	}

	// Derived paths also pick up the override.
	cfgPath, err := GetConfigFilePath()
	if err != nil {
		t.Fatalf("GetConfigFilePath: %v", err)
	}
	if want := filepath.Join(customDir, "config.yaml"); cfgPath != want {
		t.Errorf("config file path: got %q, want %q", cfgPath, want)
	}
	reposPath, err := GetReposFilePath()
	if err != nil {
		t.Fatalf("GetReposFilePath: %v", err)
	}
	if want := filepath.Join(customDir, "repos.yaml"); reposPath != want {
		t.Errorf("repos file path: got %q, want %q", reposPath, want)
	}

	// Empty env var falls back to HOME default.
	os.Setenv(ConfigDirEnvVar, "")
	dir, err = GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir: %v", err)
	}
	if want := filepath.Join(homeDir, ConfigDirName); dir != want {
		t.Errorf("empty env var should fall back: got %q, want %q", dir, want)
	}
}

func TestSaveAndLoadConfigWithEnvOverride(t *testing.T) {
	origHome := os.Getenv("HOME")
	origEnv := os.Getenv(ConfigDirEnvVar)
	defer os.Setenv("HOME", origHome)
	defer os.Setenv(ConfigDirEnvVar, origEnv)

	// Point HOME to an empty temp dir, and the override to a separate one.
	os.Setenv("HOME", t.TempDir())
	customDir := t.TempDir()
	os.Setenv(ConfigDirEnvVar, customDir)

	cfg := &GlobalConfig{
		GitHubUsername: "env-override-user",
		MaxBudgetUSD:   12.5,
		LogLevel:       "debug",
		ClaudeModel:    "opus",
		WorkspaceRoot:  "/tmp/env-override",
	}
	if err := SaveGlobalConfig(cfg); err != nil {
		t.Fatalf("SaveGlobalConfig: %v", err)
	}

	// File must land in the override dir, not HOME.
	if _, err := os.Stat(filepath.Join(customDir, "config.yaml")); err != nil {
		t.Fatalf("config file not written to override dir: %v", err)
	}
	homeCfg := filepath.Join(os.Getenv("HOME"), ConfigDirName, "config.yaml")
	if _, err := os.Stat(homeCfg); !os.IsNotExist(err) {
		t.Errorf("config file should NOT exist under HOME (%s): err=%v", homeCfg, err)
	}

	loaded, err := LoadGlobalConfig()
	if err != nil {
		t.Fatalf("LoadGlobalConfig: %v", err)
	}
	if loaded.GitHubUsername != "env-override-user" {
		t.Errorf("GitHubUsername: got %q, want %q", loaded.GitHubUsername, "env-override-user")
	}
}

func TestSaveAndLoadRepoRegistry(t *testing.T) {
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", t.TempDir())
	defer os.Setenv("HOME", origHome)

	reg := &RepoRegistry{
		Repos: map[string]RepoEntry{
			"carbon-apimgt": {
				LocalPath: "/repos/carbon-apimgt",
				Fork:      "testuser/carbon-apimgt",
				Upstream:  "wso2/carbon-apimgt",
			},
		},
	}

	if err := SaveRepoRegistry(reg); err != nil {
		t.Fatalf("SaveRepoRegistry failed: %v", err)
	}

	loaded, err := LoadRepoRegistry()
	if err != nil {
		t.Fatalf("LoadRepoRegistry failed: %v", err)
	}

	entry, ok := loaded.Repos["carbon-apimgt"]
	if !ok {
		t.Fatal("carbon-apimgt not found in loaded registry")
	}
	if entry.LocalPath != "/repos/carbon-apimgt" {
		t.Errorf("LocalPath: got %q, want %q", entry.LocalPath, "/repos/carbon-apimgt")
	}
	if entry.Fork != "testuser/carbon-apimgt" {
		t.Errorf("Fork: got %q, want %q", entry.Fork, "testuser/carbon-apimgt")
	}
}
