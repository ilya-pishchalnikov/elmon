package sql

import (
	"database/sql"
	"elmon/config"
	"elmon/logger"
	"encoding/json"
	"fmt"
)

// SQL constants for inserting metric configuration into the database.
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
// into the database if they don't already exist.
func InsertMetricsToDB(log *logger.Logger, config *config.MetricsConfig, db *sql.DB) error {
	transaction, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Defer a rollback in case of error
	defer func() {
		if r := recover(); r != nil {
			log.Error(nil, fmt.Sprintf("panic during DB insertion, attempting rollback: %v", r))
			transaction.Rollback()
			panic(r) // Re-throw panic
		} else if err != nil {
			log.Warn(fmt.Sprintf("DB insertion failed, rolling back: %v", err))
			transaction.Rollback()
		}
	}()

	for groupIndex := range config.MetricGroups {
        group := &config.MetricGroups[groupIndex]
		// 1. Insert or update metric group
		log.Debug(fmt.Sprintf("Inserting/updating metric group: %s", group.Name))
        
		row := transaction.QueryRow(SQLInsertMetricGroup, group.Name, group.Description)

        var groupId int

        if err := row.Scan(&groupId); err != nil {
            log.Error(err, "failed to insert/get metric group ID")
            return fmt.Errorf("failed to insert/get metric group ID for '%s': %w", group.Name, err)
        }

		if err != nil {
			return fmt.Errorf("failed to insert metric group '%s': %w", group.Name, err)
		}

		// 2. Insert or update metrics within the group
		for metricIndex := range group.Metrics {
            metric := &group.Metrics[metricIndex]
			log.Debug(fmt.Sprintf("Inserting/updating metric: %s (Group: %s)", metric.Name, group.Name))
            
            row = transaction.QueryRow(SQLInsertMetric, groupId, metric.Name, metric.Description)

            if err = row.Scan(&metric.DbMetricId); err != nil {
				return fmt.Errorf("failed to insert metric '%s' for group '%s': %w", metric.Name, group.Name, err)
			}
		}
	}

	if err = transaction.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Info("Successfully inserted/updated metric configuration in the database.")
	return nil
}

// InsertMetricValue выполняет вставку записи метрики в таблицу metric_value.
func InsertMetricValue(log *logger.Logger, db *sql.DB, metricId int, serverId int, value json.RawMessage) error{
	// Проверка на инициализацию соединения
	if db == nil {
		err := fmt.Errorf("database connection (DB) is nil. Cannot insert metric: serverId=%d, metricId=%d", serverId, metricId)
		log.Error(err,"Failed to insert metric")
		return err
	}

	// SQL-запрос для вставки
	const insertSQL = `
		INSERT INTO metric_value (time, server_id, metric_id, metric_value)
		VALUES (NOW(), $1, $2, $3);
	`

	// Выполняем запрос
	_, err := db.Exec(insertSQL, serverId, metricId, value)
	
	if err != nil {
		err := fmt.Errorf("failed to insert metric: serverId=%d, metricId=%d", serverId, metricId)
		log.Error(err, "Failed to insert metric")
		return err
	}

	return nil
}