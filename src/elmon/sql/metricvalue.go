package sql

import (
	"context"
	"database/sql"
	"elmon/logger"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ExecuteMetricValueGetScript executes an SQL script with a specified timeout
// The function strictly checks that the query returns exactly one row
// containing exactly one column of type JSONB or JSON
func ExecuteMetricValueGetScript(db *sql.DB, script string, timeout time.Duration) (json.RawMessage, error) {
	// 1. Create a context with the timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // Important: release context resources upon completion

	// 2. Execute the query with context to get the Rows object
	rows, err := db.QueryContext(ctx, script)
	if err != nil {
		// Handle timeout error
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("query timed out after %s: %w", timeout, ctx.Err())
		}
		return nil, fmt.Errorf("failed to execute script: %w", err)
	}
	defer rows.Close() // Close Rows after finishing

	// 3. Metadata check: column count and type
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	// 3a. Check column count
	if len(columnTypes) != 1 {
		return nil, fmt.Errorf("expected 1 column, but got %d columns", len(columnTypes))
	}

	// 3b. Check column type (PostgreSQL type name for JSONB is "jsonb")
	typeName := strings.ToLower(columnTypes[0].DatabaseTypeName())
	if typeName != "jsonb" && typeName != "json" {
		return nil, fmt.Errorf("expected column type 'jsonb' or 'json', but got '%s'", typeName)
	}

	// 4. Check for and retrieve the single row
	if !rows.Next() {
		// Check if the query returned at least one row
		if rows.Err() != nil {
			return nil, fmt.Errorf("error during iteration (zero rows): %w", rows.Err())
		}
		// If there are no rows, but no errors either
		return nil, nil // sql.ErrNoRows-like behavior
	}

	var jsonbResult []byte
	// 4b. Scan the single column
	if err := rows.Scan(&jsonbResult); err != nil {
		return nil, fmt.Errorf("failed to scan result into JSON: %w", err)
	}

	// 5. Strict check for extra rows
	if rows.Next() {
		return nil, fmt.Errorf("expected exactly 1 row, but the query returned more than 1 row")
	}

	// 6. Check for errors after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error after iteration: %w", err)
	}

	// 7. Return the result
	return json.RawMessage(jsonbResult), nil
}

// InsertMetricValue inserts metric record into metric_value table
func InsertMetricValue(log *logger.Logger, db *sql.DB, metricId int, serverId int, value json.RawMessage) error {
	// Check for initialized connection
	if db == nil {
		err := fmt.Errorf("database connection (DB) is nil. Cannot insert metric: serverId=%d, metricId=%d", serverId, metricId)
		log.Error(err, "Failed to insert metric")
		return err
	}

	// SQL query for insertion
	const insertSQL = `
		INSERT INTO metric_value (time, server_id, metric_id, metric_value)
		VALUES (NOW(), $1, $2, $3);
	`

	// Execute query
	_, err := db.Exec(insertSQL, serverId, metricId, value)

	if err != nil {
		log.Error(err, fmt.Sprintf("failed to insert metric: serverId=%d, metricId=%d", serverId, metricId))
		return err
	}

	return nil
}