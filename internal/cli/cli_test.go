package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLICommands(t *testing.T) {
	// Setup test environment
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	tests := []struct {
		name           string
		args           []string
		expectedOutput string
		expectedError  bool
	}{
		{
			name:           "help command",
			args:           []string{"--help"},
			expectedOutput: "MEV Strategy Engine for Base Layer 2",
			expectedError:  false,
		},
		{
			name:           "version command",
			args:           []string{"--version"},
			expectedOutput: "1.0.0",
			expectedError:  false,
		},
		{
			name:           "start help",
			args:           []string{"start", "--help"},
			expectedOutput: "Start the MEV engine",
			expectedError:  false,
		},
		{
			name:           "stop help",
			args:           []string{"stop", "--help"},
			expectedOutput: "Stop a running MEV engine",
			expectedError:  false,
		},
		{
			name:           "status help",
			args:           []string{"status", "--help"},
			expectedOutput: "Check the current status",
			expectedError:  false,
		},
		{
			name:           "monitor help",
			args:           []string{"monitor", "--help"},
			expectedOutput: "terminal-based monitoring",
			expectedError:  false,
		},
		{
			name:           "override help",
			args:           []string{"override", "--help"},
			expectedOutput: "Emergency override commands",
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := executeCommand(tt.args...)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, output, tt.expectedOutput)
			}
		})
	}
}

func TestStatusCommand(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	// Test offline status (no server running)
	t.Run("offline status", func(t *testing.T) {
		output, err := executeCommand("status")
		assert.NoError(t, err)
		assert.Contains(t, output, "offline")
	})

	// Test status with mock server
	t.Run("online status", func(t *testing.T) {
		server := createMockAPIServer(t)
		defer server.Close()

		// Configure viper to use test server
		setupTestServerConfig(server.URL)

		output, err := executeCommand("status")
		assert.NoError(t, err)
		assert.Contains(t, output, "running")
		assert.Contains(t, output, "Performance Metrics")
	})

	// Test JSON output
	t.Run("json output", func(t *testing.T) {
		server := createMockAPIServer(t)
		defer server.Close()

		setupTestServerConfig(server.URL)

		output, err := executeCommand("status", "--json")
		assert.NoError(t, err)
		assert.Contains(t, output, `"status":"running"`)
		assert.Contains(t, output, `"version":"1.0.0"`)
	})
}

func TestStopCommand(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	t.Run("stop non-existent process", func(t *testing.T) {
		// Create fake PID file
		pidFile := filepath.Join(t.TempDir(), "test-mev-engine.pid")
		err := os.WriteFile(pidFile, []byte("99999"), 0644)
		require.NoError(t, err)

		output, err := executeCommand("stop", "--pid-file", pidFile)
		// Should handle non-existent process gracefully
		assert.Error(t, err)
		assert.Contains(t, output, "failed to signal process")
	})

	t.Run("stop with invalid PID file", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "invalid-pid.pid")
		err := os.WriteFile(pidFile, []byte("invalid"), 0644)
		require.NoError(t, err)

		output, err := executeCommand("stop", "--pid-file", pidFile)
		assert.Error(t, err)
		assert.Contains(t, output, "invalid PID")
	})
}

func TestOverrideCommands(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	server := createMockAPIServer(t)
	defer server.Close()
	setupTestServerConfig(server.URL)

	t.Run("emergency stop with confirmation", func(t *testing.T) {
		output, err := executeCommand("override", "emergency-stop", "--confirm")
		assert.NoError(t, err)
		assert.Contains(t, output, "Emergency stop executed")
	})

	t.Run("bypass shutdown with confirmation", func(t *testing.T) {
		output, err := executeCommand("override", "bypass-shutdown", "--confirm", "--duration", "5m")
		assert.NoError(t, err)
		assert.Contains(t, output, "Shutdown bypass activated")
	})

	t.Run("resume operation", func(t *testing.T) {
		output, err := executeCommand("override", "resume")
		assert.NoError(t, err)
		assert.Contains(t, output, "Normal operation resumed")
	})
}

