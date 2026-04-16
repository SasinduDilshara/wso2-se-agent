package config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ProductsFS is set from outside (main or root cmd) since go:embed
// paths must be relative to the source file's package directory.
// The embed directive lives in the cmd package which can reach products/.
var ProductsFS embed.FS

type RepoRef struct {
	Name   string `yaml:"name"`
	Branch string `yaml:"branch"`
}

type BuildConfig struct {
	PackZipPattern string `yaml:"pack_zip_pattern"`
	PackSource     string `yaml:"pack_source"`
}

type RuntimeConfig struct {
	StartupCommand        string `yaml:"startup_command"`
	HealthCheckLog        string `yaml:"health_check_log"`
	DefaultPorts          []int  `yaml:"default_ports"`
	StartupTimeoutSeconds int    `yaml:"startup_timeout_seconds"`
}

type ProductConfig struct {
	Product     string             `yaml:"product"`
	Version     string             `yaml:"version"`
	Repos       []RepoRef          `yaml:"repos"`
	Build       BuildConfig        `yaml:"build"`
	Runtime     RuntimeConfig      `yaml:"runtime"`
	PhaseLimits map[string]float64 `yaml:"phase_limits"`
	SkipPhases  []string           `yaml:"skip_phases"`
	SkillsRef   string             `yaml:"skills_ref"`

	// SourceDir is the resolved path to the product config directory (set after load)
	SourceDir string `yaml:"-"`
}

func LoadProductConfig(product, version string) (*ProductConfig, error) {
	configPath := filepath.Join("products", product, version, "product-config.yaml")

	data, err := ProductsFS.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("product config not found for %s/%s: %w", product, version, err)
	}

	cfg := &ProductConfig{
		PhaseLimits: make(map[string]float64),
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid product config for %s/%s: %w", product, version, err)
	}

	cfg.SourceDir = filepath.Join("products", product, version)

	return cfg, nil
}

// LoadProductConfigFromDir loads from a local filesystem directory
func LoadProductConfigFromDir(dir string) (*ProductConfig, error) {
	configPath := filepath.Join(dir, "product-config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("product config not found at %s: %w", configPath, err)
	}

	cfg := &ProductConfig{
		PhaseLimits: make(map[string]float64),
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	cfg.SourceDir = dir

	return cfg, nil
}

func ListProducts() ([]string, error) {
	entries, err := ProductsFS.ReadDir("products")
	if err != nil {
		return nil, err
	}

	var products []string
	for _, e := range entries {
		if e.IsDir() {
			products = append(products, e.Name())
		}
	}
	return products, nil
}

func ListVersions(product string) ([]string, error) {
	entries, err := ProductsFS.ReadDir("products/" + product)
	if err != nil {
		return nil, err
	}

	var versions []string
	for _, e := range entries {
		if e.IsDir() {
			versions = append(versions, e.Name())
		}
	}
	return versions, nil
}
