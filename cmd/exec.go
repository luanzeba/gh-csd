package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/luanzeba/gh-csd/internal/gh"
	"github.com/luanzeba/gh-csd/internal/state"
	"github.com/spf13/cobra"
)

const (
	defaultExecConnectTimeoutSeconds = 30
	defaultExecStartTimeoutSeconds   = 300
	maxConfigRefreshBackoff          = 10 * time.Second
)

var (
	execCodespace      string
	execCwd            string
	execConnectTimeout int
	execStartTimeout   int
	execControlPersist string
	execNoMaster       bool
	execRefreshConfig  bool
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] -- <command> [args...]",
	Short: "Execute a command in a codespace over SSH",
	Long: `Execute a single command in a codespace with low latency.

This command is designed for machine-friendly workflows (e.g. editor/agent
integrations). It:
  1. Resolves a target codespace (flag or current selection)
  2. Generates/caches SSH config via 'gh cs ssh --config'
  3. Reuses an SSH control master for fast subsequent calls
  4. Executes one remote command and exits with the same exit code

Use '--' before the remote command so command flags are passed through
without being parsed by gh-csd.

Examples:
  gh csd exec -- pwd
  gh csd exec -c my-codespace -- git status --short
  gh csd exec -C /workspaces/github -- bin/rails runner "puts :ok"`,
	Args:          cobra.MinimumNArgs(1),
	RunE:          runExec,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	execCmd.Flags().StringVarP(&execCodespace, "codespace", "c", "", "Codespace name (defaults to current selection)")
	execCmd.Flags().StringVarP(&execCwd, "cwd", "C", "", "Remote working directory")
	execCmd.Flags().IntVar(&execConnectTimeout, "connect-timeout", defaultExecConnectTimeoutSeconds, "SSH connect timeout in seconds")
	execCmd.Flags().IntVar(&execStartTimeout, "start-timeout", defaultExecStartTimeoutSeconds, "Max seconds to wait when preparing SSH config (covers startup/retries)")
	execCmd.Flags().StringVar(&execControlPersist, "control-persist", "10m", "SSH ControlPersist value")
	execCmd.Flags().BoolVar(&execNoMaster, "no-master", false, "Disable SSH control master reuse")
	execCmd.Flags().BoolVar(&execRefreshConfig, "refresh-config", false, "Force refresh SSH config before executing")
	rootCmd.AddCommand(execCmd)
}

func runExec(cmd *cobra.Command, args []string) error {
	name, err := resolveExecCodespace()
	if err != nil {
		return err
	}

	session, err := newCodespaceExecSession(name, execConnectTimeout, execStartTimeout, execControlPersist)
	if err != nil {
		return err
	}

	if err := session.prepare(execRefreshConfig); err != nil {
		return err
	}

	if !execNoMaster {
		if err := session.ensureControlMaster(); err != nil {
			return err
		}
	}

	remoteCommand := joinCommandForShell(args)
	if execCwd != "" {
		remoteCommand = fmt.Sprintf("cd %s && %s", quoteForShell(execCwd), remoteCommand)
	}

	exitCode, err := session.execute(remoteCommand, !execNoMaster)
	if err != nil {
		return err
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}

	return nil
}

func resolveExecCodespace() (string, error) {
	if execCodespace != "" {
		return execCodespace, nil
	}

	name, err := state.Get()
	if err != nil {
		if errors.Is(err, state.ErrNoCodespace) {
			return "", fmt.Errorf("no codespace selected (use 'gh csd select' or pass --codespace)")
		}
		return "", err
	}

	return name, nil
}

type codespaceExecSession struct {
	name              string
	configPath        string
	controlPath       string
	host              string
	connectTimeoutSec int
	startTimeout      time.Duration
	controlPersist    string
}

func newCodespaceExecSession(name string, connectTimeoutSec, startTimeoutSec int, controlPersist string) (*codespaceExecSession, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve home directory: %w", err)
	}

	sshDir := filepath.Join(home, ".csd", "ssh")
	configPath := filepath.Join(sshDir, fmt.Sprintf("%s.config", sanitizeForFilename(name)))
	controlPath := filepath.Join(sshDir, "cm-%C")

	if connectTimeoutSec <= 0 {
		connectTimeoutSec = defaultExecConnectTimeoutSeconds
	}
	if startTimeoutSec <= 0 {
		startTimeoutSec = defaultExecStartTimeoutSeconds
	}
	if strings.TrimSpace(controlPersist) == "" {
		controlPersist = "10m"
	}

	return &codespaceExecSession{
		name:              name,
		configPath:        configPath,
		controlPath:       controlPath,
		connectTimeoutSec: connectTimeoutSec,
		startTimeout:      time.Duration(startTimeoutSec) * time.Second,
		controlPersist:    controlPersist,
	}, nil
}

func (s *codespaceExecSession) prepare(forceRefresh bool) error {
	if err := os.MkdirAll(filepath.Dir(s.configPath), 0o700); err != nil {
		return fmt.Errorf("failed to create SSH cache directory: %w", err)
	}

	if !forceRefresh {
		if err := s.loadCachedConfig(); err == nil {
			return nil
		}
	}

	if err := s.refreshConfigWithRetry(); err != nil {
		return err
	}

	return nil
}

func (s *codespaceExecSession) loadCachedConfig() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	host, err := parseSSHHost(string(data))
	if err != nil {
		return err
	}

	s.host = host
	return nil
}

