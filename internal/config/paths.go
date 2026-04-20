package config

import (
	"os"
	"path/filepath"
	"strings"
)

const AppName = "wso2-se-agent"
const ConfigDirName = ".wso2-se-agent"
const ConfigDirEnvVar = "WSO2_SE_AGENT_HOME"

func GetConfigDir() (string, error) {
	if override := os.Getenv(ConfigDirEnvVar); override != "" {
		if strings.HasPrefix(override, "~") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			override = filepath.Join(home, override[1:])
		}
		return override, nil
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
