package sql

import (
	"database/sql"
	"elmon/logger"
	"fmt"
)

// SQL constants for inserting metric configuration into the database
const (
	// SQL to insert a metric group name. It uses ON CONFLICT to prevent duplicates
	// and returns the metric_group_id of the existing or newly inserted row.
	SQLInsertMetricGroup = `
		insert into metric_group (metric_group_name, description)
		values ($1, $2)
		on conflict (metric_group_name) do update
		set description = excluded.description
		returning metric_group_id
	`
	// SQL to insert a metric name linked to its group.
	// It uses ON CONFLICT to prevent duplicates and returns the metric_id.
	SQLInsertMetric = `
		insert into metric (metric_group_id, metric_name, description)
		values ($1, $2, $3)
		on conflict (metric_name) do update
		set metric_group_id = excluded.metric_group_id,
		    description = excluded.description
        returning metric_id
	`
)

// InsertMetricsToDB inserts metric groups and metrics from the configuration
// into the database if they don't already exist
func InsertMetricsToDB(log *logger.Logger, config *MetricConfigForDB, db *sql.DB) error {
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			transaction.Rollback()
			panic(r)
		} else if err != nil {
			transaction.Rollback()
		}
	}()

	for _, group := range config.MetricGroups {
		var groupID int
		err = transaction.QueryRow(SQLInsertMetricGroup, group.Name, group.Description).Scan(&groupID)
		if err != nil {
			return fmt.Errorf("failed to insert/get group ID for '%s': %w", group.Name, err)
		}

		for _, metric := range group.Metrics {
			var metricID int
			err = transaction.QueryRow(SQLInsertMetric, groupID, metric.Name, metric.Description).Scan(&metricID)
			if err != nil {
				return fmt.Errorf("failed to insert/get metric ID for '%s': %w", metric.Name, err)
			}
			// Save ID back to structure for future use
			metric.DbMetricID = metricID
		}
	}

	if err = transaction.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info("Successfully inserted/updated metric configuration in the database.")
	return nil
}