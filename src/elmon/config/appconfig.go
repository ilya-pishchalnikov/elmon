// File: appconfig.go
package config

import (
	"bytes"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// AppConfig is the root structure containing all application configuration
type AppConfig struct {
	Log              LogConfig              `mapstructure:"log"`
	MetricsDB        DbConnectionConfig     `mapstructure:"metrics-db"`
	Grafana          GrafanaConfig          `mapstructure:"grafana"`
	DBServers        []DbConnectionConfig   `mapstructure:"db-servers"`
	Metrics          MetricsConfig          `mapstructure:"metrics"`
	ServerMetricsMap []ServerMetricsMapping `mapstructure:"servers-metrics-map"`
}

// LogConfig defines logging parameters
type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // json, text
	File   string `mapstructure:"file"`
}

// DbConnectionConfig defines database connection parameters
type DbConnectionConfig struct {
	Name                  string `mapstructure:"name"`
	Environment           string `mapstructure:"environment"`
	Host                  string `mapstructure:"host"`
	Port                  int    `mapstructure:"port"`
	User                  string `mapstructure:"user"`
	Password              string `mapstructure:"password"`
	DbName                string `mapstructure:"dbname"`
	SslMode               string `mapstructure:"ssl-mode"`                 // default: disable
	MaxOpenConnections    int    `mapstructure:"max-open-connections"`     // default: 100
	MaxIdleConnections    int    `mapstructure:"max-idle-connections"`     // default: 50
	ConnectionMaxLifetime int    `mapstructure:"connection-max-lifetime"`  // default: 3600s
	ConnectionMaxIdleTime int    `mapstructure:"connection-max-idle-time"` // default: 1800s

	// These fields are not populated from config but used at runtime
	SqlServerId   *int
	SqlConnection *sql.DB
}

// GrafanaConfig defines Grafana connection parameters
type GrafanaConfig struct {
	Url     string `mapstructure:"url"`
	Token   string `mapstructure:"token"`
	Timeout int    `mapstructure:"timeout"` // in seconds, default: 30
}

// MetricsConfig represents configuration for metrics collection
type MetricsConfig struct {
	Version      string        `mapstructure:"version"`
	Description  string        `mapstructure:"description"`
	Global       GlobalConfig  `mapstructure:"global"`
	MetricGroups []MetricGroup `mapstructure:"metric-groups"`
}

// GlobalConfig contains global settings for metrics
type GlobalConfig struct {
	DefaultInterval     Duration `mapstructure:"default-interval"`
	DefaultQueryTimeout Duration `mapstructure:"default-query-timeout"`
	DefaultMaxRetries   int      `mapstructure:"default-max-retries"`
	DefaultRetryDelay   Duration `mapstructure:"default-retry-delay"`
}

// MetricGroup represents a group of related metrics
type MetricGroup struct {
	Name        string   `mapstructure:"name"`
	Description string   `mapstructure:"description"`
	Enabled     bool     `mapstructure:"enabled"`
	Metrics     []Metric `mapstructure:"metrics"`
}

// Metric defines a single metric to collect
type Metric struct {
	Name           string   `mapstructure:"name"`
	Description    string   `mapstructure:"description"`
	ValueType      string   `mapstructure:"value-type"`      // int, float, string, bool, table
	Interval       Duration `mapstructure:"interval"`
	CollectionType string   `mapstructure:"collection-type"` // sql, go_func
	SQLFile        string   `mapstructure:"sql-file"`
	GoFunction     string   `mapstructure:"go-function"`
	QueryTimeout   Duration `mapstructure:"query-timeout"`
	MaxRetries     int      `mapstructure:"max-retries"`
	RetryDelay     Duration `mapstructure:"retry-delay"`
	Unit           string   `mapstructure:"unit"`
	DbMetricId     int      // Populated at runtime
}

// ServerMetricsMapping links a server with a set of metrics to collect
type ServerMetricsMapping struct {
	Name    string                   `mapstructure:"name"`
	Metrics []ServerMetricOverride `mapstructure:"metrics"`
}