func (s *codespaceExecSession) refreshConfigWithRetry() error {
	deadline := time.Now().Add(s.startTimeout)
	backoff := 2 * time.Second
	var lastErr error

	for {
		result, err := gh.Run("cs", "ssh", "-c", s.name, "--config")
		if err == nil {
			configOutput := strings.TrimSpace(string(result.Stdout))
			if configOutput == "" {
				err = fmt.Errorf("received empty SSH config output")
			} else {
				host, parseErr := parseSSHHost(configOutput)
				if parseErr != nil {
					err = parseErr
				} else {
					if writeErr := os.WriteFile(s.configPath, []byte(configOutput+"\n"), 0o600); writeErr != nil {
						return fmt.Errorf("failed to cache SSH config: %w", writeErr)
					}
					s.host = host
					return nil
				}
			}
		}

		lastErr = err
		if !shouldRetryConfigError(lastErr) {
			return fmt.Errorf("failed to prepare SSH config for %s: %w", s.name, lastErr)
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("failed to prepare SSH config for %s within %s: %w", s.name, s.startTimeout, lastErr)
		}

		sleep := backoff
		remaining := time.Until(deadline)
		if sleep > remaining {
			sleep = remaining
		}
		if sleep > 0 {
			time.Sleep(sleep)
		}

		if backoff < maxConfigRefreshBackoff {
			backoff *= 2
			if backoff > maxConfigRefreshBackoff {
				backoff = maxConfigRefreshBackoff
			}
		}
	}
}

func (s *codespaceExecSession) ensureControlMaster() error {
	if s.controlMasterRunning() {
		return nil
	}

	if err := s.startControlMaster(); err == nil {
		return nil
	}

	if err := s.refreshConfigWithRetry(); err != nil {
		return err
	}

	if s.controlMasterRunning() {
		return nil
	}

	if err := s.startControlMaster(); err != nil {
		return fmt.Errorf("failed to establish SSH control master for %s: %w", s.name, err)
	}

	return nil
}

func (s *codespaceExecSession) controlMasterRunning() bool {
	if s.host == "" {
		return false
	}

	args := append(s.sshArgsWithMaster(), "-O", "check", s.host)
	checkCmd := exec.Command("ssh", args...)
	return checkCmd.Run() == nil
}

func (s *codespaceExecSession) startControlMaster() error {
	if s.host == "" {
		return fmt.Errorf("SSH host not initialized")
	}

	args := append(s.sshArgsWithMaster(), "-MNf", s.host)
	startCmd := exec.Command("ssh", args...)
	var stderr bytes.Buffer
	startCmd.Stderr = &stderr

	if err := startCmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}

	return nil
}

func (s *codespaceExecSession) execute(remoteCommand string, useMaster bool) (int, error) {
	if s.host == "" {
		return 0, fmt.Errorf("SSH host not initialized")
	}

	var args []string
	if useMaster {
		args = s.sshArgsWithMaster()
	} else {
		args = s.sshArgsNoMaster()
	}
	args = append(args, s.host, remoteCommand)

	sshCmd := exec.Command("ssh", args...)
	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout

	var stderr bytes.Buffer
	sshCmd.Stderr = io.MultiWriter(os.Stderr, &stderr)

	err := sshCmd.Run()
	if err == nil {
		return 0, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 255 && looksLikeSSHTransportError(stderr.String()) {
			return 0, fmt.Errorf("ssh transport error: %s", strings.TrimSpace(stderr.String()))
		}
		return exitErr.ExitCode(), nil
	}

	return 0, fmt.Errorf("failed to execute ssh command: %w", err)
}

func (s *codespaceExecSession) sshArgsWithMaster() []string {
	args := s.sshArgsNoMaster()
	args = append(args,
		"-o", "ControlMaster=auto",
		"-o", fmt.Sprintf("ControlPersist=%s", s.controlPersist),
		"-o", fmt.Sprintf("ControlPath=%s", s.controlPath),
	)
	return args
}

func (s *codespaceExecSession) sshArgsNoMaster() []string {
	return []string{
		"-F", s.configPath,
		"-o", fmt.Sprintf("ConnectTimeout=%d", s.connectTimeoutSec),
	}
}

func parseSSHHost(configOutput string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(configOutput))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "Host ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return fields[1], nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("could not find Host entry in SSH config output")
}

func quoteForShell(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func joinCommandForShell(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, quoteForShell(arg))
	}
	return strings.Join(quoted, " ")
}

func sanitizeForFilename(value string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		" ", "_",
		":", "_",
	)
	return replacer.Replace(value)
}

func shouldRetryConfigError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	nonRetryableIndicators := []string{
		"not found",
		"too many codespaces running",
		"requires authentication",
		"authentication failed",
		"permission denied",
	}

	for _, indicator := range nonRetryableIndicators {
		if strings.Contains(msg, indicator) {
			return false
		}
	}

	return true
}

func looksLikeSSHTransportError(stderr string) bool {
	msg := strings.ToLower(stderr)
	transportIndicators := []string{
		"could not resolve hostname",
		"connection timed out",
		"operation timed out",
		"connection refused",
		"connection reset",
		"connection closed",
		"kex_exchange_identification",
		"no route to host",
		"banner exchange",
		"proxycommand",
		"failed to invoke ssh rpc",
	}

	for _, indicator := range transportIndicators {
		if strings.Contains(msg, indicator) {
			return true
		}
	}

	return false
}
