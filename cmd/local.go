package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/luanzeba/gh-csd/internal/protocol"
	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local <command> [args...]",
	Short: "Execute command on local machine via forwarded socket",
	Long: `Execute a command on your local machine from inside a Codespace.

This requires:
  1. gh-csd server running on your local machine
  2. Connecting via 'gh csd ssh' (which forwards the socket automatically)

Only 'gh' commands are allowed for security. This is useful for:
  - Creating PRs in repos you don't have Codespace token access to
  - Creating issues in other repositories
  - Any gh command that needs your local machine's credentials

Examples:
  # Create a PR in a different repo
  gh csd local gh pr create -R github/github-ui --title "Fix bug"

  # Create an issue
  gh csd local gh issue create -R github/Copilot-Controls --title "Bug report"

  # Check PR status
  gh csd local gh pr status`,
	Args:               cobra.MinimumNArgs(1),
	RunE:               runLocal,
	DisableFlagParsing: true, // Pass all args to the remote command
}

func init() {
	rootCmd.AddCommand(localCmd)
}

// getRemoteSocketPath returns the path where the socket is forwarded
// inside a Codespace.
func getRemoteSocketPath() string {
	// When in Codespace, the socket is forwarded to ~/.csd/csd.socket
	// This matches the local path structure and avoids hardcoded paths
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback for edge cases
		return "/home/codespace/.csd/csd.socket"
	}
	return home + "/.csd/csd.socket"
}

func runLocal(cmd *cobra.Command, args []string) error {
	socketPath := getRemoteSocketPath()

	// Check if socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		return fmt.Errorf(`socket not found at %s

This command only works inside a Codespace connected via 'gh csd ssh'.

Make sure:
  1. On your local machine: gh csd server start
  2. Connect to Codespace:  gh csd ssh
  3. Then run:              gh csd local gh <command>`, socketPath)
	}

	// Connect to the socket
	conn, err := net.DialTimeout("unix", socketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf(`failed to connect to local daemon at %s: %w

Make sure:
  1. gh csd server is running on your local machine
  2. You connected via 'gh csd ssh' (not plain 'gh cs ssh')`, socketPath, err)
	}

	// Create HTTP client that uses the Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return conn, nil
			},
		},
		Timeout: 60 * time.Second, // Commands might take a while
	}

	// Build and send request
	req := &protocol.ExecRequest{
		Type:    "exec",
		Command: args,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := client.Post("http://unix/", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var execResp protocol.ExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&execResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	// Handle error from server
	if execResp.Error != "" {
		fmt.Fprintln(os.Stderr, execResp.Error)
		os.Exit(execResp.ExitCode)
	}

	// Print output
	if execResp.Stdout != "" {
		fmt.Print(execResp.Stdout)
	}
	if execResp.Stderr != "" {
		fmt.Fprint(os.Stderr, execResp.Stderr)
	}

	// Exit with same code as remote command
	if execResp.ExitCode != 0 {
		os.Exit(execResp.ExitCode)
	}

	return nil
}
