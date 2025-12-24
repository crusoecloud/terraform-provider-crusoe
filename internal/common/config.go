package common

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

const (
	configFilePath = "/.crusoe/config" // full path is this appended to the user's home path

	defaultApiEndpoint = "https://api.crusoecloud.com/v1alpha5"
)

// Config holds options that can be set via ~/.crusoe/config and env variables.
type Config struct {
	ProfileName      string
	AccessKeyID      string `toml:"access_key_id"`
	SecretKey        string `toml:"secret_key"`
	SSHPublicKeyFile string `toml:"ssh_public_key_file"`
	ApiEndpoint      string `toml:"api_endpoint"`
	DefaultProject   string `toml:"default_project"`
}

// GetConfig populates a config struct based on default values, the user's Crusoe config file, and environment variables,
// in ascending priority. The config file used is ~/.crusoe/config.
func GetConfig() (*Config, error) {
	config := Config{
		ApiEndpoint: defaultApiEndpoint,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to find home dir: %w", err)
	}
	configPath := homeDir + configFilePath

	var rawData map[string]interface{}
	// Missing config/invalid config file is valid - credentials can come from env vars
	if _, err := toml.DecodeFile(configPath, &rawData); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Info: config file not found at %s\n", configPath)
			fmt.Fprintf(os.Stderr, "Using environment variables only.\n")
		} else {
			fmt.Fprintf(os.Stderr, "Warning: error reading config file at %s: %v\n", configPath, err)
			fmt.Fprintf(os.Stderr, "Continuing with environment variables only.\n")
		}
		rawData = make(map[string]interface{})
	}

	topLevelProfile := ""
	if profileVal, ok := rawData["profile"]; ok {
		if profileStr, ok := profileVal.(string); ok {
			topLevelProfile = profileStr
		}
	}

	profilesMap := make(map[string]Config)
	for key, val := range rawData {
		valMap, ok := val.(map[string]interface{})
		if !ok {
			continue
		}
		var profileConfig Config
		if accessKey, ok := valMap["access_key_id"].(string); ok {
			profileConfig.AccessKeyID = accessKey
		}
		if secretKey, ok := valMap["secret_key"].(string); ok {
			profileConfig.SecretKey = secretKey
		}
		if sshKey, ok := valMap["ssh_public_key_file"].(string); ok {
			profileConfig.SSHPublicKeyFile = sshKey
		}
		if apiEndpoint, ok := valMap["api_endpoint"].(string); ok {
			profileConfig.ApiEndpoint = apiEndpoint
		}
		if defaultProject, ok := valMap["default_project"].(string); ok {
			profileConfig.DefaultProject = defaultProject
		}
		profilesMap[key] = profileConfig
	}

	profileName := "default"
	if topLevelProfile != "" {
		profileName = topLevelProfile
	}
	if envProfile := os.Getenv("CRUSOE_PROFILE"); envProfile != "" {
		profileName = envProfile
	}

	config.ProfileName = profileName

	if profileConfig, ok := profilesMap[profileName]; ok {
		if profileConfig.AccessKeyID != "" {
			config.AccessKeyID = profileConfig.AccessKeyID
		}
		if profileConfig.SecretKey != "" {
			config.SecretKey = profileConfig.SecretKey
		}
		if profileConfig.SSHPublicKeyFile != "" {
			config.SSHPublicKeyFile = profileConfig.SSHPublicKeyFile
		}
		if profileConfig.DefaultProject != "" {
			config.DefaultProject = profileConfig.DefaultProject
		}
		if profileConfig.ApiEndpoint != "" {
			config.ApiEndpoint = profileConfig.ApiEndpoint
		}
	}

	accessKey := os.Getenv("CRUSOE_ACCESS_KEY_ID")
	secretKey := os.Getenv("CRUSOE_SECRET_KEY")
	apiEndpoint := os.Getenv("CRUSOE_API_ENDPOINT")
	defaultProject := os.Getenv("CRUSOE_DEFAULT_PROJECT")

	if accessKey != "" {
		config.AccessKeyID = accessKey
	}
	if secretKey != "" {
		config.SecretKey = secretKey
	}
	if apiEndpoint != "" {
		config.ApiEndpoint = apiEndpoint
	}
	if defaultProject != "" {
		config.DefaultProject = defaultProject
	}

	return &config, nil
}