// ServerMetricOverride allows overriding metric parameters for a specific server
type ServerMetricOverride struct {
	Name         string   `mapstructure:"name"`
	Interval     Duration `mapstructure:"interval"`
	MaxRetries   int      `mapstructure:"max-retries"`
	RetryDelay   Duration `mapstructure:"retry-delay"`
	QueryTimeout Duration `mapstructure:"query-timeout"`
}

// Duration wrapper around time.Duration for proper YAML unmarshaling
type Duration struct {
	time.Duration
}

// UnmarshalText implements interface for parsing Duration
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// customDurationHook is a mapstructure hook for parsing time strings
func customDurationHook() mapstructure.DecodeHookFunc {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if t != reflect.TypeOf(Duration{}) {
			return data, nil
		}
		if f.Kind() != reflect.String {
			return data, nil
		}
		d, err := time.ParseDuration(data.(string))
		if err != nil {
			return nil, err
		}
		return Duration{Duration: d}, nil
	}
}

// Load reads, deserializes and validates configuration file
func Load(configPath string) (*AppConfig, error) {
	// Load .env file for secrets
	if err := godotenv.Load(); err != nil {
		fmt.Println("INFO: .env file not found, using system environment variables for secrets")
	}

	// Read raw file
	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	// Expand environment variables of format ${VAR}
	expandedContent := os.ExpandEnv(string(rawContent))

	// Initialize Viper
	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewBufferString(expandedContent)); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	// Set default values
	setDefaults(v)

	var config AppConfig

	// Decode with custom hook for Duration
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:     &config,
		TagName:    "mapstructure",
		DecodeHook: mapstructure.ComposeDecodeHookFunc(customDurationHook()),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}
	if err := decoder.Decode(v.AllSettings()); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate entire configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	fmt.Printf("Configuration loaded successfully from %s\n", configPath)
	return &config, nil
}

// setDefaults sets default values for Viper
func setDefaults(v *viper.Viper) {
	// Log
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	// Grafana
	v.SetDefault("grafana.timeout", 30)
	// Metrics
	v.SetDefault("metrics.version", "1.0")
	v.SetDefault("metrics.global.default-interval", "30s")
	v.SetDefault("metrics.global.default-query-timeout", "10s")
	v.SetDefault("metrics.global.default-max-retries", 0)
	v.SetDefault("metrics.global.default-retry-delay", "5s")
}

// Validate runs all validation checks for loaded configuration
func (cfg *AppConfig) Validate() error {
	if err := cfg.Log.Validate(); err != nil {
		return fmt.Errorf("log config validation failed: %w", err)
	}
	if err := cfg.MetricsDB.Validate(); err != nil {
		return fmt.Errorf("metrics-db config validation failed: %w", err)
	}
	if err := cfg.Grafana.Validate(); err != nil {
		return fmt.Errorf("grafana config validation failed: %w", err)
	}

	// Validate server list
	serverNames := make(map[string]bool)
	for i := range cfg.DBServers {
		srv := &cfg.DBServers[i]
		if err := srv.Validate(); err != nil {
			return fmt.Errorf("db-server at index %d ('%s') validation failed: %w", i, srv.Name, err)
		}
		if serverNames[srv.Name] {
			return fmt.Errorf("duplicate db server name found: '%s'", srv.Name)
		}
		serverNames[srv.Name] = true
	}

	// Validate metrics
	if err := cfg.Metrics.Validate(); err != nil {
		return fmt.Errorf("metrics config validation failed: %w", err)
	}

	// Validate server-metrics mapping
	metricNames := cfg.Metrics.GetAllMetricNames()
	if err := validateServerMetricsMap(cfg.ServerMetricsMap, serverNames, metricNames); err != nil {
		return fmt.Errorf("servers-metrics-map validation failed: %w", err)
	}

	return nil
}

// --- Individual validation functions ---

func (c *LogConfig) Validate() error {
	validLevels := []string{"debug", "info", "warn", "error"}
	if !slices.Contains(validLevels, strings.ToLower(c.Level)) {
		return fmt.Errorf("invalid log level: '%s'", c.Level)
	}
	validFormats := []string{"json", "text"}
	if !slices.Contains(validFormats, strings.ToLower(c.Format)) {
		return fmt.Errorf("invalid log format: '%s'", c.Format)
	}
	return nil
}

