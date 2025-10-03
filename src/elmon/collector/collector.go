// File: collector.go
package collector

import (
	"elmon/logger"
	"elmon/scheduler"
	"fmt"
)

type ServerMetricScheduler struct {
	ServerName string
	MetricName string
	Scheduler  *scheduler.TaskScheduler
}

// Collector handles metric collection from servers and storage into metrics database
type Collector struct {
	Logger     *logger.Logger
	Schedulers []ServerMetricScheduler
}

// Collector constructor
func NewCollector(
	tasks []*MetricTask,
	log *logger.Logger,
) *Collector {

	var schedulers []ServerMetricScheduler
	for _, task := range tasks {
		// Create scheduler with universal task
		sch := scheduler.NewTaskScheduler(
			task.Interval,
			task.MaxRetries,
			task.RetryDelay,
			ProcessMetric, // Our executor function
			task,          // Task payload
			task.Logger,
		)
		schedulers = append(schedulers, ServerMetricScheduler{
			ServerName: task.ServerName,
			MetricName: task.MetricName,
			Scheduler:  sch,
		})
	}

	return &Collector{
		Logger:     log,
		Schedulers: schedulers,
	}
}

// Start all schedulers
func (collector *Collector) Start() error {
	for i := range collector.Schedulers {
		scheduler := collector.Schedulers[i]
		if err := scheduler.Scheduler.Start(); err != nil {
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
		scheduler.Scheduler.Stop()
	}
	collector.Logger.Info("All schedulers stopped")
}