package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/luanzeba/gh-csd/internal/protocol"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start local daemon for remote command execution",
	Long: `Start a daemon that listens on a Unix socket for command execution requests.

This allows Codespaces to execute commands on your local machine via SSH
socket forwarding. The server only allows specific commands (like 'gh')
to be executed for security.

Usage:
  1. On local machine: gh csd server
  2. Connect via SSH:   gh csd ssh  (socket forwarding is automatic)
  3. In Codespace:      gh csd local gh pr create --title "My PR"

The server can also be installed as a launchd service to start on boot:
  gh csd server install`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the server in the foreground",
	RunE:  runServerStart,
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running server",
	RunE:  runServerStop,
}

var serverSocketCmd = &cobra.Command{
	Use:   "socket",
	Short: "Print the socket path",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(GetServerSocketPath())
	},
}

// Commands allowed to be executed remotely.
// Only 'gh' is allowed by default for security.
var allowedCommands = []string{"gh"}

func init() {
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverSocketCmd)
	rootCmd.AddCommand(serverCmd)
}

// GetServerSocketPath returns the path to the server's Unix socket.
func GetServerSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csd", "csd.socket")
}

func getServerLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csd", "csd.log")
}

func getPidPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".csd", "csd.pid")
}

// Server handles incoming command execution requests.
type Server struct {
	socketPath string
	logger     *log.Logger
	httpServer *http.Server
	cancel     context.CancelFunc
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Printf("could not read request body: %v", err)
		writeErrorResponse(w, "failed to read request", 1)
		return
	}
	r.Body.Close()

	var req protocol.ExecRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.logger.Printf("could not parse request: %v", err)
		writeErrorResponse(w, "invalid request format", 1)
		return
	}

	s.logger.Printf("received request: type=%s command=%v", req.Type, req.Command)

	switch req.Type {
	case "exec":
		s.handleExec(w, &req)
	case "status":
		w.Write([]byte(`{"status":"running"}`))
	case "stop":
		s.logger.Println("received stop command")
		w.Write([]byte(`{"status":"stopping"}`))
		s.cancel()
	default:
		s.logger.Printf("unknown request type: %s", req.Type)
		writeErrorResponse(w, fmt.Sprintf("unknown request type: %s", req.Type), 1)
	}
}

func (s *Server) handleExec(w http.ResponseWriter, req *protocol.ExecRequest) {
	if len(req.Command) == 0 {
		writeErrorResponse(w, "no command specified", 1)
		return
	}

	// Security check: only allow specific commands
	if !isAllowedCommand(req.Command[0]) {
		s.logger.Printf("blocked command: %s (allowed: %s)", req.Command[0], strings.Join(allowedCommands, ", "))
		writeErrorResponse(w, fmt.Sprintf("command %q not allowed (allowed: %s)", req.Command[0], strings.Join(allowedCommands, ", ")), 1)
		return
	}

	s.logger.Printf("executing: %v", req.Command)

	// Execute command
	cmd := exec.Command(req.Command[0], req.Command[1:]...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			s.logger.Printf("command failed: %v", err)
			writeErrorResponse(w, fmt.Sprintf("command failed: %v", err), 1)
			return
		}
	}

	s.logger.Printf("command completed: exit_code=%d stdout_len=%d stderr_len=%d", exitCode, stdout.Len(), stderr.Len())

	resp := protocol.ExecResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
	json.NewEncoder(w).Encode(resp)
}

func writeErrorResponse(w http.ResponseWriter, errMsg string, exitCode int) {
	resp := protocol.ExecResponse{
		Error:    errMsg,
		ExitCode: exitCode,
	}
	json.NewEncoder(w).Encode(resp)
}

func isAllowedCommand(cmd string) bool {
	base := filepath.Base(cmd)
	for _, allowed := range allowedCommands {
		if base == allowed {
			return true
		}
	}
	return false
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	go func() {
		s.logger.Printf("server listening on %s", s.socketPath)
		err := s.httpServer.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			s.logger.Printf("server error: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err := s.httpServer.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Printf("server shutdown error: %v", err)
	} else {
		s.logger.Println("server shutdown complete")
	}
	return err
}

func (s *Server) Listen(ctx context.Context) error {
	socketDir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Try to listen on the socket
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		// If socket exists and is in use, check if server is running
		if isAddressInUse(err) {
			if isServerRunning(s.socketPath) {
				return fmt.Errorf("server already running on %s", s.socketPath)
			}
			// Stale socket, remove it
			s.logger.Printf("removing stale socket: %s", s.socketPath)
			os.Remove(s.socketPath)
			listener, err = net.Listen("unix", s.socketPath)
		}
		if err != nil {
			return fmt.Errorf("failed to listen on socket: %w", err)
		}
	}
	defer os.Remove(s.socketPath)

	return s.Serve(ctx, listener)
}

func isAddressInUse(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			if errno, ok := sysErr.Err.(syscall.Errno); ok {
				return errno == syscall.EADDRINUSE
			}
		}
	}
	return false
}

func isServerRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func newServer(socketPath string, logger *log.Logger) *Server {
	server := &Server{
		socketPath: socketPath,
		logger:     logger,
	}
	server.httpServer = &http.Server{
		Handler:      server,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		ErrorLog:     logger,
	}
	return server
}

func runServerStart(cmd *cobra.Command, args []string) error {
	socketPath := GetServerSocketPath()

	// Setup logging
	logPath := getServerLogPath()
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0700); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer logFile.Close()

	// Log to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := log.New(multiWriter, "[gh-csd] ", log.LstdFlags)

	// Write PID file
	pidPath := getPidPath()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		logger.Printf("warning: failed to write PID file: %v", err)
	}
	defer os.Remove(pidPath)

	server := newServer(socketPath, logger)

	// Handle signals for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("received signal: %v", sig)
		cancel()
	}()

	fmt.Printf("Starting gh-csd server on %s\n", socketPath)
	fmt.Println("Press Ctrl+C to stop")

	return server.Listen(ctx)
}

func runServerStop(cmd *cobra.Command, args []string) error {
	socketPath := GetServerSocketPath()

	// Try to connect and send stop command
	conn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		// Try PID file as fallback
		pidPath := getPidPath()
		data, err := os.ReadFile(pidPath)
		if err != nil {
			return fmt.Errorf("no server running (cannot connect to socket and no PID file)")
		}

		var pid int
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
			return fmt.Errorf("invalid PID file")
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("server process not found")
		}

		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to stop server: %w", err)
		}

		fmt.Println("Server stop signal sent")
		return nil
	}

	// Send stop command via HTTP
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return conn, nil
			},
		},
		Timeout: 5 * time.Second,
	}

	req := protocol.ExecRequest{Type: "stop"}
	body, _ := json.Marshal(req)

	resp, err := client.Post("http://unix/", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send stop command: %w", err)
	}
	resp.Body.Close()

	fmt.Println("Server stopped")
	return nil
}
