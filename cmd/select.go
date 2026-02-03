package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var selectCmd = &cobra.Command{
	Use:   "select [codespace-name]",
	Short: "Select the current codespace",
	Long: `Select a codespace as the current working codespace.

If no codespace name is provided, an interactive fzf picker is shown.
The selected codespace is stored in ~/.csd/current and used by other commands.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSelect,
}

func init() {
	rootCmd.AddCommand(selectCmd)
}

func runSelect(cmd *cobra.Command, args []string) error {
	var name string

	if len(args) > 0 {
		name = args[0]
	} else {
		// Interactive selection with fzf
		selected, err := selectCodespaceInteractive()
		if err != nil {
			return err
		}
		name = selected
	}

	// Verify the codespace exists
	exists, err := gh.CodespaceExists(name)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("codespace %q not found", name)
	}

	// Save selection
	if err := state.Set(name); err != nil {
		return fmt.Errorf("failed to save selection: %w", err)
	}

	fmt.Printf("Selected codespace: %s\n", name)
	return nil
}

func selectCodespaceInteractive() (string, error) {
	// Get terminal width (subtract 3 like csw does)
	width := 80 // default
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w - 3
	}

	// Run gh cs list with TTY forcing for colored, aligned output
	env := []string{fmt.Sprintf("GH_FORCE_TTY=%d", width)}
	result, err := gh.RunWithEnv(env, "cs", "list")
	if err != nil {
		return "", err
	}

	if len(bytes.TrimSpace(result.Stdout)) == 0 {
		return "", fmt.Errorf("no codespaces found")
	}

	// Pipe to fzf with --tac --ansi (matches csw behavior)
	// --tac: reverse order so newest codespace is at bottom (where fzf cursor starts)
	// --ansi: preserve colors from gh cs list
	fzfCmd := exec.Command("fzf", "--tac", "--ansi")
	fzfCmd.Stdin = bytes.NewReader(result.Stdout)
	fzfCmd.Stderr = os.Stderr

	output, err := fzfCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", fmt.Errorf("selection cancelled")
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}

	// Extract codespace name (first whitespace-separated field)
	selected := strings.TrimSpace(string(output))
	fields := strings.Fields(selected)
	if len(fields) == 0 {
		return "", fmt.Errorf("no selection made")
	}

	return fields[0], nil
}
