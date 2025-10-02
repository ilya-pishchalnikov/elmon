package collector

import (
	"database/sql"
	"elmon/config"
	"elmon/logger"
	"elmon/scheduler"
	"fmt"
)

type ServerMetricScheduler struct {
	ServerName string
	MetricName string
	Scheduler  *scheduler.TaskScheduler
}

// Type for collect metrics from servers and store them into metrics DB
type Collector struct {
	ServersMetrics config.ServerMetricMap
	Logger         *logger.Logger
	MetricsDb      *sql.DB
	Schedulers     []ServerMetricScheduler
}

func NewServerMetricsScheduler (serverName string, metricName string, scheduler *scheduler.TaskScheduler) ServerMetricScheduler{
	return ServerMetricScheduler{
		ServerName: serverName,
		MetricName: metricName,
		Scheduler: scheduler,
	}
}

//Collector struct constructor
func NewCollector(serversMetrics config.ServerMetricMap, logger *logger.Logger, metricsDb *sql.DB) *Collector{
	var collector = &Collector{
		ServersMetrics: serversMetrics,
		Logger: logger,
		MetricsDb: metricsDb,
	}

	var schedulers []ServerMetricScheduler

	for _, server := range serversMetrics.Servers {
		for _, metric := range server.Metrics {
			scheduler := scheduler.NewTaskScheduler(metric.Interval.Duration, metric.MaxRetries, metric.RetryDelay.Duration,ProcessMetric, &metric, metric.Logger)
			schedulers = append(schedulers, NewServerMetricsScheduler(server.Name, metric.Name, scheduler))
		}
	}

	collector.Schedulers = schedulers;

	return collector;
}

// Strart all schedulers
func (collector *Collector) Start() error{
	for i := range collector.Schedulers {
		scheduler := collector.Schedulers[i]
		if err:=scheduler.Scheduler.Start(); err!=nil {
			scheduler.Scheduler.Logger.Error(err, fmt.Sprintf("Error starting scheduler for server '%s' metric '%s'", scheduler.ServerName, scheduler.MetricName))
			return err
		}
	}

	collector.Logger.Info("All schedulers started")

	return nil
}

// Stop all schedulers
func (collector *Collector) Stop() {
	for i := range collector.Schedulers {
		scheduler := collector.Schedulers[i]
		scheduler.Scheduler.Stop();
	}
	collector.Logger.Info("All schedulers stopped")
}