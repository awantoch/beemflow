package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
)

var (
	userLogger      = log.New(os.Stdout, "", 0)
	internalLogger  = log.New(os.Stderr, "[beemflow] ", log.LstdFlags)
	loggerMode      = "production"
	loggerModeMutex sync.RWMutex
)

func User(format string, v ...any) {
	userLogger.Printf(format, v...)
}

func Info(format string, v ...any) {
	internalLogger.Printf("[INFO] "+format, v...)
}

func Warn(format string, v ...any) {
	internalLogger.Printf("[WARN] "+format, v...)
}

func Error(format string, v ...any) {
	internalLogger.Printf("[ERROR] "+format, v...)
}

func Debug(format string, v ...any) {
	if os.Getenv("BEEMFLOW_DEBUG") != "" || getMode() == "debug" {
		internalLogger.Printf("[DEBUG] "+format, v...)
	}
}

func SetUserOutput(w io.Writer) {
	userLogger.SetOutput(w)
}

func SetInternalOutput(w io.Writer) {
	internalLogger.SetOutput(w)
}

func SetMode(mode string) {
	loggerModeMutex.Lock()
	defer loggerModeMutex.Unlock()
	loggerMode = mode
}

func getMode() string {
	loggerModeMutex.RLock()
	defer loggerModeMutex.RUnlock()
	return loggerMode
}

// Errorf logs the error message and returns it as an error value.
func Errorf(format string, v ...any) error {
	err := fmt.Errorf(format, v...)
	internalLogger.Printf("[ERROR] %s", err)
	return err
}
