package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
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
		return sshWithRetry(name, cs, cfg)
	}
	return sshOnce(name, cfg, cs.Repository)
}

func sshOnce(name string, cfg *config.Config, repo string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start port forwarding if configured
	var ports []int
	if repoCfg := cfg.GetRepoConfig(repo); repoCfg != nil {
		ports = repoCfg.Ports
	}
	portFwdCmd := startPortForwarding(ctx, name, ports)
	defer stopPortForwarding(portFwdCmd)

	args := buildSSHArgs(name)
	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func sshWithRetry(name string, cs *gh.Codespace, cfg *config.Config) error {
	retries := 0

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Get ports config once
	var ports []int
	if repoCfg := cfg.GetRepoConfig(cs.Repository); repoCfg != nil {
		ports = repoCfg.Ports
	}

	for {
		// Refresh tab title on reconnect
		setTabTitleForCodespace(cs)

		// Start port forwarding for this connection attempt
		ctx, cancel := context.WithCancel(context.Background())
		portFwdCmd := startPortForwarding(ctx, name, ports)

		args := buildSSHArgs(name)
		cmd := exec.Command("gh", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		err := cmd.Run()

		// Stop port forwarding when SSH exits
		cancel()
		stopPortForwarding(portFwdCmd)

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
		// Add rdm TCP port forwarding for clipboard/open
		// rdm clients in SSH sessions connect to localhost:7391
		rdmSocket := getRdmSocketPath()
		if rdmSocket != "" {
			sshArgs = append(sshArgs, "-R", fmt.Sprintf("127.0.0.1:7391:%s", rdmSocket))
		}
	}

	// Add csd socket forwarding for local command execution
	// Forward to ~/.csd/csd.socket in the Codespace (matches local path structure)
	csdSocket := GetServerSocketPath()
	if _, err := os.Stat(csdSocket); err == nil {
		// Use $HOME/.csd/csd.socket as the remote path
		// SSH will expand ~ on the remote side
		sshArgs = append(sshArgs, "-R", fmt.Sprintf("~/.csd/csd.socket:%s", csdSocket))
	}

	if len(sshArgs) > 0 {
		args = append(args, "--")
		args = append(args, sshArgs...)
	}

	return args
}

func getRdmSocketPath() string {
	// Get the actual rdm socket path by running `rdm socket`
	// rdm uses os.TempDir() + "/rdm.sock" which varies by system
	cmd := exec.Command("rdm", "socket")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	socketPath := string(output)
	socketPath = socketPath[:len(socketPath)-1] // Remove trailing newline

	// Verify socket exists
	if _, err := os.Stat(socketPath); err == nil {
		return socketPath
	}

	return ""
}

// startPortForwarding starts gh cs ports forward in the background.
// Returns the exec.Cmd (for cleanup) or nil if no ports configured.
func startPortForwarding(ctx context.Context, codespaceName string, ports []int) *exec.Cmd {
	if len(ports) == 0 {
		return nil
	}

	// Build args: gh cs ports forward 80:80 3000:3000 -c <name>
	args := []string{"cs", "ports", "forward"}
	for _, port := range ports {
		args = append(args, fmt.Sprintf("%d:%d", port, port))
	}
	args = append(args, "-c", codespaceName)

	cmd := exec.CommandContext(ctx, "gh", args...)
	// Discard output to prevent escape sequence leakage into SSH session
	// (gh cs ports forward may query cursor position, causing ^[[...R responses)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start port forwarding: %v\n", err)
		return nil
	}

	// Log which ports are being forwarded (we print our own message since gh output is discarded)
	portStrs := make([]string, len(ports))
	for i, p := range ports {
		portStrs[i] = fmt.Sprintf("%d", p)
	}
	fmt.Printf("Forwarding ports: %s\n", strings.Join(portStrs, ", "))

	return cmd
}

// stopPortForwarding gracefully stops the port forwarding process.
func stopPortForwarding(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Signal(syscall.SIGTERM)
	// Give it a moment to clean up, then wait
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		cmd.Process.Kill()
	}
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
