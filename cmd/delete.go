package cmd

import (
	"bufio"
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

var (
	deleteForce bool
	deleteAll   bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [codespace-names...]",
	Short: "Delete codespaces interactively",
	Long: `Delete one or more codespaces.

Without arguments, opens an interactive fzf picker with multi-select.
Use Tab to select multiple codespaces, Enter to confirm.

If the current codespace is deleted, the selection is cleared.`,
	RunE: runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Skip confirmation prompt")
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all codespaces (requires --force)")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	var toDelete []string

	if deleteAll {
		if !deleteForce {
			return fmt.Errorf("--all requires --force flag")
		}
		codespaces, err := gh.ListCodespaces()
		if err != nil {
			return err
		}
		for _, cs := range codespaces {
			toDelete = append(toDelete, cs.Name)
		}
	} else if len(args) > 0 {
		toDelete = args
	} else {
		// Interactive multi-select
		selected, err := selectCodespacesForDeletion()
		if err != nil {
			return err
		}
		toDelete = selected
	}

	if len(toDelete) == 0 {
		fmt.Println("No codespaces selected.")
		return nil
	}

	// Confirm deletion
	if !deleteForce {
		fmt.Printf("Delete %d codespace(s):\n", len(toDelete))
		for _, name := range toDelete {
			fmt.Printf("  - %s\n", name)
		}
		fmt.Print("\nConfirm? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Get current codespace to check if we need to clear it
	currentCS, _ := state.Get()

	// Delete each codespace
	var failed []string
	for _, name := range toDelete {
		fmt.Printf("Deleting %s... ", name)
		if err := deleteCodespace(name); err != nil {
			fmt.Printf("FAILED: %v\n", err)
			failed = append(failed, name)
		} else {
			fmt.Println("done")
			// Clear current selection if deleted
			if name == currentCS {
				state.Clear()
			}
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("failed to delete %d codespace(s)", len(failed))
	}

	return nil
}

func selectCodespacesForDeletion() ([]string, error) {
	// Get terminal width (subtract 3 like select does)
	width := 80 // default
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
		width = w - 3
	}

	// Run gh cs list with TTY forcing for colored, aligned output
	env := []string{fmt.Sprintf("GH_FORCE_TTY=%d", width)}
	result, err := gh.RunWithEnv(env, "cs", "list")
	if err != nil {
		return nil, err
	}

	if len(bytes.TrimSpace(result.Stdout)) == 0 {
		return nil, fmt.Errorf("no codespaces found")
	}

	// Run fzf with multi-select
	// --tac: reverse order so newest codespace is at bottom (where fzf cursor starts)
	// --ansi: preserve colors from gh cs list
	// --bind 'tab:toggle+up': Tab toggles selection and moves cursor up (for bottom-up selection)
	fzfCmd := exec.Command("fzf",
		"--multi",
		"--tac",
		"--ansi",
		"--header", "Select codespaces to delete (Tab to select, Enter to confirm)",
		"--bind", "tab:toggle+up",
	)
	fzfCmd.Stdin = bytes.NewReader(result.Stdout)
	fzfCmd.Stderr = os.Stderr

	output, err := fzfCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 130 {
			return nil, fmt.Errorf("selection cancelled")
		}
		return nil, fmt.Errorf("fzf failed: %w", err)
	}

	// Parse selected codespaces (first whitespace-separated field is the name)
	var selected []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) > 0 {
			selected = append(selected, fields[0])
		}
	}

	return selected, nil
}

func deleteCodespace(name string) error {
	cmd := exec.Command("gh", "cs", "delete", "-c", name, "--force")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
