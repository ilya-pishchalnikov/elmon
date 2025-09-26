package config

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3" // Используем популярную библиотеку для YAML
)

// Config represents the configuration structure.
// Add all your configuration fields here.
// yaml tags are used for unmarshalling the YAML file.
type Config struct {
	MetircsDb struct {
		Host           string `yaml:"host"`
		Port           int    `yaml:"port"`
		User           string `yaml:"user"`
		Password       string `yaml:"password"`
		DbName         string `yaml:"dbname"`
		HostAuthMethod string `yaml:"host-auth-method"`
	} `yaml:"metrics-db"`
	Server struct {
		Port string `yaml:"port"`
	} `yaml:"server"`
	Log struct {
		Level    string `yaml:"level"`
		Format   string `yaml:"format"`
		FileName string `yaml:"file"`
	} `yaml:"log"`
}

// configInstance holds the singleton instance of Config and a mutex for thread-safe lazy loading.
type configInstance struct {
	cfg  *Config
	once sync.Once
	err  error // To store error from first load
}

// globalConfig is the single instance of configInstance used throughout the application.
var globalConfig configInstance

// Load initializes the configuration by reading the YAML file at the specified path.
// It uses sync.Once to ensure the file is read only once.
// If config has already been loaded, it returns the cached instance.
// If an error occurs during loading, it caches the error and returns it on subsequent calls.
func Load(configFilePath string) (*Config, error) {
	globalConfig.once.Do(func() {
		data, err := os.ReadFile(configFilePath)
		if err != nil {
			globalConfig.err = fmt.Errorf("failed to read config file '%s': %w", configFilePath, err)
			return
		}

		globalConfig.cfg = &Config{}
		err = yaml.Unmarshal(data, globalConfig.cfg)
		if err != nil {
			globalConfig.err = fmt.Errorf("failed to unmarshal config file '%s': %w", configFilePath, err)
			globalConfig.cfg = nil // Ensure no partial config is returned
			return
		}

		// Optional: Perform any post-loading validation here
		// For example, checking if required fields are present
		if globalConfig.cfg.Server.Port == "" {
			globalConfig.err = fmt.Errorf("config validation error: server port is not defined")
			globalConfig.cfg = nil
			return
		}
		fmt.Printf("Configuration loaded successfully from %s\n", configFilePath)
	})

	return globalConfig.cfg, globalConfig.err
}

// GetConfig returns the loaded configuration.
// It's a convenience function that assumes Load has already been called
// or expects the caller to handle potential nil Config and errors.
// It's often used after initial Load call has been performed and validated.
func GetConfig() *Config {
	// IMPORTANT: This function assumes that `Load` has been called
	// and handled any potential errors.
	// Callers should typically call `Load` once at application startup
	// and then use `GetConfig` for subsequent accesses.
	if globalConfig.err != nil {
		// Log this or panic if it's an unrecoverable state,
		// as GetConfig usually implies config is already valid.
		fmt.Printf("Warning: GetConfig called but config loading previously failed: %v\n", globalConfig.err)
		return nil // Or panic(globalConfig.err)
	}
	return globalConfig.cfg
}

// ClearCache is a helper function for testing purposes,
// allowing to reset the singleton for fresh loading.
// Do NOT use in production unless you have a very specific reason.
func ClearCache() {
	globalConfig = configInstance{} // Resets the once.Do and cached values
}