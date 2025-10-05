package collector

import (
	"context"
	"elmon/sql"
	"encoding/json"
	"fmt"
	"os"
)

// ProcessMetric - implementation of scheduler.TaskFunc
func ProcessMetric(ctx context.Context, taskPayload interface{}) error {
	// Type assertion from interface{} to MetricTask
	task, ok := taskPayload.(*MetricTask)
	if !ok {
		return fmt.Errorf("invalid task payload type: expected *MetricTask")
	}

	// Select collection method based on CollectionType
	switch task.CollectionType {
	case "sql":
		return executeSQLMetric(task)
	case "go_func":
		return executeGoFuncMetric(task) // <--- Updated to call the new function
	default:
		err := fmt.Errorf("collection type '%s' not implemented yet for metric '%s'",
			task.CollectionType, task.MetricName)
		task.Logger.Error(err, "Metric collection error")
		return err // Return error to prevent scheduler retries
	}
}

// executeSQLMetric performs SQL metric collection
func executeSQLMetric(task *MetricTask) error {
	log := task.Logger
	sqlScript, err := os.ReadFile(task.SQLFile)
	if err != nil {
		log.Error(err, "Error reading SQL file", "metric", task.MetricName, "file", task.SQLFile)
		return err
	}

	value, err := sql.ExecuteMetricValueGetScript(task.TargetDB, string(sqlScript), task.QueryTimeout)
	if err != nil {
		log.Error(err, "Error querying metric from target server", "metric", task.MetricName, "server", task.ServerName)
		return err
	}

	// Skip NULL values
	if value != nil {
		err = sql.InsertMetricValue(log, task.MetricsDB, task.MetricID, task.ServerID, value)
		if err != nil {
			log.Error(err, "Error inserting metric value into metrics DB", "metric", task.MetricName)
			return err
		}
	}

	return nil
}

// executeGoFuncMetric selects and executes the appropriate Go function metric collector
func executeGoFuncMetric(task *MetricTask) error {
	switch task.GoFunction {
	case "collectPostgresUptime":
		return collectPostgresUptime(task)
	default:
		err := fmt.Errorf("go function '%s' not implemented yet for metric '%s'",
			task.GoFunction, task.MetricName)
		task.Logger.Error(err, "Metric collection error")
		return err
	}
}

// collectPostgresUptime executes the PostgreSQL uptime query.
// It inserts the result or a default 0 uptime if the connection/query fails.
func collectPostgresUptime(task *MetricTask) error {
	log := task.Logger
	
	// --- 1. Define SQL for Uptime ---
	// This query calculates the difference in seconds between the current time and the postmaster start time.
	const uptimeSQL = `
		SELECT jsonb_build_object('value', EXTRACT(EPOCH FROM (NOW() - pg_postmaster_start_time()))) AS metric_value;
	`
	
	// --- 2. Attempt to query the actual Uptime ---
	value, err := sql.ExecuteMetricValueGetScript(task.TargetDB, uptimeSQL, task.QueryTimeout)

	// --- 3. Handle connection/query failure (The main requirement) ---
	if err != nil {
		log.Warn("Failed to collect actual PostgreSQL uptime. Inserting 0 as uptime value.", 
			"server", task.ServerName, 
			"metric", task.MetricName, 
			"error", err)

		// Create a JSON object with uptime 0. This structure should match the successful SQL query's output.
		zeroUptimeValue := json.RawMessage(`{"value": 0}`)
		
		// Insert the zero uptime value into the metrics database
		insertErr := sql.InsertMetricValue(log, task.MetricsDB, task.MetricID, task.ServerID, zeroUptimeValue)
		if insertErr != nil {
			// This is a critical failure: couldn't insert 0 value.
			log.Error(insertErr, "CRITICAL: Failed to insert zero uptime value after connection error", 
				"server", task.ServerName, 
				"metric", task.MetricName)
			return insertErr
		}
		
		// Successfully inserted 0 value. The scheduler should NOT retry this (since we recorded the status).
		return nil 
	}

	// --- 4. Handle successful query ---
	// If value is nil, it means the query returned 0 rows (handled in ExecuteMetricValueGetScript, but unlikely here).
	if value != nil {
		err = sql.InsertMetricValue(log, task.MetricsDB, task.MetricID, task.ServerID, value)
		if err != nil {
			log.Error(err, "Error inserting actual uptime value into metrics DB", "metric", task.MetricName)
			return err
		}
	}
	
	return nil
}