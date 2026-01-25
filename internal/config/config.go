// Package config manages the gh-csd configuration file.
// Config is stored in ~/.config/gh-csd/config.yaml
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	configDirName  = "gh-csd"
	configFileName = "config.yaml"
)

// Config represents the gh-csd configuration.
type Config struct {
	Defaults Defaults          `yaml:"defaults"`
	Repos    map[string]Repo   `yaml:"repos"`
	Terminal Terminal          `yaml:"terminal"`
}

// Defaults are the default settings for codespace creation.
type Defaults struct {
	Machine      string `yaml:"machine"`
	IdleTimeout  int    `yaml:"idle_timeout"`
	Devcontainer string `yaml:"devcontainer"`
}

// Repo is per-repository configuration.
type Repo struct {
	Alias string   `yaml:"alias"`
	Ports []int    `yaml:"ports"`
}

// Terminal configures terminal integration.
type Terminal struct {
	SetTabTitle bool   `yaml:"set_tab_title"`
	TitleFormat string `yaml:"title_format"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Machine:      "xLargePremiumLinux",
			IdleTimeout:  240,
			Devcontainer: ".devcontainer/devcontainer.json",
		},
		Repos: map[string]Repo{
			"github/github": {
				Alias: "gh",
				Ports: []int{80},
			},
			"github/meuse": {
				Alias: "meuse",
				Ports: []int{3000},
			},
			"github/billing-platform": {
				Alias: "bp",
			},
		},
		Terminal: Terminal{
			SetTabTitle: true,
			TitleFormat: "CS: {repo}:{branch}",
		},
	}
}

// configDir returns the path to the config directory.
func configDir() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, configDirName), nil
}

// configPath returns the full path to the config file.
func configPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Load reads the config from disk, or returns defaults if not found.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// Path returns the config file path.
func Path() (string, error) {
	return configPath()
}

// ResolveAlias looks up a repo by alias and returns the full repo name.
// If no alias matches, returns the input unchanged.
func (c *Config) ResolveAlias(alias string) string {
	for repo, cfg := range c.Repos {
		if cfg.Alias == alias {
			return repo
		}
	}
	return alias
}

// GetRepoConfig returns the configuration for a specific repo.
func (c *Config) GetRepoConfig(repo string) *Repo {
	if cfg, ok := c.Repos[repo]; ok {
		return &cfg
	}
	return nil
}
