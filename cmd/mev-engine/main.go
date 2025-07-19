package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mev-engine/l2-mev-strategy-engine/internal/app"
	"github.com/mev-engine/l2-mev-strategy-engine/internal/config"
	"go.uber.org/fx"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
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

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		cancel()
	}()

	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}

	if err := app.Stop(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}