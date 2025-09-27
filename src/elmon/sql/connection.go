package sql

import (
	"database/sql"
	"elmon/config"
	"elmon/logger"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

func Connect (log logger.Logger, config config.DbConnectionConfig) (*sql.DB, error) {
	connectionString := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
        config.Host, config.Port, config.User, 
        config.Password, config.DbName, config.SslMode)

	connection, err := sql.Open("postgres", connectionString);
	if err!=nil {
		log.Error(err, "error while open database")
		return connection, err
	}

	connection.SetMaxOpenConns(config.MaxOpenConnections)
	connection.SetMaxIdleConns(config.MaxIdleConnections)
	connection.SetConnMaxLifetime(time.Duration(config.ConnectionMaxLifetime) * time.Second)
	connection.SetConnMaxIdleTime(time.Duration(config.ConnectionMaxIdleTime) * time.Second)

	return connection, err
}

