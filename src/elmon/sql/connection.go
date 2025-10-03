package sql

import (
	"database/sql"
	"elmon/logger"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Connect now accepts local ConnectionParams type and doesn't depend on config
func Connect(log *logger.Logger, params ConnectionParams) (*sql.DB, error) {

	if params.SslMode == "" {
		params.SslMode = "disable"
	}

	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		params.Host, params.Port, params.User, params.Password, params.DbName, params.SslMode)

	connection, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Error(err, "error while opening database connection")
		return nil, err
	}

	connection.SetMaxOpenConns(params.MaxOpenConnections)
	connection.SetMaxIdleConns(params.MaxIdleConnections)
	connection.SetConnMaxLifetime(time.Duration(params.ConnectionMaxLifetime) * time.Second)
	connection.SetConnMaxIdleTime(time.Duration(params.ConnectionMaxIdleTime) * time.Second)

	// Test connection
	if err := connection.Ping(); err != nil {
		log.Error(err, "error pinging database")
		connection.Close() // Close connection if ping fails
		return nil, err
	}

	return connection, nil
}

// ConnectAll now accepts slice of local ConnectionParams
func ConnectAll(log *logger.Logger, serverParams []ConnectionParams) (map[string]*sql.DB, error) {
	connections := make(map[string]*sql.DB)
	for _, params := range serverParams {
		serverName := params.Name
		conn, err := Connect(log, params)
		if err != nil {
			// In case of error, close all already opened connections
			for _, c := range connections {
				c.Close()
			}
			return nil, fmt.Errorf("failed to connect to server %s: %w", serverName, err)
		}
		connections[serverName] = conn
		log.Info("Successfully connected", "server", serverName)
	}

	return connections, nil
}