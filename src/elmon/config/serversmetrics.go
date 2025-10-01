package config

import (
	"elmon/logger"
	"fmt"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// MetricForMapping holds overridden collection parameters for a specific metric on a server.
type MetricForMapping struct {
	Name         string   `mapstructure:"name"`
	Interval     Duration `mapstructure:"interval"`
	MaxRetries   int      `mapstructure:"max-retries"`
	RetryDelay   Duration `mapstructure:"retry-delay"`
	QueryTimeout Duration `mapstructure:"query-timeout"`
	MetricConfig *Metric // Pointer to the metric's base configuration
}

// ServerMetricMapItem represents a single server and the list of metrics assigned to it.
type ServerMetricMapItem struct {
	Name    string             `mapstructure:"name"`
	Metrics []MetricForMapping `mapstructure:"metrics"`
	Config  *DbConnectionConfig // Pointer to the server's connection configuration
}

// ServerMetricMap holds the overall configuration of which metrics to collect from which servers.
type ServerMetricMap struct {
	Servers []ServerMetricMapItem `mapstructure:"servers"`
	viper   *viper.Viper
}

// Load loads the server-to-metric mapping configuration from a YAML file.
func (l *ServerMetricMap) Load(log *logger.Logger, configFile string, servers DbServers, metrics MetricsConfig) (*ServerMetricMap, error) {
	// Load .env file if exists (for future environment variable support)
	envFile := ".env"
	if err := godotenv.Load(envFile); err == nil {
		log.Info(fmt.Sprintf("Loaded environment variables from: %s", envFile))
	}

	// Configure Viper
	viper := viper.New()
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	// Enable environment variable support for future use
	viper.AutomaticEnv()
	viper.SetEnvPrefix("METRICS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		err = fmt.Errorf("failed to read config file '%s': %w", configFile, err)
		log.Error(err, "failed to read config file")
		return nil, err
	}

	// Create custom decoder with hook for Duration type
	viper.Set("verbatim", true) // Prevent environment variable substitution for now

	var config ServerMetricMap

	config.viper = viper;

	// Get the raw data map from Viper
	var raw map[string]interface{}
	// We use AllSettings to get all configuration keys, including defaults
	raw = viper.AllSettings()

	// Define the configuration for the mapstructure decoder
	decoderConfig := mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &config,
		// The core fix: compose decode hooks
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			// Hook 1: Standard hook for time.Duration (safety/completeness)
			mapstructure.StringToTimeDurationHookFunc(),
			// Hook 2: Custom hook to convert strings/numbers to our Duration struct
			customDurationHook(),
		),
		// Use mapstructure tags
		TagName: "mapstructure",
		// Allow automatic type conversion for basic types
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(&decoderConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create mapstructure decoder: %w", err)
	}

	if err := decoder.Decode(raw); err != nil {
		err = fmt.Errorf("failed to unmarshal config file '%s': %w", configFile, err)
		log.Error(err, "failed to unmarshal config")
		return nil, err
	}

	// Validate configuration
	if err := config.Validate(log, &config, servers, metrics); err != nil {
		err = fmt.Errorf("config file '%s' validation failed: %w", configFile, err)
		log.Error(err, "config file validation failed")
		return nil, err
	}

	log.Info(fmt.Sprintf("Metric mapping configuration loaded successfully from: '%s'", configFile))
	return &config, nil
}

// Validate validates the server-metric mapping configuration.
func (l *ServerMetricMap) Validate(log *logger.Logger, config *ServerMetricMap, servers DbServers, metrics MetricsConfig) error {

	serverNames := make(map[string]bool)
	for serverIndex := range l.Servers {
		server := &l.Servers[serverIndex]
		// Check if the server exists in the servers config
		server.Config = servers.GetByName(server.Name)
		if server.Config == nil {
			err := fmt.Errorf("DB server with name '%s' not found in server configurations", server.Name)
			log.Error(err, fmt.Sprintf("Error while validating server-metric mapping config at server index = %d", serverIndex))
			return err
		}
		// Validate unique server name in the mapping file
		if serverNames[server.Name] {
			err := fmt.Errorf("DB server with name '%s' at index %d is duplicate in the mapping file", server.Name, serverIndex)
			log.Error(err, "Error while parsing server-metric mapping config")
			return err
		}

		serverNames[server.Name] = true

		metricNames := make(map[string]bool)
		// Validate metrics assigned to this server
		for metricIndex := range server.Metrics {
			metric := &server.Metrics[metricIndex]
			if err := metric.Validate(log, config, metrics, l.viper, serverIndex, metricIndex); err != nil {
				err := fmt.Errorf("invalid metric '%s' (index %d) for server '%s' (index %d): %w", metric.Name, metricIndex, server.Name, serverIndex, err)
				log.Error(err, "Error while parsing server-metric mapping config")
				return err
			}

			// Validate unique metric name within this server's assignment
			if metricNames[metric.Name] {
				err := fmt.Errorf("Metric with name '%s' (index %d) is duplicate for DB server '%s' (index %d)", metric.Name, metricIndex, server.Name, serverIndex)
				log.Error(err, "Error while parsing server-metric mapping config")
				return err
			}

			metricNames[metric.Name] = true
		}

	}

	return nil
}

// Validate ensures the metric entry is valid and links it to the base metric config.
func (l *MetricForMapping) Validate(log *logger.Logger, config *ServerMetricMap, metrics MetricsConfig, viper *viper.Viper, serverIndex int, metricIndex int) error {
	if l.Name == "" {
		err := fmt.Errorf("metric entry must have a name")
		log.Error(err, "Error while parsing server-metric mapping config")
		return err
	}

	l.MetricConfig = metrics.GetMetricByName(l.Name)

	if l.MetricConfig == nil {
		err := fmt.Errorf("metric '%s' is not defined in the main metrics configuration", l.Name)
		log.Error(err, "Error while parsing server-metric mapping config")
		return err
	}

	// Override interval if explicitly set in the server-metric mapping
	if !viper.IsSet(fmt.Sprintf("servers.%d.metrics.%d.interval", serverIndex, metricIndex)) {
		l.Interval = l.MetricConfig.Interval
	}

	// Override max-retries if explicitly set in the server-metric mapping
	if !viper.IsSet(fmt.Sprintf("servers.%d.metrics.%d.max-retries", serverIndex, metricIndex)) {
		l.MaxRetries = l.MetricConfig.MaxRetries
	}

	// Override retry-delay if explicitly set in the server-metric mapping
	if !viper.IsSet(fmt.Sprintf("servers.%d.metrics.%d.retry-delay", serverIndex, metricIndex)) {
		l.RetryDelay = l.MetricConfig.RetryDelay
	}

	// Override query-timeout if explicitly set in the server-metric mapping
	if !viper.IsSet(fmt.Sprintf("servers.%d.metrics.%d.query-timeout", serverIndex, metricIndex)) {
		l.QueryTimeout = l.MetricConfig.QueryTimeout
	}

	return nil
}