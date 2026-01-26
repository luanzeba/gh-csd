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

	// New fields
	if cfg.Defaults.DefaultPermissions != false {
		t.Error("Default default_permissions should be false")
	}

	if cfg.Defaults.SSHRetry != false {
		t.Error("Default ssh_retry should be false")
	}

	if cfg.Defaults.CopyTerminfo == nil || *cfg.Defaults.CopyTerminfo != true {
		t.Error("Default copy_terminfo should be true")
	}

	// github/github should have special defaults
	ghRepo := cfg.GetRepoConfig("github/github")
	if ghRepo == nil {
		t.Fatal("github/github repo config should exist")
	}
	if ghRepo.DefaultPermissions == nil || *ghRepo.DefaultPermissions != true {
		t.Error("github/github default_permissions should be true")
	}
	if ghRepo.SSHRetry == nil || *ghRepo.SSHRetry != true {
		t.Error("github/github ssh_retry should be true")
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

func TestGetRepoConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Test existing repo
	repoCfg := cfg.GetRepoConfig("github/github")
	if repoCfg == nil {
		t.Fatal("GetRepoConfig(github/github) returned nil")
	}
	if repoCfg.Alias != "gh" {
		t.Errorf("GetRepoConfig(github/github).Alias = %q, want gh", repoCfg.Alias)
	}

	// Test non-existing repo
	repoCfg = cfg.GetRepoConfig("unknown/repo")
	if repoCfg != nil {
		t.Errorf("GetRepoConfig(unknown/repo) = %v, want nil", repoCfg)
	}
}

func TestEffectiveSettings(t *testing.T) {
	cfg := DefaultConfig()

	// Test GetEffectiveMachine
	t.Run("GetEffectiveMachine", func(t *testing.T) {
		// github/github has no machine override, should use default
		if got := cfg.GetEffectiveMachine("github/github"); got != "xLargePremiumLinux" {
			t.Errorf("GetEffectiveMachine(github/github) = %q, want xLargePremiumLinux", got)
		}

		// Unknown repo should use default
		if got := cfg.GetEffectiveMachine("unknown/repo"); got != "xLargePremiumLinux" {
			t.Errorf("GetEffectiveMachine(unknown/repo) = %q, want xLargePremiumLinux", got)
		}

		// Add a repo with custom machine
		cfg.Repos["custom/repo"] = Repo{Machine: "smallLinux"}
		if got := cfg.GetEffectiveMachine("custom/repo"); got != "smallLinux" {
			t.Errorf("GetEffectiveMachine(custom/repo) = %q, want smallLinux", got)
		}
	})

	// Test GetEffectiveDefaultPermissions
	t.Run("GetEffectiveDefaultPermissions", func(t *testing.T) {
		// github/github has default_permissions: true
		if got := cfg.GetEffectiveDefaultPermissions("github/github"); got != true {
			t.Errorf("GetEffectiveDefaultPermissions(github/github) = %v, want true", got)
		}

		// github/meuse has no override, should use default (false)
		if got := cfg.GetEffectiveDefaultPermissions("github/meuse"); got != false {
			t.Errorf("GetEffectiveDefaultPermissions(github/meuse) = %v, want false", got)
		}

		// Unknown repo should use default (false)
		if got := cfg.GetEffectiveDefaultPermissions("unknown/repo"); got != false {
			t.Errorf("GetEffectiveDefaultPermissions(unknown/repo) = %v, want false", got)
		}
	})

	// Test GetEffectiveSSHRetry
	t.Run("GetEffectiveSSHRetry", func(t *testing.T) {
		// github/github has ssh_retry: true
		if got := cfg.GetEffectiveSSHRetry("github/github"); got != true {
			t.Errorf("GetEffectiveSSHRetry(github/github) = %v, want true", got)
		}

		// github/meuse has no override, should use default (false)
		if got := cfg.GetEffectiveSSHRetry("github/meuse"); got != false {
			t.Errorf("GetEffectiveSSHRetry(github/meuse) = %v, want false", got)
		}
	})

	// Test GetEffectiveCopyTerminfo
	t.Run("GetEffectiveCopyTerminfo", func(t *testing.T) {
		if got := cfg.GetEffectiveCopyTerminfo(); got != true {
			t.Errorf("GetEffectiveCopyTerminfo() = %v, want true", got)
		}

		// Test with nil value
		cfg.Defaults.CopyTerminfo = nil
		if got := cfg.GetEffectiveCopyTerminfo(); got != true {
			t.Errorf("GetEffectiveCopyTerminfo() with nil = %v, want true (default)", got)
		}
	})
}
