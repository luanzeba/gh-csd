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
	Defaults Defaults        `yaml:"defaults"`
	Repos    map[string]Repo `yaml:"repos"`
	Hooks    Hooks           `yaml:"hooks"`
	Terminal Terminal        `yaml:"terminal"`
}

// Defaults are the default settings for codespace creation.
type Defaults struct {
	Machine            string `yaml:"machine"`
	IdleTimeout        int    `yaml:"idle_timeout"`
	Devcontainer       string `yaml:"devcontainer"`
	DefaultPermissions bool   `yaml:"default_permissions"`
	SSHRetry           bool   `yaml:"ssh_retry"`
	CopyTerminfo       *bool  `yaml:"copy_terminfo"` // pointer to distinguish unset from false
}

// Repo is per-repository configuration.
type Repo struct {
	Alias              string `yaml:"alias,omitempty"`
	Machine            string `yaml:"machine,omitempty"`
	Devcontainer       string `yaml:"devcontainer,omitempty"`
	DefaultPermissions *bool  `yaml:"default_permissions,omitempty"` // pointer to allow per-repo override
	SSHRetry           *bool  `yaml:"ssh_retry,omitempty"`           // pointer to allow per-repo override
	Ports              []int  `yaml:"ports,omitempty"`
}

// Hooks defines commands to run at various lifecycle points.
type Hooks struct {
	PostCreate []string `yaml:"post_create,omitempty"`
}

// Terminal configures terminal integration.
type Terminal struct {
	SetTabTitle bool   `yaml:"set_tab_title"`
	TitleFormat string `yaml:"title_format"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	copyTerminfo := true
	defaultPermsGH := true
	sshRetryGH := true

	return &Config{
		Defaults: Defaults{
			Machine:            "xLargePremiumLinux",
			IdleTimeout:        240,
			Devcontainer:       ".devcontainer/devcontainer.json",
			DefaultPermissions: false,
			SSHRetry:           false,
			CopyTerminfo:       &copyTerminfo,
		},
		Repos: map[string]Repo{
			"github/github": {
				Alias:              "gh",
				Ports:              []int{80},
				DefaultPermissions: &defaultPermsGH,
				SSHRetry:           &sshRetryGH,
			},
			"github/meuse": {
				Alias: "meuse",
				Ports: []int{3000},
			},
			"github/billing-platform": {
				Alias: "bp",
			},
		},
		Hooks: Hooks{
			PostCreate: []string{},
		},
		Terminal: Terminal{
			SetTabTitle: true,
			TitleFormat: "CS: {short_repo}:{branch}",
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

// GetEffectiveMachine returns the machine type for a repo,
// falling back to the default if not specified.
func (c *Config) GetEffectiveMachine(repo string) string {
	if repoCfg := c.GetRepoConfig(repo); repoCfg != nil && repoCfg.Machine != "" {
		return repoCfg.Machine
	}
	return c.Defaults.Machine
}

// GetEffectiveDevcontainer returns the devcontainer path for a repo,
// falling back to the default if not specified.
func (c *Config) GetEffectiveDevcontainer(repo string) string {
	if repoCfg := c.GetRepoConfig(repo); repoCfg != nil && repoCfg.Devcontainer != "" {
		return repoCfg.Devcontainer
	}
	return c.Defaults.Devcontainer
}

// GetEffectiveDefaultPermissions returns whether to auto-accept permissions for a repo,
// falling back to the default if not specified.
func (c *Config) GetEffectiveDefaultPermissions(repo string) bool {
	if repoCfg := c.GetRepoConfig(repo); repoCfg != nil && repoCfg.DefaultPermissions != nil {
		return *repoCfg.DefaultPermissions
	}
	return c.Defaults.DefaultPermissions
}

// GetEffectiveSSHRetry returns whether to use SSH retry for a repo,
// falling back to the default if not specified.
func (c *Config) GetEffectiveSSHRetry(repo string) bool {
	if repoCfg := c.GetRepoConfig(repo); repoCfg != nil && repoCfg.SSHRetry != nil {
		return *repoCfg.SSHRetry
	}
	return c.Defaults.SSHRetry
}

// GetEffectiveCopyTerminfo returns whether to copy terminfo after creation.
func (c *Config) GetEffectiveCopyTerminfo() bool {
	if c.Defaults.CopyTerminfo != nil {
		return *c.Defaults.CopyTerminfo
	}
	return true // default to true if not set
}
