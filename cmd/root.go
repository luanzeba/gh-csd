package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gh-csd",
	Short: "Codespace development workflow tool",
	Long: `gh-csd is a GitHub CLI extension that streamlines codespace development workflows.

It provides commands to create, connect to, and manage codespaces with features like:
- Automatic SSH reconnection on disconnect
- rdm integration for clipboard/open support
- Repo aliases for quick access
- Ghostty tab title integration`,
}

func Execute() error {
	return rootCmd.Execute()
}
