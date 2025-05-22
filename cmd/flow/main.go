package main

import (
	"os"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env as early as possible!
	_ = godotenv.Load()

	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
