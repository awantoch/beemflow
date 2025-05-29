package utils

import (
	"context"
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

type requestIDKeyType struct{}

var requestIDKey = requestIDKeyType{}

func init() {
	userLogger = log.New(userWriter, "", 0)
	initLoggers("production") // Default mode
}

func initLoggers(mode string) {
	// Internal logger: to stderr, with levels and debug support
	internalCfg := zap.NewProductionConfig()
	internalCfg.OutputPaths = []string{"stderr"}
	internalCfg.Encoding = "console"
	internalCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	if os.Getenv("BEEMFLOW_DEBUG") != "" || mode == "debug" {
		internalCfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	} else {
		internalCfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	l, err := internalCfg.Build()
	if err != nil {
		// Fallback to standard library logger if zap fails
		log.Printf("Failed to initialize zap logger: %v, falling back to standard logger", err)
		internalLogger = nil
		return
	}
	internalLogger = l.Sugar()
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
	initLoggers(mode) // Pass mode directly to avoid deadlock
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

// WithRequestID returns a new context with the given request ID.
func WithRequestID(ctx context.Context, reqID string) context.Context {
	return context.WithValue(ctx, requestIDKey, reqID)
}

// RequestIDFromContext extracts the request ID from context, if present.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(requestIDKey)
	if s, ok := v.(string); ok {
		return s, true
	}
	return "", false
}

// InfoCtx logs an info message with context, including request ID if present.
func InfoCtx(ctx context.Context, msg string, fields ...any) {
	if internalLogger != nil {
		if reqID, ok := RequestIDFromContext(ctx); ok {
			fields = append(fields, "request_id", reqID)
		}
		internalLogger.Infow(msg, fields...)
	}
}

// WarnCtx logs a warning message with context, including request ID if present.
func WarnCtx(ctx context.Context, msg string, fields ...any) {
	if internalLogger != nil {
		if reqID, ok := RequestIDFromContext(ctx); ok {
			fields = append(fields, "request_id", reqID)
		}
		internalLogger.Warnw(msg, fields...)
	}
}

// ErrorCtx logs an error message with context, including request ID if present.
func ErrorCtx(ctx context.Context, msg string, fields ...any) {
	if internalLogger != nil {
		if reqID, ok := RequestIDFromContext(ctx); ok {
			fields = append(fields, "request_id", reqID)
		}
		internalLogger.Errorw(msg, fields...)
	}
}

// DebugCtx logs a debug message with context, including request ID if present.
func DebugCtx(ctx context.Context, msg string, fields ...any) {
	if internalLogger != nil {
		if reqID, ok := RequestIDFromContext(ctx); ok {
			fields = append(fields, "request_id", reqID)
		}
		internalLogger.Debugw(msg, fields...)
	}
}
