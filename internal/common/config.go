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
	AccessKeyID      string `toml:"access_key_id"`
	SecretKey        string `toml:"secret_key"`
	SSHPublicKeyFile string `toml:"ssh_public_key_file"`
	ApiEndpoint      string `toml:"api_endpoint"`
	DefaultProject   string `toml:"default_project"`
}

// GetConfig populates a config struct based on default values, the user's Crusoe config file, and environment variables,
// in ascending priority. The config file used is ~/.crusoe/config.
func GetConfig(profiles ...string) (*Config, error) {
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

	var profilesMap map[string]Config
	if _, err := toml.DecodeFile(configPath, &profilesMap); err != nil {
		if !os.IsNotExist(err) {
			// A real parsing error occurred, not just a missing file.
			return nil, fmt.Errorf("error parsing config file at %s: %w", configPath, err)
		}
		profilesMap = make(map[string]Config)
	}

	var topLevel struct {
		Profile string `toml:"profile"`
	}

	profileName := "default"

	// Priority 3: 'profile' key in config file
	if topLevel.Profile != "" {
		profileName = topLevel.Profile
	}

	// Priority 2: CRUSOE_PROFILE environment variable
	if envProfile := os.Getenv("CRUSOE_PROFILE"); envProfile != "" {
		profileName = envProfile
	}

	// Priority 1: Function argument
	if len(profiles) > 1 {
		return nil, fmt.Errorf("GetConfig accepts at most one profile name, but %d were provided", len(profiles))
	}

	if len(profiles) > 0 && profiles[0] != "" {
		profileName = profiles[0]
	}

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

	// Handle environment variables
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
