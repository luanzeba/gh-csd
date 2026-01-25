package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.Machine != "xLargePremiumLinux" {
		t.Errorf("Default machine = %q, want xLargePremiumLinux", cfg.Defaults.Machine)
	}

	if cfg.Defaults.IdleTimeout != 240 {
		t.Errorf("Default idle_timeout = %d, want 240", cfg.Defaults.IdleTimeout)
	}

	if !cfg.Terminal.SetTabTitle {
		t.Error("Default set_tab_title should be true")
	}
}

func TestResolveAlias(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		alias string
		want  string
	}{
		{"gh", "github/github"},
		{"meuse", "github/meuse"},
		{"bp", "github/billing-platform"},
		{"unknown", "unknown"},
		{"owner/repo", "owner/repo"},
	}

	for _, tt := range tests {
		got := cfg.ResolveAlias(tt.alias)
		if got != tt.want {
			t.Errorf("ResolveAlias(%q) = %q, want %q", tt.alias, got, tt.want)
		}
	}
}

func TestLoadSave(t *testing.T) {
	// Use a temp directory for testing
	tmpDir := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	// Test Load with no config (should return defaults)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Defaults.Machine != "xLargePremiumLinux" {
		t.Errorf("Load() defaults.machine = %q, want xLargePremiumLinux", cfg.Defaults.Machine)
	}

	// Modify and save
	cfg.Defaults.Machine = "customMachine"
	if err := Save(cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	configFile := filepath.Join(tmpDir, "gh-csd", "config.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load again and verify
	cfg2, err := Load()
	if err != nil {
		t.Fatalf("Load() after Save failed: %v", err)
	}
	if cfg2.Defaults.Machine != "customMachine" {
		t.Errorf("Load() after Save: defaults.machine = %q, want customMachine", cfg2.Defaults.Machine)
	}
}
