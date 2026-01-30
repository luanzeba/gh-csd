package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/brasic/launchd"
	"github.com/brasic/launchd/state"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage gh-csd as a launchd system service",
	Long: `Manage gh-csd as a launchd system service.

This allows the gh-csd server to start automatically on boot.

Usage:
  gh csd service install    Install and start the service
  gh csd service uninstall  Stop and remove the service
  gh csd service start      Start the service
  gh csd service stop       Stop the service
  gh csd service status     Show service status`,
	Run: func(cmd *cobra.Command, args []string) {
		svc := csdService()
		fmt.Println(prettyStatus(svc))
	},
}

var serviceInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install gh-csd to run on boot as a macOS LaunchAgent",
	Run:   runServiceInstall,
}

var serviceUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove a previously installed LaunchAgent",
	Run:   runServiceUninstall,
}

var serviceStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the launchd service",
	Run:   runServiceStart,
}

var serviceStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the launchd service",
	Run:   runServiceStop,
}

var serviceStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the service status",
	Run: func(cmd *cobra.Command, args []string) {
		svc := csdService()
		fmt.Println(prettyStatus(svc))
	},
}

func init() {
	serviceCmd.AddCommand(serviceInstallCmd)
	serviceCmd.AddCommand(serviceUninstallCmd)
	serviceCmd.AddCommand(serviceStartCmd)
	serviceCmd.AddCommand(serviceStopCmd)
	serviceCmd.AddCommand(serviceStatusCmd)
	rootCmd.AddCommand(serviceCmd)
}

// csdService returns a launchd.Service for gh-csd.
func csdService() *launchd.Service {
	return launchd.ForRunningProgram("com.github.luanzeba.gh-csd", []string{"server", "start"})
}

func currentExecutableName() string {
	return filepath.Base(os.Args[0])
}

func prettyStatus(svc *launchd.Service) string {
	return fmt.Sprintf("Service: %s\n  Install state: %s\n  Run state:     %s",
		svc.UserSpecifier(),
		svc.InstallState().Pretty(),
		svc.RunState().Pretty(),
	)
}

func runServiceInstall(cmd *cobra.Command, args []string) {
	logger := log.New(os.Stdout, "", 0)
	svc := csdService()

	if svc.IsHealthy() {
		logger.Println("Service is already installed and running, nothing to do!")
		return
	}

	// Install the launchagent to run `gh-csd server start` at boot
	if err := svc.Install(); err != nil {
		logger.Printf("Problem installing: %v\n", err)
		os.Exit(1)
	}

	// Start the service
	if err := svc.Start(); err != nil {
		logger.Printf("Problem starting: %v\n", err)
		os.Exit(1)
	}

	logger.Printf("Service installed and started.\n")
	logger.Printf("The server will now start automatically on boot.\n")
	logger.Printf("Uninstall using: %s service uninstall\n", currentExecutableName())
}

func runServiceUninstall(cmd *cobra.Command, args []string) {
	logger := log.New(os.Stdout, "", 0)
	svc := csdService()

	if !svc.InstallState().Is(state.Installed) {
		logger.Println("Service is not installed.")
		return
	}

	if err := svc.Bootout(true); err != nil {
		logger.Printf("Problem uninstalling: %v\n", err)
		os.Exit(1)
	}

	logger.Println("Service uninstalled.")
}

func runServiceStart(cmd *cobra.Command, args []string) {
	logger := log.New(os.Stdout, "", 0)
	svc := csdService()

	if svc.RunState().Is(state.Running) {
		logger.Println("Service is already running.")
		return
	}

	if !svc.InstallState().Is(state.Installed) {
		logger.Println("Service is not installed. Run 'gh csd service install' first.")
		os.Exit(1)
	}

	if err := svc.Start(); err != nil {
		logger.Printf("Problem starting: %v\n", err)
		os.Exit(1)
	}

	finalState, timedOut := svc.PollUntil(state.Running, 5*time.Second)
	if timedOut {
		logger.Println("Service failed to start. Currently:", finalState.Pretty())
		os.Exit(1)
	}

	logger.Println("Service started.")
}

func runServiceStop(cmd *cobra.Command, args []string) {
	logger := log.New(os.Stdout, "", 0)
	svc := csdService()

	runState := svc.RunState()

	if !runState.Is(state.Running) {
		logger.Println("Service is not running.")
		return
	}

	if err := svc.Stop(); err != nil {
		logger.Printf("Problem stopping: %v\n", err)
		os.Exit(1)
	}

	finalState, timedOut := svc.PollUntil(state.NotRunning, 5*time.Second)
	if timedOut {
		logger.Println("Service failed to stop. Currently:", finalState.Pretty())
		os.Exit(1)
	}

	logger.Println("Service stopped.")
}
