package config

import (
	"bytes"
	"elmon/logger"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type DbServers struct {
	Servers []DbConnectionConfig `mapstructure:"db-servers"`
}

func LoadDbServers(log *logger.Logger, configFilePath string) (*DbServers, error) {
    // 1. Load .env file (always first to ensure secrets are available in the OS environment)
    if err := godotenv.Load(); err != nil {
        log.Warn(".env file not found, using system environment variables for secrets")
    }

    // 2. Read the raw file content
    rawContent, err := os.ReadFile(configFilePath)
    if err != nil {
        log.Error(err, "failed to read config file", "config_file", configFilePath)
        return nil, err
    }

    // 3. Expand environment variables (this handles ${VAR} substitution)
    expandedContent := os.ExpandEnv(string(rawContent))

    // 4. Configure Viper rules (Best practice: configure before reading)
    viper.SetConfigType("yaml")

    // 5. Load the expanded content into Viper (Use ReadConfig from a buffer)
    err = viper.ReadConfig(bytes.NewBufferString(expandedContent))
    if err != nil {
        return nil, fmt.Errorf("failed to read expanded config into viper: %w", err)
    }
    
    // 6. Unmarshal the config
    var dbServersConfig *DbServers
    if err := viper.Unmarshal(&dbServersConfig); err != nil {
        log.Error(err, "failed to unmarshal config")
        return nil, err
    }
    
    // 7. Validate
    if err := dbServersConfig.Validate(log); err != nil {
        log.Error(err, "failed to validate config file")
        return nil, err
    }

    log.Info(fmt.Sprintf("Db servers config loaded from %s", configFilePath))

    return dbServersConfig, nil
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

