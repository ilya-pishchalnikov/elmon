package sql

import "database/sql"

// ConnectionParams defines parameters required exclusively for database connection
type ConnectionParams struct {
	Name                  string
	Host                  string
	Port                  int
	User                  string
	Password              string
	DbName                string
	SslMode               string
	MaxOpenConnections    int
	MaxIdleConnections    int
	ConnectionMaxLifetime int // in seconds
	ConnectionMaxIdleTime int // in seconds
}

// ServerInfo contains complete server information for saving to metrics DB
type ServerInfo struct {
	Name        string
	Environment string
	Host        string
	Port        int
	SslMode     string
	// This field is used to store ID after saving to database
	ID *int
}

// MetricInfo represents a metric for saving to database
type MetricInfo struct {
	Name        string
	Description string
	// This field is used to store ID after saving to database
	DbMetricID int
}

// MetricGroupInfo represents a metric group for saving to database
type MetricGroupInfo struct {
	Name        string
	Description string
	Metrics     []*MetricInfo
}

// MetricConfigForDB represents complete metric configuration for saving to database
type MetricConfigForDB struct {
	MetricGroups []*MetricGroupInfo
}

// ServerMetricMappingForDB is used to link servers with metrics in database
type ServerMetricMappingForDB struct {
	ServerConfig  *ServerInfo
	// SqlConnection is here to avoid passing it separately
	SqlConnection *sql.DB
}