package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	GitHubUsername      string            `yaml:"github_username"`
	MaxBudgetUSD        float64           `yaml:"max_budget_usd"`
	LogLevel            string            `yaml:"log_level"`
	ClaudeModel         string            `yaml:"claude_model"`
	PhaseModels         map[string]string `yaml:"phase_models,omitempty"`
	WorkspaceRoot       string            `yaml:"workspace_root"`
	GenericSkillsRepo   string            `yaml:"generic_skills_repo"`
	GenericSkillsBranch string            `yaml:"generic_skills_branch"`
	GenericSkillsRef    string            `yaml:"generic_skills_ref"`
}

func DefaultGlobalConfig() *GlobalConfig {
	home, _ := os.UserHomeDir()
	return &GlobalConfig{
		GitHubUsername:      "",
		MaxBudgetUSD:        15.0,
		LogLevel:            "info",
		ClaudeModel:         "",
		WorkspaceRoot:       filepath.Join(home, "wse-workspaces"),
		GenericSkillsRepo:   "",
		GenericSkillsBranch: "main",
		GenericSkillsRef:    "skills",
	}
}

func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := GetConfigFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultGlobalConfig()
			cfg.GitHubUsername = detectGitHubUsername()
			return cfg, nil
		}
		return nil, err
	}

	cfg := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Expand ~ in workspace root
	if len(cfg.WorkspaceRoot) > 0 && cfg.WorkspaceRoot[0] == '~' {
		home, _ := os.UserHomeDir()
		cfg.WorkspaceRoot = filepath.Join(home, cfg.WorkspaceRoot[1:])
	}

	// Auto-detect GitHub username if not set
	if cfg.GitHubUsername == "" {
		cfg.GitHubUsername = detectGitHubUsername()
	}

	return cfg, nil
}

// detectGitHubUsername uses gh api to get the authenticated user's login.
func detectGitHubUsername() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func SaveGlobalConfig(cfg *GlobalConfig) error {
	dir, err := EnsureConfigDir()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644)
}
