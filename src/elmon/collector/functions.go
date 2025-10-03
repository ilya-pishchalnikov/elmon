package collector

import (
	"context"
	"elmon/sql"
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
		// TODO: Add logic for go_func when implemented
		fallthrough
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