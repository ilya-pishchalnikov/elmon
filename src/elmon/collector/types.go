package collector

import (
	"database/sql"
	"elmon/logger"
	"time"
)

// MetricTask represents a single metric collection task for a specific server
// This structure contains all necessary information for scheduler and executor function
type MetricTask struct {
	// Identifiers
	ServerName string
	MetricName string
	ServerID   int
	MetricID   int

	// Execution parameters
	CollectionType string // "sql" or "go_func"
	SQLFile        string // File path for "sql" type
	GoFunction     string // Function name for "go_func" type

	// Scheduler parameters
	Interval   time.Duration
	MaxRetries int
	RetryDelay time.Duration

	// Query parameters
	QueryTimeout time.Duration

	// Runtime dependencies
	Logger    *logger.Logger
	TargetDB  *sql.DB // Connection to monitored server
	MetricsDB *sql.DB // Connection to metrics storage database
}