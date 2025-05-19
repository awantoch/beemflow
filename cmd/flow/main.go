package main

import (
	"github.com/joho/godotenv"
)

func main() {
	// Load .env as early as possible!
	_ = godotenv.Load()
	if err := NewRootCmd().Execute(); err != nil {
		exit(1)
	}
}
