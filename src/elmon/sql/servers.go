// File: servers.go
package sql

import (
	"database/sql"
	"elmon/logger"
	"fmt"
)

// SaveServerToMetricsDb now accepts local ServerInfo type
func SaveServerToMetricsDb(log *logger.Logger, server *ServerInfo, metricsDb *sql.DB) error {
	query := `
		INSERT INTO server (environment_name, name, host, port, timezone, ssl_mode, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, true)
		ON CONFLICT (name) DO UPDATE SET
			host = excluded.host, port = excluded.port, environment_name = excluded.environment_name,
			timezone = excluded.timezone, ssl_mode = excluded.ssl_mode
		RETURNING server_id;`

	var serverID int
	err := metricsDb.QueryRow(query,
		server.Environment, server.Name, server.Host, server.Port,
		"UTC", server.SslMode,
	).Scan(&serverID)

	if err != nil {
		log.Error(err, fmt.Sprintf("failed to insert/update server record for server %s", server.Name))
		return err
	}

	// Save obtained ID back to structure
	server.ID = &serverID
	return nil
}

// SaveAllServersToMetricsDb now accepts slice of local ServerInfo
func SaveAllServersToMetricsDb(log *logger.Logger, servers []*ServerInfo, metricsDb *sql.DB) error {
	for _, server := range servers {
		err := SaveServerToMetricsDb(log, server, metricsDb)
		if err != nil {
			// Error already logged inside SaveServerToMetricsDb
			return fmt.Errorf("failed to save server '%s' to metrics db: %w", server.Name, err)
		}
	}
	return nil
}