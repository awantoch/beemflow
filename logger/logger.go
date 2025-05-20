package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	userLogger      *log.Logger
	userWriter      io.Writer = os.Stdout
	internalLogger  *zap.SugaredLogger
	loggerMode      = "production"
	loggerModeMutex sync.RWMutex
)

func init() {
	userLogger = log.New(userWriter, "", 0)
	initLoggers()
}

func initLoggers() {
	// Internal logger: to stderr, with levels and debug support
	internalCfg := zap.NewProductionConfig()
	internalCfg.OutputPaths = []string{"stderr"}
	internalCfg.Encoding = "console"
	internalCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if os.Getenv("BEEMFLOW_DEBUG") != "" || getMode() == "debug" {
		internalCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		internalCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	if l, err := internalCfg.Build(); err == nil {
		internalLogger = l.Sugar()
	}
}

func User(format string, v ...any) {
	if userLogger != nil {
		userLogger.Printf(format, v...)
	}
}

func Info(format string, v ...any) {
	if internalLogger != nil {
		internalLogger.Infof(format, v...)
	}
}

func Warn(format string, v ...any) {
	if internalLogger != nil {
		internalLogger.Warnf(format, v...)
	}
}

func Error(format string, v ...any) {
	if internalLogger != nil {
		internalLogger.Errorf(format, v...)
	}
}

func Debug(format string, v ...any) {
	if internalLogger != nil {
		internalLogger.Debugf(format, v...)
	}
}

func SetUserOutput(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	userWriter = w
	userLogger = log.New(userWriter, "", 0)
}

func SetInternalOutput(w io.Writer) {
	if w == nil {
		w = os.Stderr
	}
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		zapcore.AddSync(w),
		zapcore.DebugLevel, // Always allow debug for test capture
	)
	logger := zap.New(core)
	internalLogger = logger.Sugar()
}

func SetMode(mode string) {
	loggerModeMutex.Lock()
	defer loggerModeMutex.Unlock()
	loggerMode = mode
	initLoggers() // re-init to update debug level
}

func getMode() string {
	loggerModeMutex.RLock()
	defer loggerModeMutex.RUnlock()
	return loggerMode
}

// Errorf logs the error message and returns it as an error value.
func Errorf(format string, v ...any) error {
	err := fmt.Errorf(format, v...)
	if internalLogger != nil {
		internalLogger.Errorf("%s", err)
	}
	return err
}

type LoggerWriter struct {
	Fn     func(string, ...any)
	Prefix string
}

func (w *LoggerWriter) Write(p []byte) (n int, err error) {
	lines := strings.Split(string(p), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			if w.Prefix != "" {
				w.Fn("%s%s", w.Prefix, line)
			} else {
				w.Fn("%s", line)
			}
		}
	}
	return len(p), nil
}
