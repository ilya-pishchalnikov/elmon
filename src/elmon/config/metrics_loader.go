package config

import (
	"elmon/logger"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// MetricsConfigLoader loads and manages metrics configuration
type MetricsConfigLoader struct {
    basePath string
    viper    *viper.Viper
}

// NewMetricsConfigLoader creates a new metrics config loader
func NewMetricsConfigLoader(basePath string) *MetricsConfigLoader {
    return &MetricsConfigLoader{
        basePath: basePath,
        viper:    viper.New(),
    }
}

// Load loads the metrics configuration from YAML file
func (l *MetricsConfigLoader) Load(log *logger.Logger, configFile string) (*MetricsConfig, error) {
    // Load .env file if exists (for future environment variable support)
    envFile := filepath.Join(l.basePath, ".env")
    if err := godotenv.Load(envFile); err == nil {
        log.Info(fmt.Sprintf("Loaded environment variables from: %s", envFile))
    }

    // Configure Viper
    l.viper.SetConfigFile(configFile)
    l.viper.SetConfigType("yaml")
    
    // Enable environment variable support for future use
    l.viper.AutomaticEnv()
    l.viper.SetEnvPrefix("METRICS")
    l.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

    // Set default values
    l.setDefaults()

    // Read config file
    if err := l.viper.ReadInConfig(); err != nil {
        err = fmt.Errorf("failed to read config file '%s': %w", configFile, err)
        log.Error(err, "failed to read config file")
        return nil, err
    }

    // Create custom decoder with hook for Duration type
    l.viper.Set("verbatim", true) // Prevent environment variable substitution for now

    var config MetricsConfig

     // 1. Get the raw data map from Viper
     var raw map[string]interface{}
     // We use AllSettings to get all configuration keys, including defaults
     raw = l.viper.AllSettings()
 
     // 2. Define the configuration for the mapstructure decoder
     decoderConfig := mapstructure.DecoderConfig{
         Metadata:         nil,
         Result:           &config,
         // The core fix: compose decode hooks
         DecodeHook: mapstructure.ComposeDecodeHookFunc(
             // Hook 1: Standard hook for time.Duration (safety/completeness)
             mapstructure.StringToTimeDurationHookFunc(),
             // Hook 2: Custom hook to convert strings/numbers to our Duration struct
             customDurationHook(),
         ),
         // Use mapstructure tags
         TagName:          "mapstructure",
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
    if err := l.Validate(log, &config); err != nil {
        err = fmt.Errorf("config file '%s' validation failed: %w", configFile, err)
        log.Error(err, "config file validation failed")
        return nil, err
    }

    log.Info(fmt.Sprintf("Metrics configuration loaded successfully from: '%s'", configFile))
    return &config, nil
}

// setDefaults sets default values for configuration
func (l *MetricsConfigLoader) setDefaults() {
    l.viper.SetDefault("version", "1.0")
    l.viper.SetDefault("description", "PostgreSQL server metrics collection configuration")
    l.viper.SetDefault("global.default-interval", "30s")
    l.viper.SetDefault("global.default-query-timeout", "10s")
    l.viper.SetDefault("global.default-max-retries", 3)
    l.viper.SetDefault("global.default-retry-delay", "5s")
}

// Validate validates the metrics configuration
func (l *MetricsConfigLoader) Validate(log *logger.Logger, config *MetricsConfig) error {
    if l.viper.IsSet("Version") && config.Version != "1.0" {
        err := fmt.Errorf("version 1.0 expected")
        log.Error(err, "error while metrics config validation");
        return err
    }

    //Validate global config
    if err:=l.validateGlobalConfig(log, &config.Global); err!=nil {
        log.Error(err, "error in global section of config");
        return err
    }

    // Validate metric groups and metrics
    for i, group := range config.MetricGroups {
        if err := l.validateMetricGroup(log, &group, i); err != nil {
            log.Error(err, fmt.Sprintf("error while metrics group[%d] '%s' in metrics config validation", i, group.Name))
            return err
        }
    }

    if err:=l.validateUniqueMetricGroupNames(log, config); err!=nil {
        log.Error(err, "error while metrics config validation");
        return err
    }

    if err:=l.validateUniqueMetricNamesGlobally(log, config); err!=nil {
        log.Error(err, "error while metrics config validation");
        return err
    }


    return nil
}


// validateUniqueMetricGroupNames ensures all metric group names are unique
func (l *MetricsConfigLoader) validateUniqueMetricGroupNames(log *logger.Logger, config *MetricsConfig) error {
    names := make(map[string]bool)
    for _, group := range config.MetricGroups {
        if group.Name == "" {
            // Already handled in validateMetricGroup, but a non-empty name is needed for uniqueness check
            continue
        }
        if names[group.Name] {
            err := fmt.Errorf("duplicate metric group name found: '%s'", group.Name)
            log.Error(err, "config validation error: duplicate metric group name")
            return err
        }
        names[group.Name] = true
    }
    return nil
}

// validateUniqueMetricNamesGlobally ensures all metric names across all groups are unique
func (l *MetricsConfigLoader) validateUniqueMetricNamesGlobally(log *logger.Logger, config *MetricsConfig) error {
    names := make(map[string]bool)
    for _, group := range config.MetricGroups {
        for _, metric := range group.Metrics {
            if metric.Name == "" {
                // Already handled in validateMetric, but a non-empty name is needed for uniqueness check
                continue
            }
            if names[metric.Name] {
                err := fmt.Errorf("duplicate metric name found globally: '%s'", metric.Name)
                log.Error(err, "config validation error: duplicate metric name")
                return err
            }
            names[metric.Name] = true
        }
    }
    return nil
}

func (l *MetricsConfigLoader) validateGlobalConfig(log *logger.Logger, global *GlobalConfig) error {
    // Validate default interval (use default if not set)
    if !l.viper.IsSet("global.default-interval") {
        var err error
        global.DefaultInterval.Duration, err = time.ParseDuration("30s")
        if err!=nil {
            log.Error (err, "error while parse global default interval default")
            return  err
        }
    }

    // Validate default query timeout (use default if not set)
    if !l.viper.IsSet("global.default-query-timeout") {
        var err error
        global.DefaultQueryTimeout.Duration, err = time.ParseDuration("10s")
        if err!=nil {
            log.Error (err, "error while parse global default query timeout default")
            return  err
        }
    }

    // Validate default max retries (use default if not set)
    if !l.viper.IsSet("global.default-max-retries") {
        global.DefaultMaxRetries = 0
    }

    // Validate default query timeout (use default if not set)
    if !l.viper.IsSet("global.default-retry-delay") {
        var err error
        global.DefaultQueryTimeout.Duration, err = time.ParseDuration("5s")
        if err!=nil {
            log.Error (err, "error while parse global default retry delay default")
            return  err
        }
    }

    return nil
}

// validateMetricGroup validates a metric group
func (l *MetricsConfigLoader) validateMetricGroup(log *logger.Logger, group *MetricGroup, index int) error {
    if group.Name == "" {
        err := fmt.Errorf("metric group [%d]: name is required", index)
        log.Error (err, "config validation error")
        return  err
    }

    if !l.viper.IsSet(fmt.Sprintf("metric-groups.%d.enabled", index)) {
        group.Enabled = true
    }

    if !group.Enabled {
        return nil // Skip validation for disabled groups
    }

    if !l.viper.IsSet(fmt.Sprintf("metric-groups.%d.description", index)) {
        group.Description = ""
    }

    for j, metric := range group.Metrics {
        if err := l.validateMetric(log, &metric, index, j); err != nil {
            err = fmt.Errorf("metric group '%s' metric [%d] '%s': %w", group.Name, j, metric.Name, err)
            log.Error (err, "config validation error")
            return err
        }
    }

    return nil
}

// validateMetric validates a single metric configuration
func (l *MetricsConfigLoader) validateMetric(log *logger.Logger, metric *Metric, groupIndex int, metricIndex int) error {
    if metric.Name == "" {
        err := fmt.Errorf("name is required")
        log.Error (err, "config validation error")
        return err
    }

    // Validate ValueType (use default if not set)
    validValueTypes := []ValueType{
		ValueTypeInt,
        ValueTypeFloat,
        ValueTypeString,
        ValueTypeBool,
        ValueTypeTable,
        ValueTypeInt64,
	}

    if metric.ValueType == "" {
        //Set default
        metric.ValueType = ValueTypeFloat
    } else if !slices.Contains(validValueTypes, metric.ValueType) {
        err := fmt.Errorf("invalid metric type '%s'", metric.ValueType)
        log.Error(err, "config validation error")
        return  err;
    }

    // Validate interval (use global default if not set)
    if metric.Interval.Duration == 0 {
        metric.Interval.Duration = l.viper.GetDuration("global.default-interval")
    } 

    // Validate collection type specific requirements
    switch metric.CollectionType {
    case CollectionTypeSQL:
        if metric.SQLFile == "" {
            return fmt.Errorf("sql_file is required for sql collection type")
        }
        
        // Validate SQL file path and existence
        sqlPath := filepath.Join(l.basePath, metric.SQLFile)
        if _, err := os.Stat(sqlPath); err != nil {
            return fmt.Errorf("sql file not found: %s", sqlPath)
        }

    case CollectionTypeGoFunc:
        if metric.GoFunction == "" {
            return fmt.Errorf("go_function is required for go_func collection type")
        }

    default:
        err := fmt.Errorf("unknown collection type: %s", metric.CollectionType)
        log.Error(err, "config validation error")
        return  err;
    }

    // Validate query timeout (use global default if not set)
    if !viper.IsSet(fmt.Sprintf("metric-groups.%d.metrics.%d.query_timeout", groupIndex, metricIndex)) {
        metric.QueryTimeout = Duration{l.viper.GetDuration("global.default-query-timeout")}
    }

    // Validate max-retries (use global default if not set)
    if !viper.IsSet(fmt.Sprintf("metric-groups.%d.metrics.%d.max-retries", groupIndex, metricIndex)) {
        metric.MaxRetries = l.viper.GetInt("global.default-max-retries")
    } else if metric.MaxRetries < 0 {
        err := fmt.Errorf("invalid max retries value: %d", metric.MaxRetries)
        log.Error(err, "config validation error")
        return  err;
    }

    // Validate retry delay (use global default if not set)
    if !viper.IsSet(fmt.Sprintf("metric-groups.%d.metrics.%d.retry-delay", groupIndex, metricIndex)) {
        metric.RetryDelay.Duration = l.viper.GetDuration("global.default-retry-delay")
    }

    return nil
}

// GetSQLQuery loads SQL query from file
func (l *MetricsConfigLoader) GetSQLQuery(metric *Metric) (string, error) {
    if metric.CollectionType != CollectionTypeSQL {
        return "", fmt.Errorf("metric '%s' is not an SQL metric", metric.Name)
    }

    sqlPath := filepath.Join(l.basePath, metric.SQLFile)
    data, err := os.ReadFile(sqlPath)
    if err != nil {
        return "", fmt.Errorf("failed to read SQL file '%s': %w", sqlPath, err)
    }

    return string(data), nil
}

// GetEnabledMetrics returns all enabled metrics
func (l *MetricsConfigLoader) GetEnabledMetrics(config *MetricsConfig) []*Metric {
    var enabledMetrics []*Metric

    for i := range config.MetricGroups {
        group := &config.MetricGroups[i]
        if !group.Enabled {
            continue
        }

        for j := range group.Metrics {
            metric := &group.Metrics[j]
            enabledMetrics = append(enabledMetrics, metric)
        }
    }

    return enabledMetrics
}