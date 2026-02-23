package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/luanzeba/gh-csd/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Open the interactive codespaces dashboard",
	RunE:  runTUI,
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

func runTUI(cmd *cobra.Command, args []string) error {
	program := tea.NewProgram(tui.NewModel(), tea.WithAltScreen())
	_, err := program.Run()
	return err
}
