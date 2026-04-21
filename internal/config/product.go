package config

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProductsFS holds the embedded default product configs.
// Set from main.go via go:embed.
var ProductsFS embed.FS

type RepoRef struct {
	Name     string `yaml:"name"`
	Upstream string `yaml:"upstream"` // e.g., "wso2/carbon-apimgt"
	Branch   string `yaml:"branch"`
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
	Product      string             `yaml:"product"`
	Version      string             `yaml:"version"`
	Repos        []RepoRef          `yaml:"repos"`
	Build        BuildConfig        `yaml:"build"`
	Runtime      RuntimeConfig      `yaml:"runtime"`
	PhaseLimits  map[string]float64 `yaml:"phase_limits"`
	SkipPhases   []string           `yaml:"skip_phases"`
	SkillsRepo   string `yaml:"skills_repo"`   // GitHub org/repo for product-specific skills
	SkillsBranch string `yaml:"skills_branch"` // branch/tag/commit
	SkillsRef    string `yaml:"skills_ref"`    // product-specific skills path within repo
	ClaudeMDURL  string `yaml:"claude_md_url"` // GitHub URL to a custom CLAUDE.md (e.g., https://github.com/org/repo/blob/main/CLAUDE.md)

	// SourceDir is the resolved path to the product config directory (set after load)
	SourceDir string `yaml:"-"`
}

// LoadProductConfig loads from the local config dir first.
// Falls back to embedded defaults if not found locally.
func LoadProductConfig(product, version string) (*ProductConfig, error) {
	// Try local config dir first
	configDir, err := GetConfigDir()
	if err == nil {
		localDir := filepath.Join(configDir, "products", product, version)
		localPath := filepath.Join(localDir, "product-config.yaml")
		if _, err := os.Stat(localPath); err == nil {
			return loadFromFile(localPath, localDir)
		}
	}

	// Fall back to embedded
	cfg, err := loadFromEmbed(product, version)
	if err != nil {
		configDir, _ := GetConfigDir()
		productsDir := filepath.Join(configDir, "products")

		// List available products for a helpful error message
		available := ""
		if products, err := ListProducts(); err == nil && len(products) > 0 {
			available = "\n\nAvailable products: " + strings.Join(products, ", ")
		}

		return nil, fmt.Errorf("product config not found for %s/%s%s\n\nTo add a new product, create:\n  %s/%s/%s/product-config.yaml",
			product, version, available, productsDir, product, version)
	}
	return cfg, nil
}

func loadFromFile(path, sourceDir string) (*ProductConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read product config: %w", err)
	}

	cfg := &ProductConfig{PhaseLimits: make(map[string]float64)}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid product config: %w", err)
	}

	cfg.SourceDir = sourceDir
	return cfg, nil
}

func loadFromEmbed(product, version string) (*ProductConfig, error) {
	configPath := filepath.Join("products", product, version, "product-config.yaml")

	data, err := ProductsFS.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("product config not found for %s/%s: %w\nRun: wso2-se-agent config init", product, version, err)
	}

	cfg := &ProductConfig{PhaseLimits: make(map[string]float64)}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid product config for %s/%s: %w", product, version, err)
	}

	cfg.SourceDir = filepath.Join("products", product, version)
	return cfg, nil
}

// CopyEmbeddedProducts copies all embedded product configs to the local config dir.
// Existing files are NOT overwritten unless force is true.
func CopyEmbeddedProducts(force bool) error {
	configDir, err := EnsureConfigDir()
	if err != nil {
		return err
	}

	return fs.WalkDir(ProductsFS, "products", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Relative path from "products" prefix
		relPath := path

		destPath := filepath.Join(configDir, relPath)

		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// Skip if file exists and not forcing
		if !force {
			if _, err := os.Stat(destPath); err == nil {
				return nil
			}
		}

		data, err := ProductsFS.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		perm := os.FileMode(0644)
		if strings.HasSuffix(path, ".sh") {
			perm = 0755
		}
		return os.WriteFile(destPath, data, perm)
	})
}

// ListLocalProducts lists products from the local config dir.
// Falls back to embedded if local dir doesn't exist.
func ListProducts() ([]string, error) {
	configDir, err := GetConfigDir()
	if err == nil {
		localDir := filepath.Join(configDir, "products")
		if entries, err := os.ReadDir(localDir); err == nil {
			var products []string
			for _, e := range entries {
				if e.IsDir() {
					products = append(products, e.Name())
				}
			}
			if len(products) > 0 {
				return products, nil
			}
		}
	}

	// Fall back to embedded
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
	configDir, err := GetConfigDir()
	if err == nil {
		localDir := filepath.Join(configDir, "products", product)
		if entries, err := os.ReadDir(localDir); err == nil {
			var versions []string
			for _, e := range entries {
				if e.IsDir() {
					versions = append(versions, e.Name())
				}
			}
			if len(versions) > 0 {
				return versions, nil
			}
		}
	}

	// Fall back to embedded
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
