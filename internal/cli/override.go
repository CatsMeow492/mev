package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var overrideCmd = &cobra.Command{
	Use:   "override",
	Short: "Emergency override commands",
	Long: `Emergency override commands for manual control of the MEV engine.
These commands allow operators to bypass automatic safety mechanisms.`,
}

var emergencyStopCmd = &cobra.Command{
	Use:   "emergency-stop",
	Short: "Emergency stop with confirmation",
	Long: `Immediately stop the MEV engine bypassing normal shutdown procedures.
This should only be used in emergency situations.`,
	RunE: runEmergencyStop,
}

var bypassShutdownCmd = &cobra.Command{
	Use:   "bypass-shutdown",
	Short: "Bypass automatic shutdown",
	Long: `Temporarily bypass automatic shutdown mechanisms triggered by 
performance thresholds. Use with extreme caution.`,
	RunE: runBypassShutdown,
}

var resumeOperationCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume normal operation",
	Long: `Resume normal operation after emergency stop or bypass. This will
re-enable all safety mechanisms and monitoring.`,
	RunE: runResumeOperation,
}

var (
	confirmOverride bool
	bypassDuration  time.Duration
)

func init() {
	rootCmd.AddCommand(overrideCmd)
	overrideCmd.AddCommand(emergencyStopCmd)
	overrideCmd.AddCommand(bypassShutdownCmd)
	overrideCmd.AddCommand(resumeOperationCmd)

	// Emergency stop flags
	emergencyStopCmd.Flags().BoolVar(&confirmOverride, "confirm", false, "confirm emergency stop")

	// Bypass shutdown flags
	bypassShutdownCmd.Flags().BoolVar(&confirmOverride, "confirm", false, "confirm shutdown bypass")
	bypassShutdownCmd.Flags().DurationVar(&bypassDuration, "duration", 30*time.Minute, "bypass duration")
}

func runEmergencyStop(cmd *cobra.Command, args []string) error {
	fmt.Println("⚠️  EMERGENCY STOP REQUESTED")
	fmt.Println("=============================")
	fmt.Println("This will immediately stop the MEV engine bypassing")
	fmt.Println("normal shutdown procedures. This may result in:")
	fmt.Println("• Loss of pending transactions")
	fmt.Println("• Incomplete opportunity processing")
	fmt.Println("• Potential data inconsistency")
	fmt.Println()

	if !confirmOverride {
		fmt.Print("Type 'EMERGENCY STOP' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "EMERGENCY STOP" {
			fmt.Println("❌ Emergency stop cancelled")
			return nil
		}
	}

	fmt.Println("🚨 Executing emergency stop...")

	if err := sendOverrideCommand("emergency_stop", nil); err != nil {
		return fmt.Errorf("failed to send emergency stop: %w", err)
	}

	fmt.Println("✅ Emergency stop executed")
	return nil
}

func runBypassShutdown(cmd *cobra.Command, args []string) error {
	fmt.Println("⚠️  SHUTDOWN BYPASS REQUESTED")
	fmt.Println("==============================")
	fmt.Printf("This will bypass automatic shutdown for %v\n", bypassDuration)
	fmt.Println("WARNING: This disables safety mechanisms that prevent losses")
	fmt.Println("during poor market conditions or system performance issues.")
	fmt.Println()

	if !confirmOverride {
		fmt.Print("Type 'BYPASS SHUTDOWN' to confirm: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "BYPASS SHUTDOWN" {
			fmt.Println("❌ Shutdown bypass cancelled")
			return nil
		}
	}

	fmt.Printf("🔧 Bypassing shutdown for %v...\n", bypassDuration)

	params := map[string]interface{}{
		"duration_seconds": int(bypassDuration.Seconds()),
	}

	if err := sendOverrideCommand("bypass_shutdown", params); err != nil {
		return fmt.Errorf("failed to bypass shutdown: %w", err)
	}

	fmt.Printf("✅ Shutdown bypass activated for %v\n", bypassDuration)
	fmt.Println("⏰ Normal safety mechanisms will resume automatically")
	return nil
}

func runResumeOperation(cmd *cobra.Command, args []string) error {
	fmt.Println("🔄 Resuming normal operation...")

	if err := sendOverrideCommand("resume_operation", nil); err != nil {
		return fmt.Errorf("failed to resume operation: %w", err)
	}

	fmt.Println("✅ Normal operation resumed")
	fmt.Println("🛡️  All safety mechanisms re-enabled")
	return nil
}

func sendOverrideCommand(command string, params map[string]interface{}) error {
	apiHost := viper.GetString("server.host")
	if apiHost == "" {
		apiHost = "localhost"
	}
	apiPort := viper.GetInt("server.port")
	if apiPort == 0 {
		apiPort = 8080
	}

	url := fmt.Sprintf("http://%s:%d/api/v1/override/%s", apiHost, apiPort, command)

	payload := map[string]interface{}{
		"command":   command,
		"timestamp": time.Now().Unix(),
	}

	if params != nil {
		payload["params"] = params
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to send override command: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("override command failed with status: %s", resp.Status)
	}

	return nil
}
