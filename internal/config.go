package internal

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	configFilePath = "/.crusoe/config" // full path is this appended to the user's home path

	defaultApiEndpoint = "https://api.crusoecloud.com/v1alpha4"
)

// Config holds options that can be set via ~/.crusoe/config and env variables.
type Config struct {
	AccessKeyID      string `toml:"access_key_id"`
	SecretKey        string `toml:"secret_key"`
	SSHPublicKeyFile string `toml:"ssh_public_key_file"`
	ApiEndpoint      string `toml:"api_endpoint"`
}

// ConfigFile reflects the structure of a valid Crusoe config, which should have a default profile at the root level.
type ConfigFile struct {
	Default Config
}

// GetConfig populates a config struct based on default values, the user's Crusoe config file, and environment variables,
// in ascending priority. The config file used is ~/.crusoe/config.
// TODO: add support
func GetConfig() (*Config, error) {
	config := Config{
		ApiEndpoint: defaultApiEndpoint,
	}

	// Parse config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// failing to get the home dir is worth surfacing an error
		return nil, fmt.Errorf("failed to find home dir: %w", err)
	}
	configPath := homeDir + configFilePath

	var configFile ConfigFile
	_, err = toml.DecodeFile(configPath, &configFile)
	if err == nil { // just skip if error - not having a config file is valid
		config.AccessKeyID = configFile.Default.AccessKeyID
		config.SecretKey = configFile.Default.SecretKey
		config.SSHPublicKeyFile = configFile.Default.SSHPublicKeyFile

		if configFile.Default.ApiEndpoint != "" {
			config.ApiEndpoint = configFile.Default.ApiEndpoint
		}
	}

	// Handle environment variables
	accessKey := os.Getenv("CRUSOE_ACCESS_KEY_ID")
	secretKey := os.Getenv("CRUSOE_SECRET_KEY")
	apiEndpoint := os.Getenv("CRUSOE_API_ENDPOINT")

	if accessKey != "" {
		config.AccessKeyID = accessKey
	}
	if secretKey != "" {
		config.SecretKey = secretKey
	}
	if apiEndpoint != "" {
		config.ApiEndpoint = apiEndpoint
	}

	return &config, nil
}
