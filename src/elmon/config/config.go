package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type DbConnectionConfig struct {
	Host                  string `mapstructure:"host"`
	Port                  int    `mapstructure:"port"`
	User                  string `mapstructure:"user"`
	Password              string `mapstructure:"password"`
	DbName                string `mapstructure:"dbname"`
	HostAuthMethod        string `mapstructure:"host-auth-method"` //default md5
	SslMode               string `mapstructure:"ssl-mode"` // default disable
	MaxOpenConnections    int    `mapstructure:"max-open-connections"` // default 100
	MaxIdleConnections    int    `mapstructure:"max-idle-connections"` // default 50
	ConnectionMaxLifetime int    `mapstructure:"connection-max-lifetime"` // default 3600
	ConnectionMaxIdleTime int    `mapstructure:"connection-max-idle-time"` //default 1800
}

type GrafanaConfig struct {
	Url     string `mapstructure:"url"`
	Token   string `mapstructure:"token"`
	Timeout int    `mapstructure:"timeout"` // default 30
}

type LogConfig struct {
	Level    string `mapstructure:"level"`  // default error
	Format   string `mapstructure:"format"` // default json
	FileName string `mapstructure:"file"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"` //default 8080
}

type Config struct {
	MetricsDb DbConnectionConfig `mapstructure:"metrics-db"`
	Server    ServerConfig       `mapstructure:"server"`
	Grafana   GrafanaConfig      `mapstructure:"grafana"`
	Log       LogConfig          `mapstructure:"log"`
}

type configInstance struct {
	cfg  *Config
	once sync.Once
	err  error
}

var globalConfig configInstance

// Load initializes the configuration using Viper with support for YAML and environment variables
func Load( configFilePath string) (*Config, error) {
	globalConfig.once.Do(func() {
		// Load .env file if it exists (for secrets only)
		if err := godotenv.Load(); err != nil {
			fmt.Println(".env file not found, using system environment variables for secrets")
		}

		// Configure Viper
		viper.SetConfigFile(configFilePath)
		viper.SetConfigType("yaml")
		
		// Add paths to search for config files
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		
		// Enable environment variables support only for sensitive data
		viper.AutomaticEnv()
		
		// Configure environment variables prefix
		viper.SetEnvPrefix("METRICS")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		
		// Read config file first (non-sensitive data remains in YAML)
		if err := viper.ReadInConfig(); err != nil {
			globalConfig.err = fmt.Errorf("failed to read config file '%s': %w", configFilePath, err)
			fmt.Printf("failed to read config file '%s': %s/n", configFilePath, err.Error())
			return
		}

		substituteEnvVars()

		globalConfig.cfg = &Config{}
		if err := viper.Unmarshal(globalConfig.cfg); err != nil {
			globalConfig.err = fmt.Errorf("failed to unmarshal config: %w", err)
			fmt.Printf("failed to unmarshal config: %s/n", err.Error())
			globalConfig.cfg = nil
			return
		}
		
		if err := globalConfig.cfg.Validate(); err!=nil {
			globalConfig.err = fmt.Errorf("failed to validate config file: %w", err)
			return
		}

		fmt.Printf("Config loaded from %s/n", configFilePath)
		fmt.Println("Secrets loaded from environment variables with prefix: METRICS_")
	})

	return globalConfig.cfg, globalConfig.err
}

// substituteEnvVars manually substitutes environment variables for sensitive fields
func substituteEnvVars() {
	// Database credentials
	if dbUser := os.Getenv("METRICS_DB_USER"); dbUser != "" {
		viper.Set("metrics-db.user", dbUser)
	}
	if dbPassword := os.Getenv("METRICS_DB_PASSWORD"); dbPassword != "" {
		viper.Set("metrics-db.password", dbPassword)
	}
	if dbHost := os.Getenv("METRICS_DB_HOST"); dbHost != "" {
		viper.Set("metrics-db.host", dbHost)
	}
	if dbName := os.Getenv("METRICS_DB_NAME"); dbName != "" {
		viper.Set("metrics-db.dbname", dbName)
	}

	// Grafana token
	if grafanaToken := os.Getenv("METRICS_GRAFANA_TOKEN"); grafanaToken != "" {
		viper.Set("grafana.token", grafanaToken)
	}
	if grafanaUrl := os.Getenv("METRICS_GRAFANA_URL"); grafanaUrl != "" {
		viper.Set("grafana.url", grafanaUrl)
	}

	// Log configuration
	if logLevel := os.Getenv("METRICS_LOG_LEVEL"); logLevel != "" {
		viper.Set("log.level", logLevel)
	}
}

// GetConfig returns the loaded configuration
func GetConfig() *Config {
	if globalConfig.err != nil {
		fmt.Printf("GetConfig called but config loading previously failed: %s/n", globalConfig.err.Error())
		return nil
	}
	return globalConfig.cfg
}

