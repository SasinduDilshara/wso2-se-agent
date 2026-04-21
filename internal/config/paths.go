package config

import (
	"os"
	"path/filepath"
)

const AppName = "wso2-se-agent"
const ConfigDirName = ".wso2-se-agent"
const ConfigDirEnvVar = "WSE_CONFIG_DIR"

func GetConfigDir() (string, error) {
	if dir := os.Getenv(ConfigDirEnvVar); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ConfigDirName), nil
}

func EnsureConfigDir() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

func GetConfigFilePath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

func GetReposFilePath() (string, error) {
	dir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "repos.yaml"), nil
}
