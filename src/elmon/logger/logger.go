package logger

import (
	"context"
	"elmon/config"
	"log/slog"
	"os"
	"runtime"
	"time"
)

type Logger struct {
    *slog.Logger
}

// New creates a new logger
func New(level slog.Level, isJSON bool, logFileName string) (*Logger, error) {
    opts := &slog.HandlerOptions{
        Level: level,
    }

	writer := os.Stdout;

	if logFileName!="" {
		logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		defer logFile.Close()

		writer = logFile;
	}
    
    var handler slog.Handler
    if isJSON {
        handler = slog.NewJSONHandler(writer, opts)
    } else {
        handler = slog.NewTextHandler(writer, opts)
    }
    
    return &Logger{Logger: slog.New(handler)} , nil
}

func NewByConfig(config config.Config) (*Logger, error) {
	var logFileName string = config.Log.FileName;

	var level slog.Level = parseLevel(config.Log.Level)

	var isJson bool = config.Log.Format == "json";

	logger, err := New (level, isJson, logFileName)

	return logger, err
}

// WithContext adds context to the logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
    if ctx == nil {
        return l
    }
    
    // Extract values from context
    if requestID, ok := ctx.Value("request_id").(string); ok {
        return &Logger{l.With("request_id", requestID)}
    }
    
    return l
}

// Debug with additional information
func (l *Logger) Debug(ctx context.Context, msg string, args ...any) {
    l.WithContext(ctx).log(ctx, slog.LevelDebug, msg, args...)
}

// Info with additional information
func (l *Logger) Info(ctx context.Context, msg string, args ...any) {
    l.WithContext(ctx).log(ctx, slog.LevelInfo, msg, args...)
}

// Warning with additional information
func (l *Logger) Warn(ctx context.Context, msg string, args ...any) {
    l.WithContext(ctx).log(ctx, slog.LevelWarn, msg, args...)
}

// Error with error information
func (l *Logger) Error(ctx context.Context, err error, msg string, args ...any) {
    args = append(args, "error", err.Error())
    l.WithContext(ctx).log(ctx, slog.LevelError, msg, args...)
}

// log internal logging method
func (l *Logger) log(ctx context.Context, level slog.Level, msg string, args ...any) {
    if !l.Enabled(ctx, level) {
        return
    }
    
    var pcs [1]uintptr
    runtime.Callers(3, pcs[:]) // skip log, public function, and this function
    
    r := slog.NewRecord(time.Now(), level, msg, pcs[0])
    r.Add(args...)
    
    _ = l.Handler().Handle(ctx, r)
}

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