func (c *DbConnectionConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("invalid port: %d", c.Port)
	}
	if c.User == "" {
		return fmt.Errorf("user is required")
	}
	if c.DbName == "" {
		return fmt.Errorf("dbname is required")
	}
	if c.Name == "" {
		c.Name = fmt.Sprintf("%s:%d_%s", c.Host, c.Port, c.DbName)
	}
	if c.SslMode == "" {
		c.SslMode = "disable"
	}

	return nil
}

func (c *GrafanaConfig) Validate() error {
	if c.Url == "" {
		return fmt.Errorf("url is required")
	}
	if c.Token == "" {
		return fmt.Errorf("token is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive: %d", c.Timeout)
	}
	return nil
}

func (c *MetricsConfig) Validate() error {
	if c.Version != "1.0" {
		return fmt.Errorf("unsupported metrics config version: '%s', expected '1.0'", c.Version)
	}

	groupNames := make(map[string]bool)
	metricNames := make(map[string]bool)

	for _, group := range c.MetricGroups {
		if group.Name == "" {
			return fmt.Errorf("metric group name is required")
		}
		if groupNames[group.Name] {
			return fmt.Errorf("duplicate metric group name: '%s'", group.Name)
		}
		groupNames[group.Name] = true

		for _, metric := range group.Metrics {
			if metric.Name == "" {
				return fmt.Errorf("metric name is required in group '%s'", group.Name)
			}
			if metricNames[metric.Name] {
				return fmt.Errorf("duplicate metric name found globally: '%s'", metric.Name)
			}
			// Validate specific metric
			if err := metric.Validate(); err != nil {
				return fmt.Errorf("metric '%s' validation failed: %w", metric.Name, err)
			}
			metricNames[metric.Name] = true
		}
	}
	return nil
}

func (m *Metric) Validate() error {
	// Validate ValueType
	validValueTypes := []string{"int", "float", "string", "bool", "table", "int64"}
	if !slices.Contains(validValueTypes, m.ValueType) {
		return fmt.Errorf("invalid value-type: '%s'", m.ValueType)
	}

	// Validate CollectionType
	switch m.CollectionType {
	case "sql":
		if m.SQLFile == "" {
			return fmt.Errorf("sql-file is required for collection-type 'sql'")
		}
		// File existence check - optional, better to do when collector starts
	case "go_func":
		if m.GoFunction == "" {
			return fmt.Errorf("go-function is required for collection-type 'go_func'")
		}
	default:
		return fmt.Errorf("unknown collection-type: '%s'", m.CollectionType)
	}
	return nil
}

func validateServerMetricsMap(mappings []ServerMetricsMapping, serverNames map[string]bool, metricNames map[string]bool) error {
	mapServerNames := make(map[string]bool)
	for _, mapping := range mappings {
		if mapping.Name == "" {
			return fmt.Errorf("server name is required in servers-metrics-map")
		}
		if !serverNames[mapping.Name] {
			return fmt.Errorf("server '%s' from servers-metrics-map is not defined in db-servers", mapping.Name)
		}
		if mapServerNames[mapping.Name] {
			return fmt.Errorf("duplicate server name '%s' in servers-metrics-map", mapping.Name)
		}
		mapServerNames[mapping.Name] = true

		mapMetricNames := make(map[string]bool)
		for _, metric := range mapping.Metrics {
			if metric.Name == "" {
				return fmt.Errorf("metric name is required for server '%s' in mapping", mapping.Name)
			}
			if !metricNames[metric.Name] {
				return fmt.Errorf("metric '%s' for server '%s' is not defined in metrics configuration", metric.Name, mapping.Name)
			}
			if mapMetricNames[metric.Name] {
				return fmt.Errorf("duplicate metric '%s' for server '%s' in mapping", metric.Name, mapping.Name)
			}
			mapMetricNames[metric.Name] = true
		}
	}
	return nil
}

// --- Helper functions ---

// GetAllMetricNames returns a slice of all defined metric names
func (c *MetricsConfig) GetAllMetricNames() map[string]bool {
	names := make(map[string]bool)
	for _, group := range c.MetricGroups {
		for _, metric := range group.Metrics {
			names[metric.Name] = true
		}
	}
	return names
}