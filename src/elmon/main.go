package main

import (
	"elmon/collector"
	"elmon/config"
	"elmon/grafana"
	"elmon/logger"
	"elmon/sql"
	stdlog "log"
	"log/slog"
	"os"
)

func main() {
	// 1. Load configuration
	appConfig, err := config.Load("config.yaml")
	if err != nil {
		stdlog.Fatalf("FATAL: Failed to load configuration: %v", err)
	}

	// 2. Initialize logger
	log, err := logger.NewByConfig(logger.Config{
		Level:    appConfig.Log.Level,
		Format:   appConfig.Log.Format,
		FileName: appConfig.Log.File,
	})
	if err != nil {
		stdlog.Fatalf("FATAL: Failed to initialize logger: %v", err)
	}
	slog.SetDefault(log.Logger)
	log.Info("Logger started")

	// 3. Connect to metrics database
	metricsDBParams := sql.ConnectionParams{
		Host:                  appConfig.MetricsDB.Host,
		Port:                  appConfig.MetricsDB.Port,
		User:                  appConfig.MetricsDB.User,
		Password:              appConfig.MetricsDB.Password,
		DbName:                appConfig.MetricsDB.DbName,
		SslMode:               appConfig.MetricsDB.SslMode,
		MaxOpenConnections:    appConfig.MetricsDB.MaxOpenConnections,
		MaxIdleConnections:    appConfig.MetricsDB.MaxIdleConnections,
		ConnectionMaxLifetime: appConfig.MetricsDB.ConnectionMaxLifetime,
		ConnectionMaxIdleTime: appConfig.MetricsDB.ConnectionMaxIdleTime,
	}

	db, err := sql.Connect(log, metricsDBParams)
	if err != nil {
		log.Error(err, "error connecting to metrics database server")
		stdlog.Fatalf("Fatal error connecting to metrics SQL server: %v", err)
	}
	defer db.Close()
	log.Info("Metrics database server connected")

	// 4. Execute database migrations
	sqlBytes, err := os.ReadFile("sql/script/init.sql")
	if err != nil {
		log.Error(err, "error opening initial SQL script file")
		stdlog.Fatalf("Fatal error: %v", err)
	}
	if _, err = db.Exec(string(sqlBytes)); err != nil {
		log.Error(err, "failed to execute initial SQL script")
		stdlog.Fatalf("Fatal error: %v", err)
	}
	log.Info("Initial SQL script executed successfully")

	// Initialize Grafana client
	grafanaParams := grafana.ClientParams{
		URL:     appConfig.Grafana.Url,
		Token:   appConfig.Grafana.Token,
		Timeout: appConfig.Grafana.Timeout,
		Retries: 10,
		RetryDelay: 5, // seconds
	}
	grafanaClient := grafana.NewClient(grafanaParams)

	// Check Grafana connection status
	response, err := grafanaClient.Health(log)
	if err != nil {
		log.Error(err, "failed to connect to Grafana")
	} else {
		log.Info("Grafana connected")
	}
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}

	// 5. Save metrics configuration to database
	metricsForDB := &sql.MetricConfigForDB{}
	metricMap := make(map[string]*sql.MetricInfo) // Map for quick metric lookup by name
	for _, group := range appConfig.Metrics.MetricGroups {
		g := &sql.MetricGroupInfo{Name: group.Name, Description: group.Description}
		for _, metric := range group.Metrics {
			m := &sql.MetricInfo{Name: metric.Name, Description: metric.Description}
			g.Metrics = append(g.Metrics, m)
			metricMap[m.Name] = m // Populate the map
		}
		metricsForDB.MetricGroups = append(metricsForDB.MetricGroups, g)
	}
	err = sql.InsertMetricsToDB(log, metricsForDB, db)
	if err != nil {
		log.Error(err, "Error inserting metrics into database")
		stdlog.Fatalf("Fatal error: %v", err)
	}

	// 6. Connect to all monitored database servers
	var allServerParams []sql.ConnectionParams
	serverInfoMap := make(map[string]*sql.ServerInfo) // Map to link server name with server info
	for _, srvCfg := range appConfig.DBServers {
		params := sql.ConnectionParams{
			Name:                  srvCfg.Name,
			Host:                  srvCfg.Host,
			Port:                  srvCfg.Port,
			User:                  srvCfg.User,
			Password:              srvCfg.Password,
			DbName:                srvCfg.DbName,
			SslMode:               srvCfg.SslMode,
			MaxOpenConnections:    srvCfg.MaxOpenConnections,
			MaxIdleConnections:    srvCfg.MaxIdleConnections,
			ConnectionMaxLifetime: srvCfg.ConnectionMaxLifetime,
			ConnectionMaxIdleTime: srvCfg.ConnectionMaxIdleTime,
		}
		allServerParams = append(allServerParams, params)

		info := &sql.ServerInfo{
			Name:        srvCfg.Name,
			Environment: srvCfg.Environment,
			Host:        srvCfg.Host,
			Port:        srvCfg.Port,
			SslMode:     srvCfg.SslMode,
		}
		serverInfoMap[info.Name] = info
	}

	// connections is now map[string]*sql.DB where key is unique server name
	connections, err := sql.ConnectAll(log, allServerParams)
	if err != nil {
		log.Error(err, "Error establishing connections to database servers")
		stdlog.Fatalf("Fatal error: %v", err)
	}
	// Don't forget to close all connections on exit
	defer func() {
		for _, conn := range connections {
			conn.Close()
		}
	}()
	log.Info("Connection to all database servers established")

	// 7. Save server information to metrics database
	var serversToSave []*sql.ServerInfo
	for _, info := range serverInfoMap {
		serversToSave = append(serversToSave, info)
	}
	err = sql.SaveAllServersToMetricsDb(log, serversToSave, db)
	if err != nil {
		log.Error(err, "error saving servers to metrics DB")
		stdlog.Fatalf("Fatal error: %v", err)
	}
	log.Info("Servers loaded to metrics DB")

	log.Info("Assembling metric tasks for the collector...")
	var metricTasks []*collector.MetricTask

	// Create lookup maps for faster access by name
	metricsConfigMap := make(map[string]config.Metric)
	for _, group := range appConfig.Metrics.MetricGroups {
		for _, metric := range group.Metrics {
			metricsConfigMap[metric.Name] = metric
		}
	}

	// Create metric tasks based on server-metric mappings
	for _, mapping := range appConfig.ServerMetricsMap {
		serverInfo, ok := serverInfoMap[mapping.Name]
		if !ok {
			log.Warn("Server from mapping not found in server list, skipping", "server", mapping.Name)
			continue
		}

		targetDBConn, ok := connections[serverInfo.Name]
		if !ok {
			log.Warn("Active connection for server not found, skipping", "server", mapping.Name)
			continue
		}

		for _, metricOverride := range mapping.Metrics {
			metricInfo, ok := metricMap[metricOverride.Name]
			if !ok {
				log.Warn("Metric from mapping not found in metric list, skipping", "metric", metricOverride.Name)
				continue
			}

			baseMetricConfig := metricsConfigMap[metricOverride.Name]

			// Create task combining base and overridden parameters
			task := &collector.MetricTask{
				ServerName:     serverInfo.Name,
				MetricName:     metricInfo.Name,
				ServerID:       *serverInfo.ID,
				MetricID:       metricInfo.DbMetricID,
				CollectionType: baseMetricConfig.CollectionType,
				SQLFile:        baseMetricConfig.SQLFile,
				GoFunction:     baseMetricConfig.GoFunction,
				Interval:       metricOverride.Interval.Duration, // Apply overrides
				MaxRetries:     metricOverride.MaxRetries,
				RetryDelay:     metricOverride.RetryDelay.Duration,
				QueryTimeout:   metricOverride.QueryTimeout.Duration,
				Logger:         log,
				TargetDB:       targetDBConn,
				MetricsDB:      db,
			}

			// Use global/base values if overrides are not provided
			if task.Interval == 0 {
				task.Interval = baseMetricConfig.Interval.Duration
			}
			if task.MaxRetries == 0 {
				task.MaxRetries = baseMetricConfig.MaxRetries
			}
			if task.RetryDelay == 0 {
				task.RetryDelay = baseMetricConfig.RetryDelay.Duration
			}
			if task.QueryTimeout == 0 {
				task.QueryTimeout = baseMetricConfig.QueryTimeout.Duration
			}

			metricTasks = append(metricTasks, task)
		}
	}

	log.Info("Initializing and starting the collector", "task_count", len(metricTasks))
	collector := collector.NewCollector(metricTasks, log)
	if err := collector.Start(); err != nil {
		log.Error(err, "Failed to start the collector")
		stdlog.Fatalf("Fatal error: %v", err)
	}
	defer collector.Stop()

	log.Info("Application is running. Press Ctrl+C to exit.")
	// TODO: Add OS signal handling for graceful shutdown
	select {} // Infinite blocking
}