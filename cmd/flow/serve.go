package main

import (
	"fmt"
	"os"

	beemhttp "github.com/awantoch/beemflow/http"
	"github.com/spf13/cobra"
)

// newServeCmd creates the 'serve' subcommand.
func newServeCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the BeemFlow runtime HTTP server",
		Run: func(cmd *cobra.Command, args []string) {
			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("Starting BeemFlow HTTP server on %s...\n", addr)
			// If stdout is not a terminal (e.g., piped in tests), skip starting the server to avoid blocking
			if fi, statErr := os.Stdout.Stat(); statErr == nil && fi.Mode()&os.ModeCharDevice == 0 {
				return
			}
			err := beemhttp.StartServer(addr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
				os.Exit(1)
			}
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8080, "Port to listen on (default 8080)")
	return cmd
}
