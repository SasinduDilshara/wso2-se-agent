package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type GlobalConfig struct {
	GitHubUsername string  `yaml:"github_username"`
	RiskThreshold  int     `yaml:"risk_threshold"`
	MaxBudgetUSD   float64 `yaml:"max_budget_usd"`
	LogLevel       string  `yaml:"log_level"`
	ClaudeModel    string  `yaml:"claude_model"`
	WorkspaceRoot  string  `yaml:"workspace_root"`
}

func DefaultGlobalConfig() *GlobalConfig {
	home, _ := os.UserHomeDir()
	return &GlobalConfig{
		GitHubUsername: "",
		RiskThreshold:  7,
		MaxBudgetUSD:   15.0,
		LogLevel:       "info",
		ClaudeModel:    "",
		WorkspaceRoot:  filepath.Join(home, "wse-workspaces"),
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
			return DefaultGlobalConfig(), nil
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

	return cfg, nil
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
