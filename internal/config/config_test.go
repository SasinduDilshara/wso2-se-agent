package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultGlobalConfig(t *testing.T) {
	cfg := DefaultGlobalConfig()

	if cfg.RiskThreshold != 7 {
		t.Errorf("RiskThreshold: got %d, want 7", cfg.RiskThreshold)
	}
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
		RiskThreshold:  5,
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
	if loaded.RiskThreshold != 5 {
		t.Errorf("RiskThreshold: got %d, want 5", loaded.RiskThreshold)
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
	if cfg.RiskThreshold != 7 {
		t.Errorf("expected default RiskThreshold 7, got %d", cfg.RiskThreshold)
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
