package logger

import (
	"io"
	"log"
	"os"
)

var (
	stdLogger = log.New(os.Stderr, "[beemflow] ", log.LstdFlags)
)

func Info(format string, v ...any) {
	stdLogger.Printf("[INFO] "+format, v...)
}

func Warn(format string, v ...any) {
	stdLogger.Printf("[WARN] "+format, v...)
}

func Error(format string, v ...any) {
	stdLogger.Printf("[ERROR] "+format, v...)
}

func Debug(format string, v ...any) {
	if os.Getenv("BEEMFLOW_DEBUG") != "" {
		stdLogger.Printf("[DEBUG] "+format, v...)
	}
}

func SetOutput(w io.Writer) {
	stdLogger.SetOutput(w)
}
