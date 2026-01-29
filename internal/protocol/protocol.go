// Package protocol defines the message format for communication between
// the gh-csd client (in Codespace) and server (on local machine).
package protocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// ExecRequest is sent from the Codespace to the local machine
// to execute a command.
type ExecRequest struct {
	Type    string   `json:"type"`    // Always "exec" for now
	Command []string `json:"command"` // Command and arguments
	Workdir string   `json:"workdir,omitempty"`
}

// ExecResponse is sent back from the local machine with the result.
type ExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// WriteRequest encodes and writes a request to the writer.
func WriteRequest(w io.Writer, req *ExecRequest) error {
	if err := json.NewEncoder(w).Encode(req); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	return nil
}

// ReadRequest decodes a request from the reader.
func ReadRequest(r io.Reader) (*ExecRequest, error) {
	var req ExecRequest
	if err := json.NewDecoder(r).Decode(&req); err != nil {
		return nil, fmt.Errorf("failed to decode request: %w", err)
	}
	return &req, nil
}

// WriteResponse encodes and writes a response to the writer.
func WriteResponse(w io.Writer, resp *ExecResponse) error {
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		return fmt.Errorf("failed to encode response: %w", err)
	}
	return nil
}

// ReadResponse decodes a response from the reader.
func ReadResponse(r io.Reader) (*ExecResponse, error) {
	var resp ExecResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}
