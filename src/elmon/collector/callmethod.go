package collector

import (
	"context"
	"elmon/config"
	"fmt"
)

func ProcessMetric (ctx context.Context, metric *config.ServerMetric) error {
	if metric.MetricConfig.CollectionType == "sql" {
		return ExecuteSql(metric)
	}else {
		err:= fmt.Errorf("collection type '%s' not implemented yet. Metric '%s' of server '%s' not collected",
						metric.MetricConfig.CollectionType, metric.Name, metric.ServerConfig.Name);
		metric.Logger.Error(err, "Metric collection error")
		return err
	}
}