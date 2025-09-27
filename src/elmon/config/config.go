package config

import (
	"fmt"
	"os"
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
	HostAuthMethod        string `mapstructure:"host-auth-method"`
	SslMode               string `mapstructure:"ssl-mode"`
	MaxOpenConnections    int    `mapstructure:"max-open-connections"`
	MaxIdleConnections    int    `mapstructure:"max-idle-connections"`
	ConnectionMaxLifetime int    `mapstructure:"connection-max-lifetime"`
	ConnectionMaxIdleTime int    `mapstructure:"connection-max-idle-time"`
}

type GrafanaConfig struct {
	Url     string `mapstructure:"url"`
	Token   string `mapstructure:"token"`
	Timeout int    `mapstructure:"timeout"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	FileName string `mapstructure:"file"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type Config struct {
	MetircsDb DbConnectionConfig `mapstructure:"metrics-db"`
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

// Validate performs basic configuration validation
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate database configuration
	if c.MetircsDb.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if c.MetircsDb.Port <= 0 || c.MetircsDb.Port > 65535 {
		return fmt.Errorf("invalid database port: %d", c.MetircsDb.Port)
	}
	if c.MetircsDb.User == "" {
		return fmt.Errorf("database user is required")
	}
	if c.MetircsDb.DbName == "" {
		return fmt.Errorf("database name is required")
	}

	// Validate server configuration
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	return nil
}