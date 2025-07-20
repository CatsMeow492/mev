package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mev-engine/l2-mev-strategy-engine/internal/app"
	"github.com/mev-engine/l2-mev-strategy-engine/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the MEV engine",
	Long: `Start the MEV engine to begin monitoring the mempool and detecting 
MEV opportunities. The engine will run continuously until stopped.`,
	RunE: runStart,
}

var (
	daemonMode  bool
	profileMode bool
)

func init() {
	rootCmd.AddCommand(startCmd)

	startCmd.Flags().BoolVarP(&daemonMode, "daemon", "d", false, "run in daemon mode (background)")
	startCmd.Flags().BoolVar(&profileMode, "profile", false, "enable CPU and memory profiling")
	startCmd.Flags().String("bind", "", "bind address for API server (overrides config)")
	startCmd.Flags().Int("port", 0, "port for API server (overrides config)")

	viper.BindPFlag("daemon", startCmd.Flags().Lookup("daemon"))
	viper.BindPFlag("profile", startCmd.Flags().Lookup("profile"))
	viper.BindPFlag("server.host", startCmd.Flags().Lookup("bind"))
	viper.BindPFlag("server.port", startCmd.Flags().Lookup("port"))
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("🚀 Starting MEV Strategy Engine...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if viper.GetBool("debug") {
		fmt.Printf("Configuration loaded: %+v\n", cfg)
	}

	// Create application with dependency injection
	app := fx.New(
		fx.Provide(
			func() *config.Config { return cfg },
		),
		app.Module,
		fx.Invoke(func(lifecycle fx.Lifecycle, app *app.Application) {
			lifecycle.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go app.Start(ctx)
					return nil
				},
				OnStop: func(ctx context.Context) error {
					return app.Stop(ctx)
				},
			})
		}),
	)

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n🛑 Shutdown signal received, stopping engine...")
		cancel()
	}()

	// Start the application
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	// Wait for shutdown
	<-ctx.Done()

	if err := app.Stop(ctx); err != nil {
		fmt.Printf("⚠️  Error during shutdown: %v\n", err)
	}

	fmt.Println("✅ MEV Engine stopped successfully")
	return nil
}
