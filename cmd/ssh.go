package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/luanzeba/gh-csd/internal/config"
	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/luanzeba/gh-csd/internal/terminal"
	"github.com/spf13/cobra"
)

var (
	sshRetry      bool
	sshRetryDelay int
	sshMaxRetries int
	sshNoRdm      bool
	sshCodespace  string
)

var sshCmd = &cobra.Command{
	Use:   "ssh [codespace-name]",
	Short: "SSH into a codespace with rdm and local exec support",
	Long: `SSH into a codespace with socket forwarding for rdm and local command execution.

By default, connects to the currently selected codespace.
Use --retry to automatically reconnect on disconnect.

The --retry flag can be set as a default for specific repos in config:

    repos:
      github/github:
        ssh_retry: true

Socket forwarding:
  - rdm: enables clipboard (copy/paste) and open functionality
  - csd: enables 'gh csd local' for running commands on your local machine

To use local command execution:
  1. Start the server on local: gh csd server start
  2. Connect via:              gh csd ssh
  3. In codespace:             gh csd local gh pr create ...`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSSH,
}

func init() {
	sshCmd.Flags().BoolVar(&sshRetry, "retry", false, "Automatically reconnect on disconnect")
	sshCmd.Flags().IntVar(&sshRetryDelay, "retry-delay", 3, "Seconds to wait before reconnecting")
	sshCmd.Flags().IntVar(&sshMaxRetries, "max-retries", 0, "Maximum reconnection attempts (0 = unlimited)")
	sshCmd.Flags().BoolVar(&sshNoRdm, "no-rdm", false, "Disable rdm socket forwarding")
	sshCmd.Flags().StringVarP(&sshCodespace, "codespace", "c", "", "Codespace name (overrides current selection)")
	rootCmd.AddCommand(sshCmd)
}

func runSSH(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	// Determine which codespace to connect to
	name := sshCodespace
	if name == "" && len(args) > 0 {
		name = args[0]
	}
	if name == "" {
		var err error
		name, err = state.Get()
		if err != nil {
			if errors.Is(err, state.ErrNoCodespace) {
				return fmt.Errorf("no codespace specified and none selected (use 'gh csd select' or provide a name)")
			}
			return err
		}
	}

	// Verify codespace exists
	cs, err := gh.GetCodespace(name)
	if err != nil {
		return err
	}

	// Update current selection
	if err := state.Set(name); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update current codespace: %v\n", err)
	}

	fmt.Printf("Connecting to %s (%s @ %s)...\n", cs.Name, cs.Repository, cs.Branch)

	// Set terminal tab title if configured
	setTabTitleForCodespace(cs)

	// Determine if we should use retry: flag overrides config
	useRetry := sshRetry
	if !cmd.Flags().Changed("retry") {
		// Use per-repo config if flag not explicitly set
		useRetry = cfg.GetEffectiveSSHRetry(cs.Repository)
	}

	if useRetry {
		return sshWithRetry(name, cs)
	}
	return sshOnce(name)
}

func sshOnce(name string) error {
	args := buildSSHArgs(name)
	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func sshWithRetry(name string, cs *gh.Codespace) error {
	retries := 0

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for {
		// Refresh tab title on reconnect
		setTabTitleForCodespace(cs)

		args := buildSSHArgs(name)
		cmd := exec.Command("gh", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		// Check for intentional exit (exit code 0 or user interrupt)
		if err == nil {
			fmt.Println("SSH session ended normally.")
			return nil
		}

		// Check if we received an interrupt
		select {
		case <-sigChan:
			fmt.Println("\nDisconnected.")
			return nil
		default:
		}

		retries++
		if sshMaxRetries > 0 && retries >= sshMaxRetries {
			return fmt.Errorf("max retries (%d) reached, giving up", sshMaxRetries)
		}

		fmt.Printf("\nConnection lost. Reconnecting in %d seconds... (attempt %d", sshRetryDelay, retries+1)
		if sshMaxRetries > 0 {
			fmt.Printf("/%d", sshMaxRetries)
		}
		fmt.Println(")")

		// Wait with interrupt handling
		select {
		case <-sigChan:
			fmt.Println("\nReconnection cancelled.")
			return nil
		case <-time.After(time.Duration(sshRetryDelay) * time.Second):
		}
	}
}

func buildSSHArgs(name string) []string {
	args := []string{"cs", "ssh", "-c", name}

	var sshArgs []string

	if !sshNoRdm {
		// Add rdm socket forwarding for clipboard/open
		rdmSocket := getRdmSocketPath()
		if rdmSocket != "" {
			sshArgs = append(sshArgs, "-R", fmt.Sprintf("/home/linuxbrew/.rdm/rdm.socket:%s", rdmSocket))
		}
	}

	// Add csd socket forwarding for local command execution
	csdSocket := GetServerSocketPath()
	if _, err := os.Stat(csdSocket); err == nil {
		sshArgs = append(sshArgs, "-R", fmt.Sprintf("/home/linuxbrew/.csd/csd.socket:%s", csdSocket))
	}

	if len(sshArgs) > 0 {
		args = append(args, "--")
		args = append(args, sshArgs...)
	}

	return args
}

func getRdmSocketPath() string {
	// Check if rdm is running and get socket path
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	socketPath := home + "/.rdm/rdm.socket"
	if _, err := os.Stat(socketPath); err == nil {
		return socketPath
	}

	return ""
}

func setTabTitleForCodespace(cs *gh.Codespace) {
	cfg, err := config.Load()
	if err != nil {
		return
	}

	if !cfg.Terminal.SetTabTitle {
		return
	}

	if !terminal.IsSupportedTerminal() {
		return
	}

	title := terminal.FormatTitle(cfg.Terminal.TitleFormat, cs.Repository, cs.Branch, cs.Name)
	terminal.SetTabTitle(title)
}
