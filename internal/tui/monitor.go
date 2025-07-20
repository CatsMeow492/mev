package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/viper"
)

// Config holds configuration for the TUI monitor
type Config struct {
	RefreshRate int
	CompactMode bool
	Debug       bool
}

// Model represents the TUI application state
type Model struct {
	config     Config
	status     *EngineStatus
	loading    bool
	error      error
	width      int
	height     int
	lastUpdate time.Time
}

// EngineStatus represents the status data from the API
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

// tickMsg is sent when the refresh timer ticks
type tickMsg time.Time

// statusMsg is sent when status is updated
type statusMsg *EngineStatus

// errorMsg is sent when an error occurs
type errorMsg error

// StartMonitor starts the TUI monitor application
func StartMonitor(config Config) error {
	p := tea.NewProgram(initialModel(config), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialModel(config Config) Model {
	return Model{
		config:  config,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		fetchStatus(),
		tickCmd(m.config.RefreshRate),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			// Manual refresh
			return m, fetchStatus()
		}

	case tickMsg:
		return m, tea.Batch(
			fetchStatus(),
			tickCmd(m.config.RefreshRate),
		)

	case statusMsg:
		m.status = msg
		m.loading = false
		m.error = nil
		m.lastUpdate = time.Now()
		return m, nil

	case errorMsg:
		m.error = msg
		m.loading = false
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Define styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	contentStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2)

	var content string

	// Title
	title := titleStyle.Width(m.width - 2).Render("🎯 MEV Strategy Engine Monitor")
	content += title + "\n\n"

	// Instructions
	instructions := "Press 'r' to refresh manually, 'q' to quit"
	content += lipgloss.NewStyle().Faint(true).Render(instructions) + "\n\n"

	// Status content
	if m.error != nil {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)
		content += errorStyle.Render(fmt.Sprintf("❌ Error: %v", m.error)) + "\n"
	} else if m.loading {
		content += "🔄 Loading status...\n"
	} else if m.status != nil {
		content += m.renderStatus()
	}

	// Last update time
	if !m.lastUpdate.IsZero() {
		updateTime := fmt.Sprintf("Last updated: %s", m.lastUpdate.Format("15:04:05"))
		content += "\n" + lipgloss.NewStyle().Faint(true).Render(updateTime)
	}

	// Wrap content in border
	return contentStyle.Width(m.width - 4).Render(content)
}

func (m Model) renderStatus() string {
	var content string

	// Status indicator
	statusIcon := "❌"
	statusColor := lipgloss.Color("#FF0000")
	if m.status.Status == "running" {
		statusIcon = "✅"
		statusColor = lipgloss.Color("#00FF00")
	} else if m.status.Status == "starting" {
		statusIcon = "🔄"
		statusColor = lipgloss.Color("#FFFF00")
	}

	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)
	content += fmt.Sprintf("Status: %s %s\n", statusIcon, statusStyle.Render(m.status.Status))

	if m.status.Uptime != "" {
		content += fmt.Sprintf("Uptime: %s\n", m.status.Uptime)
	}
	content += fmt.Sprintf("Version: %s\n", m.status.Version)

	// Performance Metrics
	if m.status.Metrics != nil {
		content += "\n📈 Performance Metrics\n"
		content += "─────────────────────\n"
		content += fmt.Sprintf("Opportunities Detected:   %d\n", m.status.Metrics.OpportunitiesDetected)
		content += fmt.Sprintf("Profitable Opportunities: %d\n", m.status.Metrics.ProfitableOpportunities)
		content += fmt.Sprintf("Total Profit:            %s\n", m.status.Metrics.TotalProfit)
		content += fmt.Sprintf("Success Rate:            %.2f%%\n", m.status.Metrics.SuccessRate*100)
		content += fmt.Sprintf("Average Latency:         %s\n", m.status.Metrics.AvgLatency)
	}

	// Connection Status
	if m.status.Connections != nil {
		content += "\n🔗 Connection Status\n"
		content += "──────────────────\n"
		content += fmt.Sprintf("Base RPC:     %s\n", m.status.Connections.BaseRPC)
		content += fmt.Sprintf("WebSocket:    %s\n", m.status.Connections.WebSocket)
		content += fmt.Sprintf("Anvil Forks:  %d\n", m.status.Connections.AnvilForks)
		content += fmt.Sprintf("Queue Size:   %d\n", m.status.Connections.QueueSize)
	}

	return content
}

func fetchStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := getEngineStatus()
		if err != nil {
			return errorMsg(err)
		}
		return statusMsg(status)
	}
}

func tickCmd(refreshRate int) tea.Cmd {
	return tea.Tick(time.Duration(refreshRate)*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func getEngineStatus() (*EngineStatus, error) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		// Engine might not be running
		return &EngineStatus{
			Status:    "offline",
			Version:   "unknown",
			Timestamp: time.Now(),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &EngineStatus{
			Status:    "error",
			Version:   "unknown",
			Timestamp: time.Now(),
		}, nil
	}

	var status EngineStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode status response: %w", err)
	}

	return &status, nil
}
