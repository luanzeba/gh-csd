package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
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
	codespaces, err := gh.ListCodespaces()
	if err != nil {
		return "", err
	}

	if len(codespaces) == 0 {
		return "", fmt.Errorf("no codespaces found")
	}

	// Build fzf input: "name | repo | branch | state"
	var lines []string
	for _, cs := range codespaces {
		line := fmt.Sprintf("%s\t%s\t%s\t%s", cs.Name, cs.Repository, cs.Branch, cs.State)
		lines = append(lines, line)
	}

	input := strings.Join(lines, "\n")

	// Run fzf
	fzfCmd := exec.Command("fzf",
		"--header", "Select a codespace",
		"--delimiter", "\t",
		"--with-nth", "2,3,4",
		"--preview", "gh cs view {1}",
		"--preview-window", "right:50%:wrap",
	)
	fzfCmd.Stdin = strings.NewReader(input)
	fzfCmd.Stderr = os.Stderr

	output, err := fzfCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return "", fmt.Errorf("selection cancelled")
		}
		return "", fmt.Errorf("fzf failed: %w", err)
	}

	// Extract the codespace name (first field)
	selected := strings.TrimSpace(string(output))
	parts := strings.Split(selected, "\t")
	if len(parts) == 0 {
		return "", fmt.Errorf("no selection made")
	}

	return parts[0], nil
}
