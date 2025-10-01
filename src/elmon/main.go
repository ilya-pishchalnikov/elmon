package main

import (
	"elmon/collector"
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
	// Load logging configuration
	logconfig, err := configlog.Load("configlog.yaml")
	if err != nil {
		stdlog.Fatalf("Fatal error reading log configuration: %v", err)
	}

	// Initialize application logger
	log, err := logger.NewByConfig(*logconfig)
	if err != nil {
		stdlog.Fatalf("Fatal error initializing logger: %v", err)
	}

	slog.SetDefault(log.Logger)

	log.Info("Logger started")

	// Load main application configuration
	conf, err := config.Load(log, "config.yaml")
	if err != nil {
		stdlog.Fatalf("Fatal error reading application configuration: %v", err)
	}

	log.Info("Application configuration loaded")

	// Connect to the metrics database server
	db, err := sql.Connect(log, &conf.MetricsDb)
	if err != nil {
		log.Error(err, "error connecting to metrics database server")
		stdlog.Fatalf("Fatal error connecting to metrics SQL server: %v", err)
	}

	log.Info("Metrics database server connected")

	// Test the database connection
	err = conf.MetricsDb.SqlConnection.Ping()
	if err != nil {
		log.Error(err, "error pinging metrics database server")
		stdlog.Fatalf("Fatal error connecting to metrics database server: %v", err)
	}

	// Read the initial SQL script from the file
	sqlBytes, err := os.ReadFile("sql/script/init.sql")
	if err != nil {
		log.Error(err, "error opening initial SQL script file")
		stdlog.Fatalf("Fatal error opening initial SQL script file: %v", err)
	}
	sqlScript := string(sqlBytes)

	// Execute the initial script (e.g., table creation, dictionary insertion)
	_, err = db.Exec(sqlScript)
	if err != nil {
		log.Error(err, "failed to execute initial SQL script")
		stdlog.Fatalf("Fatal error executing initial SQL script: %v", err)
	}

	log.Info("Initial SQL script executed successfully")

	// Initialize Grafana client
	grafanaClient := grafana.NewClient(conf.Grafana)

	// Check Grafana connection status
	response, err := grafanaClient.Health(log)
	if err != nil {
		log.Error(err, "failed to connect to Grafana")
	} else {
		log.Info("Grafana connected")
	}
	// Safely close the HTTP response body
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	// Initialize metrics config loader
	loader := config.NewMetricsConfigLoader(".")

	// Load metrics configuration
	metricsCfg, err := loader.Load(log, "configmetrics.yaml")
	if err != nil {
		log.Error(err, "Error loading metrics configuration")
		stdlog.Fatalf("Fatal error loading metrics configuration: %v", err)
	}

	log.Info(fmt.Sprintf("Loaded metrics config version '%s'", metricsCfg.Version))

	// Insert metric groups and metrics into the database
	err = sql.InsertMetricsToDB(log, metricsCfg, db)
	if err != nil {
		log.Error(err, "Error inserting metrics into database")
		stdlog.Fatalf("Fatal error inserting metrics into database: %v", err)
	}

	// Load database servers configuration
	var servers *config.DbServers

	servers, err = config.LoadDbServers(log, "configservers.yaml")
	if err != nil {
		log.Error(err, "Error loading database servers configuration")
		stdlog.Fatalf("Fatal error loading database servers configuration: %v", err)
	}

	// Establish connections to all configured DB servers
	err = sql.ConnectAll(log, servers)
	if err != nil {
		log.Error(err, "Error establishing connections to database servers")
		stdlog.Fatalf("Fatal error establishing connections to database servers: %v", err)
	}
	log.Info("Connection to database servers established")

	var serversMetrics *config.ServerMetricMap

	// Load server-metric assignments
	serversMetrics, err = serversMetrics.Load(log, "configserversmetrics.yaml", *servers, *metricsCfg)
	if err != nil {
		log.Error(err, "error loading server-metric assignments")
		stdlog.Fatalf("Fatal error loading server-metric assignments: %v", err)
	}
	log.Info("Server-metric assignments loaded")

	err = sql.SaveAllServersToMetricsDb(log, serversMetrics, db)
	if err != nil {
		log.Error(err, "error loading servers to metrics DB")
		stdlog.Fatalf("Fatal error loading servers to metrics DB: %v", err)
	}
	log.Info("Servers loaded to metrics DB")

	 fmt.Println("--------------------------------------------------------------------------------------")

	// 2. Вызов CallMethodAndReturnError
	fmt.Println("--- STARTING DYNAMIC CALL ---")
	err = collector.CallMethod(
		collector.CollectFunctions{}, // service
		"ExecuteSql",       // methodName
		log,           // arg 1: *Logger
		serversMetrics.Servers[0].Config,           // arg 2: *DbConnectionConfig
		&serversMetrics.Servers[0].Metrics[0],             // arg 3: *MetricForMapping
		db,          // arg 4: *dbsql.DB
	)
	fmt.Println("--- ENDING DYNAMIC CALL ---")

	if err != nil {
		fmt.Printf("Dynamic call failed: %v\n", err)
	} else {
		fmt.Println("Dynamic call completed successfully (simulated).")
	}

}