func TestConfigurationFlags(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	// Create test config file
	configDir := t.TempDir()
	configFile := filepath.Join(configDir, "test-config.yaml")
	configContent := `
server:
  host: "test-host"
  port: 9999
debug: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Run("custom config file", func(t *testing.T) {
		output, err := executeCommand("--config", configFile, "status")
		assert.NoError(t, err)
		// Should handle custom config without error
		assert.NotEmpty(t, output)
	})

	t.Run("debug flag", func(t *testing.T) {
		output, err := executeCommand("--debug", "status")
		assert.NoError(t, err)
		assert.NotEmpty(t, output)
	})
}

func TestStartCommandValidation(t *testing.T) {
	setupTestEnvironment(t)
	defer cleanupTestEnvironment(t)

	t.Run("start with custom flags", func(t *testing.T) {
		// Start command should validate flags without actually starting
		// We'll use a short timeout context to test validation
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// This would normally start the engine, but should validate flags
		output, err := executeCommandWithContext(ctx, "start", "--bind", "127.0.0.1", "--port", "8888")
		// Since we're canceling quickly, we expect either success (validation passed)
		// or a context canceled error
		if err != nil {
			assert.Contains(t, err.Error(), "context")
		}
		assert.NotEmpty(t, output)
	})
}

// Helper functions

func setupTestEnvironment(t *testing.T) {
	// Reset viper configuration
	viper.Reset()

	// Set test defaults
	viper.Set("server.host", "localhost")
	viper.Set("server.port", 8080)
	viper.Set("debug", false)
}

func cleanupTestEnvironment(t *testing.T) {
	viper.Reset()
}

func executeCommand(args ...string) (string, error) {
	return executeCommandWithContext(context.Background(), args...)
}

func executeCommandWithContext(ctx context.Context, args ...string) (string, error) {
	buf := new(bytes.Buffer)

	// Create a new root command for testing
	testRootCmd := &cobra.Command{
		Use: "mev-engine",
	}

	// Add all subcommands
	testRootCmd.AddCommand(startCmd)
	testRootCmd.AddCommand(stopCmd)
	testRootCmd.AddCommand(statusCmd)
	testRootCmd.AddCommand(monitorCmd)
	testRootCmd.AddCommand(overrideCmd)

	testRootCmd.SetOut(buf)
	testRootCmd.SetErr(buf)
	testRootCmd.SetArgs(args)

	// Set context if provided
	if ctx != context.Background() {
		testRootCmd.SetContext(ctx)
	}

	err := testRootCmd.Execute()
	return buf.String(), err
}

func createMockAPIServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()

	// Mock status endpoint
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"status":    "running",
			"uptime":    "2h30m",
			"version":   "1.0.0",
			"timestamp": time.Now(),
			"metrics": map[string]interface{}{
				"opportunities_detected":   150,
				"profitable_opportunities": 45,
				"total_profit":             "2.5 ETH",
				"success_rate":             0.85,
				"avg_latency":              "45ms",
			},
			"connections": map[string]interface{}{
				"base_rpc":    "connected",
				"anvil_forks": 3,
				"websocket":   "connected",
				"queue_size":  1250,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(status); err != nil {
			t.Errorf("Failed to encode status: %v", err)
		}
	})

	// Mock override endpoints
	mux.HandleFunc("/api/v1/override/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Read and validate request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Simple validation - ensure it's JSON with command field
		if !strings.Contains(string(body), "command") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"success"}`))
	})

	return httptest.NewServer(mux)
}

func setupTestServerConfig(serverURL string) {
	// Parse the test server URL to get host and port
	// This is a simplified version - in production you'd want more robust parsing
	parts := strings.Split(strings.TrimPrefix(serverURL, "http://"), ":")
	if len(parts) == 2 {
		viper.Set("server.host", parts[0])
		if port := parts[1]; port != "" {
			viper.Set("server.port", port)
		}
	}
}

func BenchmarkStatusCommand(b *testing.B) {
	setupTestEnvironment(&testing.T{})
	server := createMockAPIServer(&testing.T{})
	defer server.Close()
	setupTestServerConfig(server.URL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executeCommand("status")
		if err != nil {
			b.Fatalf("Status command failed: %v", err)
		}
	}
}
