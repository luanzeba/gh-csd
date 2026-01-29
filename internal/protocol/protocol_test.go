package protocol

import (
	"bytes"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	req := &ExecRequest{
		Type:    "exec",
		Command: []string{"gh", "pr", "create", "--title", "Test PR"},
		Workdir: "/path/to/repo",
	}

	var buf bytes.Buffer
	if err := WriteRequest(&buf, req); err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}

	if decoded.Type != req.Type {
		t.Errorf("Type mismatch: got %q, want %q", decoded.Type, req.Type)
	}
	if len(decoded.Command) != len(req.Command) {
		t.Errorf("Command length mismatch: got %d, want %d", len(decoded.Command), len(req.Command))
	}
	for i, arg := range req.Command {
		if decoded.Command[i] != arg {
			t.Errorf("Command[%d] mismatch: got %q, want %q", i, decoded.Command[i], arg)
		}
	}
	if decoded.Workdir != req.Workdir {
		t.Errorf("Workdir mismatch: got %q, want %q", decoded.Workdir, req.Workdir)
	}
}

func TestResponseRoundTrip(t *testing.T) {
	resp := &ExecResponse{
		Stdout:   "Created PR #42",
		Stderr:   "",
		ExitCode: 0,
	}

	var buf bytes.Buffer
	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	decoded, err := ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}

	if decoded.Stdout != resp.Stdout {
		t.Errorf("Stdout mismatch: got %q, want %q", decoded.Stdout, resp.Stdout)
	}
	if decoded.Stderr != resp.Stderr {
		t.Errorf("Stderr mismatch: got %q, want %q", decoded.Stderr, resp.Stderr)
	}
	if decoded.ExitCode != resp.ExitCode {
		t.Errorf("ExitCode mismatch: got %d, want %d", decoded.ExitCode, resp.ExitCode)
	}
}

func TestResponseWithError(t *testing.T) {
	resp := &ExecResponse{
		Error:    "command not allowed",
		ExitCode: 1,
	}

	var buf bytes.Buffer
	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse failed: %v", err)
	}

	decoded, err := ReadResponse(&buf)
	if err != nil {
		t.Fatalf("ReadResponse failed: %v", err)
	}

	if decoded.Error != resp.Error {
		t.Errorf("Error mismatch: got %q, want %q", decoded.Error, resp.Error)
	}
}
