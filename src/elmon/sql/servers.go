package sql

import (
	"database/sql"
	"elmon/config"
	"elmon/logger"
	"fmt"
)

// inserts or updates a server record to metrics DB
func SaveServerToMetricsDb(log *logger.Logger, config *config.DbConnectionConfig, metricsDb *sql.DB) error {
	// The UPSERT statement: INSERT ON CONFLICT DO UPDATE
	// If the record exists, we update properties.
	query := `
		insert into server (
			environment_name, 
			name, 
			host, 
			port, 
			host_auth_method, 
			timezone, 
			ssl_mode, 
			is_active
		) values (
			$1, $2, $3, $4, $5, $6, $7, true
		) on conflict (name) do update set
			host             = excluded.host,
			port             = excluded.port,
			environment_name = excluded.environment_name,
			host_auth_method = excluded.host_auth_method,
			timezone         = excluded.timezone,
			ssl_mode         = excluded.ssl_mode
		returning server_id; -- Return the ID of the inserted/updated record
	`
    var serverID int
	
	// Execute the query
	err := metricsDb.QueryRow(query,
		config.Environment,
		config.Name,
		config.Host,
		config.Port,
		config.HostAuthMethod,
		"UTC",
		config.SslMode,
	).Scan(&serverID)

	if err != nil {
		log.Error(err, fmt.Sprintf("failed to insert/update server record for server %s", config.Name))
		return err
	}

	config.SqlServerId = &serverID

	return nil
}


// inserts or updates all servers records to metrics DB
func SaveAllServersToMetricsDb (log *logger.Logger, config *config.ServerMetricMap, metricsDb *sql.DB) error {
	for serverIndex := range config.Servers {
		serverConfig := config.Servers[serverIndex].Config;

		err := SaveServerToMetricsDb(log , serverConfig, metricsDb)		
		if err != nil {
			log.Error(err, fmt.Sprintf("failed to insert/update server record for server %s", serverConfig.Name))
			return err
		}
	}

	return nil
}