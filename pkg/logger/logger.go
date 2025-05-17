package logger

import (
	"log"
	"os"
)

var Logger = log.New(os.Stderr, "[beemflow] ", log.LstdFlags)
