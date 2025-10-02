package collector

import (
	"elmon/config"
	"elmon/sql"
	"fmt"
	"os"
)

// ExecuteSql gets the metric value by executing an SQL command
func ExecuteSql (metric *config.ServerMetric) error {
	
	log := metric.Logger;
	sqlScript, err := os.ReadFile(metric.MetricConfig.SQLFile)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error while read sql file of metric '%s' for server '%s'", metric.Name, metric.ServerConfig.Name))
		return err
	}

	value, err := sql.ExecuteMetricValueGetScript(metric.ServerConfig.SqlConnection, string(sqlScript), metric.QueryTimeout.Duration)
	if err != nil {
		log.Error(err, fmt.Sprintf("Error while query metric '%s' from server '%s'", metric.Name, metric.ServerConfig.Name))
		return err
	}

	// omit null metrics values
	if value != nil {
		err = sql.InsertMetricValue(log, metric.MetricDb, metric.MetricConfig.DbMetricId, *metric.ServerConfig.SqlServerId, value);
		if err != nil {
			log.Error(err, fmt.Sprintf("Error while insert metric '%s' from server '%s' into metrics database", metric.Name, metric.ServerConfig.Name))
			return err
		} 
	}

	return nil
}

