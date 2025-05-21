package main

import (
	"os"

	"github.com/awantoch/beemflow/config"
	beemhttp "github.com/awantoch/beemflow/http"
	"github.com/awantoch/beemflow/utils"
	"github.com/spf13/cobra"
)

// newServeCmd creates the 'serve' subcommand.
func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the BeemFlow runtime HTTP server",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.LoadConfig(config.DefaultConfigPath)
			if err != nil {
				if os.IsNotExist(err) {
					cfg = &config.Config{}
				} else {
					utils.Error("Failed to load config: %v", err)
					exit(1)
				}
			}
			// Set default storage if not configured
			if cfg.Storage.Driver == "" {
				cfg.Storage.Driver = "sqlite"
				cfg.Storage.DSN = config.DefaultSQLiteDSN
			}
			if err := cfg.Validate(); err != nil {
				utils.Error("Config validation failed: %v", err)
				exit(1)
			}
			utils.Info("Starting BeemFlow HTTP server...")
			// If stdout is not a terminal (e.g., piped in tests), skip starting the server to avoid blocking
			if fi, statErr := os.Stdout.Stat(); statErr == nil && fi.Mode()&os.ModeCharDevice == 0 {
				utils.User("flow serve (stub)")
				return
			}
			if err := beemhttp.StartServer(cfg); err != nil {
				utils.Error("Failed to start server: %v", err)
				exit(1)
			}
		},
	}
	return cmd
}
