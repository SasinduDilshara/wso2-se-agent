package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type RepoEntry struct {
	LocalPath string `yaml:"local_path"`
	Fork      string `yaml:"fork"`     // e.g., "SasinduDilshara/carbon-apimgt"
	Upstream  string `yaml:"upstream"` // e.g., "wso2/carbon-apimgt"
}

type RepoRegistry struct {
	Repos map[string]RepoEntry `yaml:"repos"`
}

func LoadRepoRegistry() (*RepoRegistry, error) {
	path, err := GetReposFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RepoRegistry{Repos: make(map[string]RepoEntry)}, nil
		}
		return nil, err
	}

	reg := &RepoRegistry{Repos: make(map[string]RepoEntry)}
	if err := yaml.Unmarshal(data, reg); err != nil {
		return nil, err
	}

	return reg, nil
}

func SaveRepoRegistry(reg *RepoRegistry) error {
	dir, err := EnsureConfigDir()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(reg)
	if err != nil {
		return err
	}

	path := dir + "/repos.yaml"
	return os.WriteFile(path, data, 0644)
}
