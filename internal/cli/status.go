package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check MEV engine status",
	Long: `Check the current status of the MEV engine including system health,
performance metrics, and operational statistics.`,
	RunE: runStatus,
}

var (
	jsonOutput    bool
	watchMode     bool
	watchInterval time.Duration
)

type EngineStatus struct {
	Status      string            `json:"status"`
	Uptime      string            `json:"uptime"`
	Version     string            `json:"version"`
	Timestamp   time.Time         `json:"timestamp"`
	Metrics     *Metrics          `json:"metrics,omitempty"`
	Connections *ConnectionStatus `json:"connections,omitempty"`
}

type Metrics struct {
	OpportunitiesDetected   int     `json:"opportunities_detected"`
	ProfitableOpportunities int     `json:"profitable_opportunities"`
	TotalProfit             string  `json:"total_profit"`
	SuccessRate             float64 `json:"success_rate"`
	AvgLatency              string  `json:"avg_latency"`
}

type ConnectionStatus struct {
	BaseRPC    string `json:"base_rpc"`
	AnvilForks int    `json:"anvil_forks"`
	WebSocket  string `json:"websocket"`
	QueueSize  int    `json:"queue_size"`
}

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "output in JSON format")
	statusCmd.Flags().BoolVarP(&watchMode, "watch", "w", false, "watch mode (continuous updates)")
	statusCmd.Flags().DurationVar(&watchInterval, "interval", 5*time.Second, "watch interval duration")
}

func runStatus(cmd *cobra.Command, args []string) error {
	if watchMode {
		return runWatchStatus()
	}

	status, err := getEngineStatus()
	if err != nil {
		return fmt.Errorf("failed to get engine status: %w", err)
	}

	if jsonOutput {
		return outputJSON(status)
	}

	return outputFormatted(status)
}

func runWatchStatus() error {
	fmt.Printf("📊 Watching MEV Engine status (interval: %v)\n", watchInterval)
	fmt.Println("Press Ctrl+C to stop watching...")
	fmt.Println()

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	// Show initial status
	if err := showCurrentStatus(); err != nil {
		return err
	}

	for {
		select {
		case <-ticker.C:
			fmt.Print("\033[H\033[2J") // Clear screen
			if err := showCurrentStatus(); err != nil {
				return err
			}
		}
	}
}

func showCurrentStatus() error {
	status, err := getEngineStatus()
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return nil
	}

	return outputFormatted(status)
}

func getEngineStatus() (*EngineStatus, error) {
	// Try to get status from API endpoint
	apiHost := viper.GetString("server.host")
	if apiHost == "" {
		apiHost = "localhost"
	}
	apiPort := viper.GetInt("server.port")
	if apiPort == 0 {
		apiPort = 8080
	}

	url := fmt.Sprintf("http://%s:%d/api/v1/status", apiHost, apiPort)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		// Engine might not be running
		return &EngineStatus{
			Status:    "offline",
			Timestamp: time.Now(),
		}, nil
	}
	defer resp.Body.Close()

	var status EngineStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &status, nil
}

func outputJSON(status *EngineStatus) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(status)
}

func outputFormatted(status *EngineStatus) error {
	fmt.Printf("🎯 MEV Strategy Engine Status\n")
	fmt.Printf("=============================\n\n")

	// Status indicator
	statusIcon := "❌"
	if status.Status == "running" {
		statusIcon = "✅"
	} else if status.Status == "starting" {
		statusIcon = "🔄"
	}

	fmt.Printf("Status:      %s %s\n", statusIcon, status.Status)
	if status.Uptime != "" {
		fmt.Printf("Uptime:      %s\n", status.Uptime)
	}
	fmt.Printf("Version:     %s\n", status.Version)
	fmt.Printf("Timestamp:   %s\n", status.Timestamp.Format(time.RFC3339))

	if status.Metrics != nil {
		fmt.Printf("\n📈 Performance Metrics\n")
		fmt.Printf("---------------------\n")
		fmt.Printf("Opportunities Detected:  %d\n", status.Metrics.OpportunitiesDetected)
		fmt.Printf("Profitable Opportunities: %d\n", status.Metrics.ProfitableOpportunities)
		fmt.Printf("Total Profit:           %s\n", status.Metrics.TotalProfit)
		fmt.Printf("Success Rate:           %.2f%%\n", status.Metrics.SuccessRate*100)
		fmt.Printf("Average Latency:        %s\n", status.Metrics.AvgLatency)
	}

	if status.Connections != nil {
		fmt.Printf("\n🔗 Connection Status\n")
		fmt.Printf("-------------------\n")
		fmt.Printf("Base RPC:       %s\n", status.Connections.BaseRPC)
		fmt.Printf("WebSocket:      %s\n", status.Connections.WebSocket)
		fmt.Printf("Anvil Forks:    %d\n", status.Connections.AnvilForks)
		fmt.Printf("Queue Size:     %d\n", status.Connections.QueueSize)
	}

	return nil
}
