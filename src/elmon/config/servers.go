package config

import (
	"elmon/logger"
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type DbServers struct {
	Servers []DbConnectionConfig `mapstructure:"db-servers"`
}

func LoadDbServers(log *logger.Logger, configFilePath string) (*DbServers, error) {
	if err := godotenv.Load(); err != nil {
		log.Warn(".env file not found, using system environment variables for secrets")
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
		log.Error(err, "failed to read config file", "config_file", configFilePath)
		return nil, err
	}

	substituteEnvVars(log)

	var dbServersConfig *DbServers
	if err := viper.Unmarshal(&dbServersConfig); err != nil {
		log.Error(err, "failed to unmarshal config")
		return nil, err
	}
	
	if err := dbServersConfig.Validate(log); err!=nil {
		log.Error(err, "failed to validate config file")
		return nil, err
	}

	log.Info(fmt.Sprintf("Db servers config loaded from %s", configFilePath))

	return dbServersConfig, nil;
}

func (dbServers *DbServers) Validate (log *logger.Logger) error {
	
	names := make(map[string]bool)
	for i := range dbServers.Servers {
		dbServer := &dbServers.Servers[i]
		if err:=dbServer.Validate(log);err!=nil {
			log.Error(err, fmt.Sprintf("Error while validate config of server [%d] '%s'", i, dbServer.Name))
			return  err;
		}

		if names[dbServer.Name] {
            err := fmt.Errorf("duplicate db server name found: '%s'", dbServer.Name)
            log.Error(err, "config validation error: duplicate db server name")
            return err
		}

		names[dbServer.Name] = true

		log.Debug(fmt.Sprintf("Validated config of server [%d] '%s'", i, dbServer.Name))
	}

    return nil
}

// Get server by name
func (dbServers *DbServers) GetByName (name string) *DbConnectionConfig {
	for _, server := range dbServers.Servers {
		if server.Name == name {
			return &server
		}
	}
	return nil
}

