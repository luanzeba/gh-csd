package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/luanzeba/gh-csd/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configEdit bool
	configInit bool
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or edit configuration",
	Long: `View or edit the gh-csd configuration file.

Without flags, prints the current configuration.
Use --edit to open in $EDITOR.
Use --init to create a default config file.

Config location: ~/.config/gh-csd/config.yaml`,
	RunE: runConfig,
}

func init() {
	configCmd.Flags().BoolVarP(&configEdit, "edit", "e", false, "Open config in $EDITOR")
	configCmd.Flags().BoolVar(&configInit, "init", false, "Create default config file")
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}

	if configInit {
		// Check if config already exists
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists at %s", path)
		}

		cfg := config.DefaultConfig()
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to create config: %w", err)
		}

		fmt.Printf("Created config at %s\n", path)
		return nil
	}

	if configEdit {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim"
		}

		// Create default config if it doesn't exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			cfg := config.DefaultConfig()
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("failed to create config: %w", err)
			}
		}

		editCmd := exec.Command(editor, path)
		editCmd.Stdin = os.Stdin
		editCmd.Stdout = os.Stdout
		editCmd.Stderr = os.Stderr
		return editCmd.Run()
	}

	// Print current config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	fmt.Printf("# Config file: %s\n\n", path)
	fmt.Println(string(data))
	return nil
}
