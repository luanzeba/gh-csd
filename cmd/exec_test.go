package cmd

import (
	"errors"
	"testing"
)

func TestParseSSHHost(t *testing.T) {
	config := `Host cs.example-123
	User vscode
	ProxyCommand gh cs ssh -c example --stdio
`

	host, err := parseSSHHost(config)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if host != "cs.example-123" {
		t.Fatalf("expected host cs.example-123, got %s", host)
	}
}

func TestParseSSHHostMissing(t *testing.T) {
	_, err := parseSSHHost("User vscode\n")
	if err == nil {
		t.Fatal("expected an error when Host line is missing")
	}
}

func TestJoinCommandForShell(t *testing.T) {
	args := []string{"bin/rails", "runner", "puts 'ok'"}
	got := joinCommandForShell(args)
	want := "'bin/rails' 'runner' 'puts '\"'\"'ok'\"'\"''"

	if got != want {
		t.Fatalf("unexpected command string\nwant: %s\n got: %s", want, got)
	}
}

func TestLooksLikeSSHTransportError(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want bool
	}{
		{name: "transport", msg: "ssh: Could not resolve hostname foo", want: true},
		{name: "rpc", msg: "failed to invoke SSH RPC", want: true},
		{name: "regular stderr", msg: "bin/rake: command not found", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeSSHTransportError(tt.msg)
			if got != tt.want {
				t.Fatalf("unexpected result for %q: want %v, got %v", tt.msg, tt.want, got)
			}
		})
	}
}

func TestShouldRetryConfigError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "transient rpc", err: errors.New("failed to invoke SSH RPC"), want: true},
		{name: "not found", err: errors.New("codespace not found"), want: false},
		{name: "too many running", err: errors.New("You have too many codespaces running"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldRetryConfigError(tt.err)
			if got != tt.want {
				t.Fatalf("unexpected retry decision: want %v, got %v", tt.want, got)
			}
		})
	}
}
