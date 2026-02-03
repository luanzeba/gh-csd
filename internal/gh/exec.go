package gh

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Result holds the output from a gh command.
type Result struct {
	Stdout []byte
	Stderr []byte
}

// Run executes a gh command and captures both stdout and stderr.
// If the command fails, the error includes the stderr content.
func Run(args ...string) (*Result, error) {
	return RunWithEnv(nil, args...)
}

// RunWithEnv executes a gh command with custom environment variables.
// The env slice should contain strings in "KEY=VALUE" format.
// If the command fails, the error includes the stderr content.
func RunWithEnv(env []string, args ...string) (*Result, error) {
	cmd := exec.Command("gh", args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if err != nil {
		return result, wrapError(args, err, stderr.String())
	}

	return result, nil
}

// RunWithStderr executes a gh command, streaming stderr to the terminal
// in real-time while also capturing it. This is useful for commands where
// you want the user to see progress/errors as they happen, but still want
// the stderr content included in error messages on failure.
func RunWithStderr(args ...string) (*Result, error) {
	return RunWithStderrAndEnv(nil, args...)
}

// RunWithStderrAndEnv is like RunWithStderr but allows setting environment variables.
func RunWithStderrAndEnv(env []string, args ...string) (*Result, error) {
	cmd := exec.Command("gh", args...)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	// Tee stderr to both the buffer and os.Stderr
	cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

	err := cmd.Run()
	result := &Result{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if err != nil {
		// Don't include stderr in error message since it was already printed
		return result, fmt.Errorf("gh %s failed: %w", args[0], err)
	}

	return result, nil
}

// wrapError creates a formatted error that includes stderr content if available.
func wrapError(args []string, err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr != "" {
		return fmt.Errorf("gh %s failed: %w\n%s", args[0], err, stderr)
	}
	return fmt.Errorf("gh %s failed: %w", args[0], err)
}
