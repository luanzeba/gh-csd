package cmd

import (
	"errors"
	"fmt"

	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Print the current codespace name",
	Long: `Print the name of the currently selected codespace.

This is useful for scripts and shell prompts.
Exit code 1 if no codespace is selected.`,
	Args: cobra.NoArgs,
	RunE: runGet,
}

func init() {
	rootCmd.AddCommand(getCmd)
}

func runGet(cmd *cobra.Command, args []string) error {
	name, err := state.Get()
	if err != nil {
		if errors.Is(err, state.ErrNoCodespace) {
			return fmt.Errorf("no codespace selected (use 'gh csd select' to select one)")
		}
		return err
	}

	fmt.Println(name)
	return nil
}
