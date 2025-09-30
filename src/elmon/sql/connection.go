package sql

import (
	"database/sql"
	"elmon/config"
	"elmon/logger"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Connect establishes a connection to a single database server using the provided configuration.
func Connect(log *logger.Logger, config *config.DbConnectionConfig) (*sql.DB, error) {
	// Basic connection parameters
	connectionString := fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		config.Host, config.Port, config.User, config.DbName, config.SslMode)

	// Add password parameter only for password-based authentication methods
	// HostAuthMethod is used to determine if the password field should be included in the connection string.
	// Methods like 'certificate', 'gss', 'sspi' typically do not require 'password'.
	switch config.HostAuthMethod {
	case "password", "md5", "scram-sha-256":
		// Append password only if required
		connectionString += fmt.Sprintf(" password=%s", config.Password)
	// For other methods (e.g., "certificate", "gss", "sspi"), the password is not included.
	}

	connection, err := sql.Open("postgres", connectionString)
	if err != nil {
		log.Error(err, "error while open database")
		return connection, err
	}

	// Set connection pool parameters
	connection.SetMaxOpenConns(config.MaxOpenConnections)
	connection.SetMaxIdleConns(config.MaxIdleConnections)
	connection.SetConnMaxLifetime(time.Duration(config.ConnectionMaxLifetime) * time.Second)
	connection.SetConnMaxIdleTime(time.Duration(config.ConnectionMaxIdleTime) * time.Second)

	// Store the active connection object in the configuration structure
	config.SqlConnection = connection

	return connection, err
}

// ConnectAll iterates through all configured database servers and establishes a connection for each one.
func ConnectAll(log *logger.Logger, config *config.DbServers) error {
	for _, server := range config.Servers {
		_, err := Connect(log, &server)
		if err != nil {
			log.Error(err, "error while open database")
			return err
		}
	}

	return nil
}