package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the MEV engine",
	Long: `Stop a running MEV engine instance gracefully. This will send a 
SIGTERM signal to allow for proper cleanup and shutdown.`,
	RunE: runStop,
}

var (
	forceKill bool
	pidFile   string
)

func init() {
	rootCmd.AddCommand(stopCmd)

	stopCmd.Flags().BoolVarP(&forceKill, "force", "f", false, "force kill the process (SIGKILL)")
	stopCmd.Flags().StringVar(&pidFile, "pid-file", "./mev-engine.pid", "path to PID file")
}

func runStop(cmd *cobra.Command, args []string) error {
	fmt.Println("🛑 Stopping MEV Engine...")

	// Try to find the process by name if no PID file
	if _, err := os.Stat(pidFile); os.IsNotExist(err) {
		return stopByProcessName()
	}

	// Read PID from file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return fmt.Errorf("invalid PID in file: %w", err)
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send appropriate signal
	var signal os.Signal = syscall.SIGTERM
	if forceKill {
		signal = syscall.SIGKILL
		fmt.Println("⚠️  Force killing process...")
	} else {
		fmt.Println("📨 Sending graceful shutdown signal...")
	}

	if err := process.Signal(signal); err != nil {
		return fmt.Errorf("failed to signal process: %w", err)
	}

	// Clean up PID file
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		fmt.Printf("⚠️  Warning: failed to remove PID file: %v\n", err)
	}

	fmt.Println("✅ MEV Engine stop signal sent")
	return nil
}

func stopByProcessName() error {
	fmt.Println("🔍 Looking for MEV Engine process...")

	// Use pgrep to find the process
	cmd := exec.Command("pgrep", "-f", "mev-engine")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("no running MEV Engine process found")
	}

	pid, err := strconv.Atoi(string(output[:len(output)-1])) // Remove newline
	if err != nil {
		return fmt.Errorf("invalid PID from pgrep: %w", err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	var signal os.Signal = syscall.SIGTERM
	if forceKill {
		signal = syscall.SIGKILL
	}

	fmt.Printf("📨 Sending signal to process %d...\n", pid)
	return process.Signal(signal)
}
