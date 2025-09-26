package main

import (
	"context"
	"elmon/config"
	"elmon/logger"
	stdlog "log"
	"log/slog"
	"time"
)

func main() {
    config.Load("config.yaml");

    var config = config.GetConfig();

    log, err := logger.NewByConfig(*config)
    if err!=nil {
        stdlog.Fatalf("Error while initializing log: %v", err)
    }

    slog.SetDefault(log.Logger)

    log.Info(context.Background(), "Application started", "version", "0.1.0", "start_time", time.Now().Format(time.RFC3339))

}
