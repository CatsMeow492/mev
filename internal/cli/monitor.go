package cli

import (
	"github.com/mev-engine/l2-mev-strategy-engine/internal/tui"
	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Start terminal-based monitoring interface",
	Long: `Launch an interactive terminal-based UI for real-time monitoring of 
MEV opportunities, system performance, and operational metrics. Use arrow keys 
to navigate, 'q' to quit.`,
	RunE: runMonitor,
}

var (
	refreshRate int
	compactMode bool
)

func init() {
	rootCmd.AddCommand(monitorCmd)

	monitorCmd.Flags().IntVarP(&refreshRate, "refresh", "r", 1000, "refresh rate in milliseconds")
	monitorCmd.Flags().BoolVarP(&compactMode, "compact", "c", false, "compact display mode")
}

func runMonitor(cmd *cobra.Command, args []string) error {
	config := tui.Config{
		RefreshRate: refreshRate,
		CompactMode: compactMode,
		Debug:       cmd.Flag("debug").Value.String() == "true",
	}

	return tui.StartMonitor(config)
}
