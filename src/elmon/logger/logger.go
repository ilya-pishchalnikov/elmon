package logger

import (
	"context"
	"elmon/configlog"
	"log/slog"
	"os"
	"runtime"
	"time"
)

// Logger provides a wrapper around slog.Logger.
type Logger struct {
	*slog.Logger
}

// New creates a new logger instance with specified level, format (JSON/text), and output file.
// If logFileName is empty, output goes to os.Stdout.
// Note: defer logFile.Close() is omitted for production-like long-lived loggers,
// file closure should be handled at application shutdown.
func New(level slog.Level, isJSON bool, logFileName string) (*Logger, error) {
	opts := &slog.HandlerOptions{
		Level: level,
		// AddSource: true, // Uncomment to include file and line number in logs
	}

	writer := os.Stdout

	if logFileName != "" {
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		writer = logFile
	}

	var handler slog.Handler
	if isJSON {
		handler = slog.NewJSONHandler(writer, opts)
	} else {
		handler = slog.NewTextHandler(writer, opts)
	}

	return &Logger{Logger: slog.New(handler)}, nil
}

// NewByConfig creates a new logger instance based on the provided configuration.
func NewByConfig(config configlog.LogConfig) (*Logger, error) {
	logFileName := config.FileName
	level := parseLevel(config.Level)
	isJson := config.Format == "json"

	logger, err := New(level, isJson, logFileName)
	return logger, err
}

// Debug logs a debug-level message with additional key-value pairs.
func (l *Logger) Debug(msg string, args ...any) {
	l.log(slog.LevelDebug, msg, args...)
}

// Info logs an info-level message with additional key-value pairs.
func (l *Logger) Info(msg string, args ...any) {
	l.log(slog.LevelInfo, msg, args...)
}

// Warn logs a warning-level message with additional key-value pairs.
func (l *Logger) Warn(msg string, args ...any) {
	l.log(slog.LevelWarn, msg, args...)
}

// Error logs an error-level message with an error object and additional key-value pairs.
func (l *Logger) Error(err error, msg string, args ...any) {
	args = append(args, "error", err.Error())
	l.log(slog.LevelError, msg, args...)
}

// log is an internal method to handle the actual logging using slog.
// It sets the call frame and passes context.Background() as a placeholder.
func (l *Logger) log(level slog.Level, msg string, args ...any) {
	// context.Background() is used as a placeholder, as public methods do not accept context.
	// It's required by slog.Logger.Enabled and slog.Handler.Handle.
	ctx := context.Background()

	if !l.Enabled(ctx, level) {
		return
	}

	var pcs [1]uintptr
	// Skip 3 frames: runtime.Callers, l.log, and the public method (Debug/Info/Warn/Error).
	runtime.Callers(3, pcs[:]) 

	r := slog.NewRecord(time.Now(), level, msg, pcs[0])
	r.Add(args...)

	_ = l.Handler().Handle(ctx, r)
}

// parseLevel converts a string representation of a log level to slog.Level.
// Defaults to slog.LevelInfo if the string is not recognized.
func parseLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}