package configlog

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type LogConfig struct {
	Level    string `mapstructure:"level"`  // default error
	Format   string `mapstructure:"format"` // default json
	FileName string `mapstructure:"file"`
}

type configInstance struct {
	config *LogConfig
	once   sync.Once
	err    error
}

var globalConfig configInstance

func Load( configFilePath string) (*LogConfig, error) {
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

		globalConfig.config = &LogConfig{}
		if err := viper.Unmarshal(globalConfig.config); err != nil {
			globalConfig.err = fmt.Errorf("failed to unmarshal config: %w", err)
			fmt.Printf("failed to unmarshal config: %s/n", err.Error())
			globalConfig.config = nil
			return
		}
		
		if err := globalConfig.config.Validate(); err!=nil {
			globalConfig.err = fmt.Errorf("failed to validate config file: %w", err)
			return
		}
	})

	return globalConfig.config, globalConfig.err
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

// GetConfig returns the loaded configuration
func GetConfig() *LogConfig {
	if globalConfig.err != nil {
		fmt.Printf("GetConfig called but log config loading previously failed: %s/n", globalConfig.err.Error())
		return nil
	}
	return globalConfig.config
}

// ClearCache resets the singleton for testing purposes
func ClearCache() {
	globalConfig = configInstance{}
}