// ClearCache resets the singleton for testing purposes
func ClearCache() {
	globalConfig = configInstance{}
}

//Validate DatabaseConfig
func (dbConnectionConfig *DbConnectionConfig) Validate() error {
	// Validate database configuration
	if dbConnectionConfig.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if dbConnectionConfig.Port <= 0 || dbConnectionConfig.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", dbConnectionConfig.Port)
	}
	if dbConnectionConfig.User == "" {
		return fmt.Errorf("database user is required")
	}
	if dbConnectionConfig.DbName == "" {
		return fmt.Errorf("database name is required")
	}
	// List of valid authentication methods
	validAuthMethods := []string{
		"md5",
		"scram-sha-256",
		"password",
	}

	// Checking authentication method
	if dbConnectionConfig.HostAuthMethod == "" {
		//default 
		dbConnectionConfig.HostAuthMethod = "md5"
	} else if !slices.Contains(validAuthMethods, dbConnectionConfig.HostAuthMethod) {
		return fmt.Errorf("invalid auth method: %s", dbConnectionConfig.HostAuthMethod)
	}

	// List of valid SSL modes
	validSslModes := []string{
		"disable",
		"allow",
		"prefer",
		"require",
	}

	// Checking SSL mode
	if dbConnectionConfig.SslMode == "" {
		//default
		dbConnectionConfig.SslMode = "disable"
	} else if !slices.Contains(validSslModes, dbConnectionConfig.SslMode) {
		return fmt.Errorf("invalid SSL mode: %s", dbConnectionConfig.SslMode)
	}

	// MetricsDb.MaxOpenConnections
	if dbConnectionConfig.MaxOpenConnections < 0 {
		return fmt.Errorf("invalid max open connections: %d", dbConnectionConfig.MaxOpenConnections)
	} else if dbConnectionConfig.MaxOpenConnections == 0 {
		//default
		dbConnectionConfig.MaxOpenConnections = 100
	}

	// MetricsDb.MaxIdleConnections
	if dbConnectionConfig.MaxIdleConnections < 0 {
		return fmt.Errorf("invalid max idle connections: %d", dbConnectionConfig.MaxIdleConnections)
	} else if dbConnectionConfig.MaxIdleConnections == 0 {
		//default
		dbConnectionConfig.MaxIdleConnections = 50
	}	

	// MetricsDb.ConnectionMaxLifetime
	if dbConnectionConfig.ConnectionMaxLifetime < 0 {
		return fmt.Errorf("invalid connection max lifetime: %d", dbConnectionConfig.ConnectionMaxLifetime)
	} else if dbConnectionConfig.ConnectionMaxLifetime == 0 {
		//default
		dbConnectionConfig.ConnectionMaxLifetime = 3600
	}	

	return nil
}

//Validate Server config
func (serverConfig *ServerConfig) Validate() error {
	// Validate server configuration
	if serverConfig.Port < 0 || serverConfig.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", serverConfig.Port)
	} else if serverConfig.Port == 0 {
		// default
		serverConfig.Port = 8080;
	}
	return nil
}

//Validate Grafana config
func (grafanaConfig *GrafanaConfig) Validate() error {
	if grafanaConfig.Url == "" {
		return fmt.Errorf("grafana URL is required")
	}

	if grafanaConfig.Token == "" {
		return fmt.Errorf("grafana Token is required")
	}

	if grafanaConfig.Timeout == 0 {
		//default
		grafanaConfig.Timeout = 30
	} else if grafanaConfig.Timeout < 0 {
		return fmt.Errorf("invalid grafana timeout: %d", grafanaConfig.Timeout)
	}

	return nil
}

// Validate LogConfig
func (logConfig *LogConfig) Validate() error {
	validLogLevels := []string{
		"debug",
		"info",
		"warn",
		"error",
	}

	if logConfig.Level == "" {
		//default
		logConfig.Level = "warn"
	} else if !slices.Contains(validLogLevels, logConfig.Level) {
		return fmt.Errorf("invalid log level: %s", logConfig.Level)
	}

	validFormats := []string{
		"json",
		"text",
	}

	if logConfig.Format == "" {
		//default 
		logConfig.Format = "json"
	} else if !slices.Contains(validFormats, logConfig.Format) {
		return fmt.Errorf("invalid log format: %s", logConfig.Format)
	}

	return  nil
}


// Validate performs basic configuration validation
func (config *Config) Validate() error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate MetricsDb config
	if err := config.MetricsDb.Validate(); err!=nil {
		return err
	}
	
	// Validate Server config
	if err := config.Server.Validate(); err!=nil {
		return err
	}

	// Validate grafana config
	if err := config.Grafana.Validate(); err!=nil {
		return err
	}
	
	// Validate log config
	if err:= config.Log.Validate(); err!=nil {
		return err
	}

	return nil
}