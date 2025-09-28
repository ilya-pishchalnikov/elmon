package main

import (
	"elmon/config"
	"elmon/configlog"
	"elmon/grafana"
	"elmon/logger"
	"elmon/sql"
	"fmt"
	stdlog "log"
	"log/slog"
	"os"
)

func main() {
    logconfig, err := configlog.Load("configlog.yaml")
    if err!=nil {        
        stdlog.Fatalf("error while reading log config: %v", err)
    }

    log, err := logger.NewByConfig(*logconfig)
    if err!=nil {
        stdlog.Fatalf("error while initializing log: %v", err)
    }

    slog.SetDefault(log.Logger)

    log.Info("Log started")

    conf, err := config.Load(log, "config.yaml");
    if err!=nil {        
        stdlog.Fatalf("error while reading config: %v", err)
    }

    
    log.Info("Application config loaded")

    db, err := sql.Connect(log, &conf.MetricsDb);
    if err!=nil {
        log.Error(err, "error connecting metrics database server");
        stdlog.Fatalf("error while connecting metrics SQL server: %v", err)
    }
 
    log.Info("Metrics database server connected")

    err = conf.MetricsDb.SqlConnection.Ping()
    if err!=nil {
        log.Error(err, "error connecting metrics database server");
        stdlog.Fatalf("error connecting metrics database server: %v", err)
    }

    // read the  initial sql script from the file
	sqlBytes, err := os.ReadFile("sql/script/init.sql")
	if err != nil {
		log.Error(err, "error opening initial sql script file");
        stdlog.Fatalf("error opening initial sql script file: %v", err)
	}
	sqlScript := string(sqlBytes)

	// execute the initial script
	_, err = db.Exec( sqlScript)
	if err != nil {        
		log.Error(err, "failed to execute sql script");
		stdlog.Fatalf("failed to execute sql script: %v", err)
	}

	log.Info("Initial sql script executed successfully")

    grafanaClient := grafana.NewClient(conf.Grafana)

    response, err := grafanaClient.Health(log)
    if err!=nil {
        log.Error(err, "failed to connect Grafana");
    } else {
        log.Info ("grafana connected")
    }
    defer response.Body.Close()

    loader := config.NewMetricsConfigLoader(".")

    metricsCfg, err := loader.Load(log, "configmetrics.yaml")
	if err != nil {
		log.Error(err, "Error loading metrics confgiruation")
		stdlog.Fatalf("Fatal error loading metrics configuration: %v", err)
	}

    log.Info(fmt.Sprintf("Loaded metrics config version '%s'", metricsCfg.Version));  
    
    err = loader.InsertMetricsToDB(log, metricsCfg, db)
    if err != nil {
		log.Error(err, "Error loading metrics to database")
		stdlog.Fatalf("Fatal error loading metrics to database: %v", err)
	}

    var servers *config.DbServers

    servers, err = config.LoadDbServers(log, "configservers.yaml");
    if err != nil {
		log.Error(err, "Error loading db servers config")
		stdlog.Fatalf("Fatal error loading db servers config: %v", err)
	}

    err = sql.ConnectAll(log, servers)
    if err != nil {
		log.Error(err, "Error establishing connection to db servers")
		stdlog.Fatalf("Fatal error establishing connection to db servers: %v", err)
	}
    log.Info("Connection to db servers established");  
}
