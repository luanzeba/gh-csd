package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/luanzeba/gh-csd/internal/config"
	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
)

var (
	createMachine        string
	createDevcontainer   string
	createBranch         string
	createNoSSH          bool
	createNoTerminfo     bool
	createNoNotify       bool
)

var createCmd = &cobra.Command{
	Use:   "create <repo>",
	Short: "Create a codespace and optionally SSH into it",
	Long: `Create a new codespace for the specified repository.

Repo can be a full name (owner/repo) or an alias defined in config.
After creation:
1. Copies Ghostty terminfo for terminal support
2. Sends a desktop notification when ready
3. SSHes into the codespace with rdm forwarding

Use --no-ssh to just create without connecting.`,
	Args: cobra.ExactArgs(1),
	RunE: runCreate,
}

func init() {
	createCmd.Flags().StringVarP(&createMachine, "machine", "m", "xLargePremiumLinux", "Machine type")
	createCmd.Flags().StringVarP(&createDevcontainer, "devcontainer", "d", ".devcontainer/devcontainer.json", "Devcontainer path")
	createCmd.Flags().StringVarP(&createBranch, "branch", "b", "", "Branch to create codespace from")
	createCmd.Flags().BoolVar(&createNoSSH, "no-ssh", false, "Don't SSH after creation")
	createCmd.Flags().BoolVar(&createNoTerminfo, "no-terminfo", false, "Don't copy Ghostty terminfo")
	createCmd.Flags().BoolVar(&createNoNotify, "no-notify", false, "Don't send desktop notification")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Resolve alias to full repo name
	repo := cfg.ResolveAlias(args[0])
	if !strings.Contains(repo, "/") {
		// Assume it's a GitHub org repo
		repo = "github/" + repo
	}

	fmt.Printf("Creating codespace for %s...\n", repo)

	// Get defaults from config, but flags override
	machine := createMachine
	devcontainer := createDevcontainer
	if !cmd.Flags().Changed("machine") && cfg.Defaults.Machine != "" {
		machine = cfg.Defaults.Machine
	}
	if !cmd.Flags().Changed("devcontainer") && cfg.Defaults.Devcontainer != "" {
		devcontainer = cfg.Defaults.Devcontainer
	}

	// Build gh cs create command
	createArgs := []string{"cs", "create",
		"-R", repo,
		"-m", machine,
		"--devcontainer-path", devcontainer,
		"--status",
	}
	if createBranch != "" {
		createArgs = append(createArgs, "-b", createBranch)
	}

	// Create the codespace
	createCmd := exec.Command("gh", createArgs...)
	var stdout, stderr bytes.Buffer
	createCmd.Stdout = &stdout
	createCmd.Stderr = os.Stderr

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("failed to create codespace: %w\n%s", err, stderr.String())
	}

	name := strings.TrimSpace(stdout.String())
	if name == "" {
		return fmt.Errorf("no codespace name returned")
	}

	fmt.Printf("Created codespace: %s\n", name)

	// Save as current codespace
	if err := state.Set(name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save current codespace: %v\n", err)
	}

	// Copy Ghostty terminfo
	if !createNoTerminfo {
		fmt.Println("Copying Ghostty terminfo...")
		if err := copyTerminfo(name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to copy terminfo: %v\n", err)
		}
	}

	// Send notification
	if !createNoNotify {
		sendNotification("Codespace ready", fmt.Sprintf("✅ %s", name))
	}

	if createNoSSH {
		return nil
	}

	// SSH into the codespace
	fmt.Println("Connecting...")
	sshNoRdm = false
	sshRetry = false
	return sshOnce(name)
}

// expandRepoAlias is deprecated - use config.ResolveAlias instead
func expandRepoAlias(alias string) string {
	cfg, _ := config.Load()
	if cfg != nil {
		resolved := cfg.ResolveAlias(alias)
		if resolved != alias {
			return resolved
		}
	}

	// If it looks like a full repo name, use as-is
	if strings.Contains(alias, "/") {
		return alias
	}

	// Assume it's a GitHub org repo
	return "github/" + alias
}

func copyTerminfo(name string) error {
	// Get terminfo from local Ghostty
	infocmp := exec.Command("infocmp", "-x")
	var terminfo bytes.Buffer
	infocmp.Stdout = &terminfo
	if err := infocmp.Run(); err != nil {
		return fmt.Errorf("infocmp failed: %w", err)
	}

	// Pipe to tic on the remote
	sshCmd := exec.Command("gh", "cs", "ssh", "-c", name, "--", "tic", "-x", "-")
	sshCmd.Stdin = &terminfo
	sshCmd.Stderr = os.Stderr

	return sshCmd.Run()
}

func sendNotification(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q sound name "Glass"`, message, title)
		exec.Command("osascript", "-e", script).Run()
	case "linux":
		exec.Command("notify-send", title, message).Run()
	}
}

// Helper function to check if a codespace with the given repo already exists
func findExistingCodespace(repo string) (*gh.Codespace, error) {
	codespaces, err := gh.ListCodespaces()
	if err != nil {
		return nil, err
	}

	for _, cs := range codespaces {
		if cs.Repository == repo {
			return &cs, nil
		}
	}
	return nil, nil
}
