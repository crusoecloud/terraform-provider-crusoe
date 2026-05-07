package common

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempConfig creates a temp directory, writes TOML content to a config file, and returns the path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	return path
}

// clearCrusoeEnvVars unsets all CRUSOE_* env vars and restores them on cleanup.
func clearCrusoeEnvVars(t *testing.T) {
	t.Helper()

	envVars := []string{
		"CRUSOE_PROFILE",
		"CRUSOE_DEFAULT_PROJECT",
		"CRUSOE_ACCESS_KEY_ID",
		"CRUSOE_SECRET_KEY",
		"CRUSOE_API_ENDPOINT",
	}

	saved := make(map[string]string)
	for _, key := range envVars {
		saved[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	t.Cleanup(func() {
		for _, key := range envVars {
			if val, ok := saved[key]; ok && val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	})
}

func TestProfilePrecedence_FullChain(t *testing.T) {
	configContent := `
profile = "config-top-level"

[default]
access_key_id = "default-key"
secret_key = "default-secret"

[config-top-level]
access_key_id = "toplevel-key"
secret_key = "toplevel-secret"

[env-profile]
access_key_id = "env-key"
secret_key = "env-secret"

[opts-profile]
access_key_id = "opts-key"
secret_key = "opts-secret"
`

	t.Run("falls back to default when nothing specified", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, `
[default]
access_key_id = "default-key"
secret_key = "default-secret"
`)
		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.ProfileName != "default" {
			t.Errorf("ProfileName: got %q, want %q", config.ProfileName, "default")
		}
		if config.AccessKeyID != "default-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "default-key")
		}
	})

	t.Run("config file top-level profile key overrides default", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.ProfileName != "config-top-level" {
			t.Errorf("ProfileName: got %q, want %q", config.ProfileName, "config-top-level")
		}
		if config.AccessKeyID != "toplevel-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "toplevel-key")
		}
	})

	t.Run("CRUSOE_PROFILE env overrides config file top-level key", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_PROFILE", "env-profile")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.ProfileName != "env-profile" {
			t.Errorf("ProfileName: got %q, want %q", config.ProfileName, "env-profile")
		}
		if config.AccessKeyID != "env-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "env-key")
		}
	})

	t.Run("opts.Profile overrides env var and config file", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_PROFILE", "env-profile")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{
			ConfigPath: configPath,
			Profile:    "opts-profile",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.ProfileName != "opts-profile" {
			t.Errorf("ProfileName: got %q, want %q", config.ProfileName, "opts-profile")
		}
		if config.AccessKeyID != "opts-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "opts-key")
		}
	})
}

func TestProjectPrecedence_FullChain(t *testing.T) {
	configContent := `
[default]
access_key_id = "key"
secret_key = "secret"
default_project = "profile-project"
`

	t.Run("profile default_project used when no opts or env", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultProject != "profile-project" {
			t.Errorf("DefaultProject: got %q, want %q", config.DefaultProject, "profile-project")
		}
	})

	t.Run("CRUSOE_DEFAULT_PROJECT env overrides profile default_project", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_DEFAULT_PROJECT", "env-project")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultProject != "env-project" {
			t.Errorf("DefaultProject: got %q, want %q", config.DefaultProject, "env-project")
		}
	})

	t.Run("opts.Project overrides both env and profile", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_DEFAULT_PROJECT", "env-project")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{
			ConfigPath: configPath,
			Project:    "opts-project",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.DefaultProject != "opts-project" {
			t.Errorf("DefaultProject: got %q, want %q", config.DefaultProject, "opts-project")
		}
	})
}

func TestCredentialEnvOverridesProfile(t *testing.T) {
	configContent := `
[default]
access_key_id = "profile-access-key"
secret_key = "profile-secret-key"
api_endpoint = "https://profile-endpoint/v1"
`

	t.Run("all env vars override profile credentials", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_ACCESS_KEY_ID", "env-access-key")
		os.Setenv("CRUSOE_SECRET_KEY", "env-secret-key")
		os.Setenv("CRUSOE_API_ENDPOINT", "https://env-endpoint/v1")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.AccessKeyID != "env-access-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "env-access-key")
		}
		if config.SecretKey != "env-secret-key" {
			t.Errorf("SecretKey: got %q, want %q", config.SecretKey, "env-secret-key")
		}
		if config.ApiEndpoint != "https://env-endpoint/v1" {
			t.Errorf("ApiEndpoint: got %q, want %q", config.ApiEndpoint, "https://env-endpoint/v1")
		}
	})

	t.Run("profile credentials used when env vars unset", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.AccessKeyID != "profile-access-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "profile-access-key")
		}
		if config.SecretKey != "profile-secret-key" {
			t.Errorf("SecretKey: got %q, want %q", config.SecretKey, "profile-secret-key")
		}
		if config.ApiEndpoint != "https://profile-endpoint/v1" {
			t.Errorf("ApiEndpoint: got %q, want %q", config.ApiEndpoint, "https://profile-endpoint/v1")
		}
	})

	t.Run("partial override - only access key from env", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		os.Setenv("CRUSOE_ACCESS_KEY_ID", "env-access-key")
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.AccessKeyID != "env-access-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "env-access-key")
		}
		if config.SecretKey != "profile-secret-key" {
			t.Errorf("SecretKey should come from profile: got %q, want %q", config.SecretKey, "profile-secret-key")
		}
		if config.ApiEndpoint != "https://profile-endpoint/v1" {
			t.Errorf("ApiEndpoint should come from profile: got %q, want %q", config.ApiEndpoint, "https://profile-endpoint/v1")
		}
	})
}

func TestDefaultApiEndpoint(t *testing.T) {
	clearCrusoeEnvVars(t)
	configPath := writeTempConfig(t, `
[default]
access_key_id = "key"
secret_key = "secret"
`)

	config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://api.crusoecloud.com/v1"
	if config.ApiEndpoint != expected {
		t.Errorf("ApiEndpoint: got %q, want %q", config.ApiEndpoint, expected)
	}
}

func TestMissingConfigFile(t *testing.T) {
	clearCrusoeEnvVars(t)
	os.Setenv("CRUSOE_ACCESS_KEY_ID", "env-key")
	os.Setenv("CRUSOE_SECRET_KEY", "env-secret")

	nonexistentPath := filepath.Join(t.TempDir(), "nonexistent", "config")

	config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: nonexistentPath})
	if err != nil {
		t.Fatalf("missing config file should not cause error: %v", err)
	}

	if config.ProfileName != "default" {
		t.Errorf("ProfileName should fall back to default: got %q", config.ProfileName)
	}
	if config.AccessKeyID != "env-key" {
		t.Errorf("AccessKeyID should come from env: got %q, want %q", config.AccessKeyID, "env-key")
	}
	if config.SecretKey != "env-secret" {
		t.Errorf("SecretKey should come from env: got %q, want %q", config.SecretKey, "env-secret")
	}
}

func TestInvalidConfigFile(t *testing.T) {
	clearCrusoeEnvVars(t)
	os.Setenv("CRUSOE_ACCESS_KEY_ID", "env-key")
	os.Setenv("CRUSOE_SECRET_KEY", "env-secret")

	configPath := writeTempConfig(t, `this is not valid toml {{{{`)

	config, err := GetConfigWithOptions(ConfigOptions{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("invalid config file should not cause error: %v", err)
	}

	if config.AccessKeyID != "env-key" {
		t.Errorf("AccessKeyID should come from env: got %q, want %q", config.AccessKeyID, "env-key")
	}
	if config.SecretKey != "env-secret" {
		t.Errorf("SecretKey should come from env: got %q, want %q", config.SecretKey, "env-secret")
	}
}

func TestProfileNotFoundInConfig(t *testing.T) {
	clearCrusoeEnvVars(t)
	configPath := writeTempConfig(t, `
[default]
access_key_id = "default-key"
secret_key = "default-secret"
`)

	config, err := GetConfigWithOptions(ConfigOptions{
		ConfigPath: configPath,
		Profile:    "nonexistent-profile",
	})
	if err != nil {
		t.Fatalf("nonexistent profile should not cause error: %v", err)
	}

	if config.ProfileName != "nonexistent-profile" {
		t.Errorf("ProfileName should be set to requested profile: got %q, want %q",
			config.ProfileName, "nonexistent-profile")
	}
	if config.AccessKeyID != "" {
		t.Errorf("AccessKeyID should be empty for nonexistent profile: got %q", config.AccessKeyID)
	}
	if config.SecretKey != "" {
		t.Errorf("SecretKey should be empty for nonexistent profile: got %q", config.SecretKey)
	}
}

func TestMultipleProfiles(t *testing.T) {
	configContent := `
[production]
access_key_id = "prod-key"
secret_key = "prod-secret"
default_project = "prod-project"
api_endpoint = "https://api.prod.example.com/v1"

[staging]
access_key_id = "staging-key"
secret_key = "staging-secret"
default_project = "staging-project"
api_endpoint = "https://api.staging.example.com/v1"
`

	t.Run("load production profile", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{
			ConfigPath: configPath,
			Profile:    "production",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.AccessKeyID != "prod-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "prod-key")
		}
		if config.SecretKey != "prod-secret" {
			t.Errorf("SecretKey: got %q, want %q", config.SecretKey, "prod-secret")
		}
		if config.DefaultProject != "prod-project" {
			t.Errorf("DefaultProject: got %q, want %q", config.DefaultProject, "prod-project")
		}
		if config.ApiEndpoint != "https://api.prod.example.com/v1" {
			t.Errorf("ApiEndpoint: got %q, want %q", config.ApiEndpoint, "https://api.prod.example.com/v1")
		}
	})

	t.Run("load staging profile", func(t *testing.T) {
		clearCrusoeEnvVars(t)
		configPath := writeTempConfig(t, configContent)

		config, err := GetConfigWithOptions(ConfigOptions{
			ConfigPath: configPath,
			Profile:    "staging",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if config.AccessKeyID != "staging-key" {
			t.Errorf("AccessKeyID: got %q, want %q", config.AccessKeyID, "staging-key")
		}
		if config.SecretKey != "staging-secret" {
			t.Errorf("SecretKey: got %q, want %q", config.SecretKey, "staging-secret")
		}
		if config.DefaultProject != "staging-project" {
			t.Errorf("DefaultProject: got %q, want %q", config.DefaultProject, "staging-project")
		}
		if config.ApiEndpoint != "https://api.staging.example.com/v1" {
			t.Errorf("ApiEndpoint: got %q, want %q", config.ApiEndpoint, "https://api.staging.example.com/v1")
		}
	})
}

// Retained from the original test file: backward-compatibility and basic opts-vs-env tests.

func TestGetConfigWithOptions_ProfilePrecedence(t *testing.T) {
	origProfile := os.Getenv("CRUSOE_PROFILE")
	defer os.Setenv("CRUSOE_PROFILE", origProfile)

	t.Run("opts.Profile takes precedence over env var", func(t *testing.T) {
		os.Setenv("CRUSOE_PROFILE", "env-profile")

		opts := ConfigOptions{
			Profile: "opts-profile",
		}

		config, err := GetConfigWithOptions(opts)
		if err != nil {
			t.Fatalf("GetConfigWithOptions failed: %v", err)
		}

		if config.ProfileName != "opts-profile" {
			t.Errorf("Provider block profile should override env var: got %q, want %q",
				config.ProfileName, "opts-profile")
		}
	})

	t.Run("env var used when opts.Profile is empty", func(t *testing.T) {
		os.Setenv("CRUSOE_PROFILE", "env-profile")

		opts := ConfigOptions{
			Profile: "",
		}

		config, err := GetConfigWithOptions(opts)
		if err != nil {
			t.Fatalf("GetConfigWithOptions failed: %v", err)
		}

		if config.ProfileName != "env-profile" {
			t.Errorf("Env var should be used when opts is empty: got %q, want %q",
				config.ProfileName, "env-profile")
		}
	})
}

func TestGetConfigWithOptions_ProjectPrecedence(t *testing.T) {
	origProject := os.Getenv("CRUSOE_DEFAULT_PROJECT")
	defer os.Setenv("CRUSOE_DEFAULT_PROJECT", origProject)

	t.Run("opts.Project takes precedence over env var", func(t *testing.T) {
		os.Setenv("CRUSOE_DEFAULT_PROJECT", "env-project")

		opts := ConfigOptions{
			Project: "opts-project",
		}

		config, err := GetConfigWithOptions(opts)
		if err != nil {
			t.Fatalf("GetConfigWithOptions failed: %v", err)
		}

		if config.DefaultProject != "opts-project" {
			t.Errorf("Provider block project should override env var: got %q, want %q",
				config.DefaultProject, "opts-project")
		}
	})

	t.Run("env var used when opts.Project is empty", func(t *testing.T) {
		os.Setenv("CRUSOE_DEFAULT_PROJECT", "env-project")

		opts := ConfigOptions{
			Project: "",
		}

		config, err := GetConfigWithOptions(opts)
		if err != nil {
			t.Fatalf("GetConfigWithOptions failed: %v", err)
		}

		if config.DefaultProject != "env-project" {
			t.Errorf("Env var should be used when opts is empty: got %q, want %q",
				config.DefaultProject, "env-project")
		}
	})
}

func TestGetConfig_BackwardCompatibility(t *testing.T) {
	origProfile := os.Getenv("CRUSOE_PROFILE")
	origProject := os.Getenv("CRUSOE_DEFAULT_PROJECT")
	defer func() {
		os.Setenv("CRUSOE_PROFILE", origProfile)
		os.Setenv("CRUSOE_DEFAULT_PROJECT", origProject)
	}()

	os.Setenv("CRUSOE_PROFILE", "test-profile")
	os.Setenv("CRUSOE_DEFAULT_PROJECT", "test-project")

	config1, err1 := GetConfig()
	config2, err2 := GetConfigWithOptions(ConfigOptions{})

	if (err1 == nil) != (err2 == nil) {
		t.Errorf("GetConfig and GetConfigWithOptions({}) returned different error states: err1=%v, err2=%v", err1, err2)
	}

	if config1 == nil || config2 == nil {
		return
	}

	if config1.ProfileName != config2.ProfileName {
		t.Errorf("ProfileName mismatch: GetConfig()=%q, GetConfigWithOptions({})=%q",
			config1.ProfileName, config2.ProfileName)
	}

	if config1.DefaultProject != config2.DefaultProject {
		t.Errorf("DefaultProject mismatch: GetConfig()=%q, GetConfigWithOptions({})=%q",
			config1.DefaultProject, config2.DefaultProject)
	}
}

func TestConfigOptions_EmptyStringsAsNotSpecified(t *testing.T) {
	origProfile := os.Getenv("CRUSOE_PROFILE")
	origProject := os.Getenv("CRUSOE_DEFAULT_PROJECT")
	defer func() {
		os.Setenv("CRUSOE_PROFILE", origProfile)
		os.Setenv("CRUSOE_DEFAULT_PROJECT", origProject)
	}()

	os.Setenv("CRUSOE_PROFILE", "fallback-profile")
	os.Setenv("CRUSOE_DEFAULT_PROJECT", "fallback-project")

	opts := ConfigOptions{
		Profile: "",
		Project: "",
	}

	config, err := GetConfigWithOptions(opts)
	if err != nil {
		t.Logf("GetConfigWithOptions returned error (expected in test env): %v", err)
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if config.ProfileName != "fallback-profile" {
		t.Errorf("Empty Profile should fall through to env var: got %q, want %q",
			config.ProfileName, "fallback-profile")
	}

	if config.DefaultProject != "fallback-project" {
		t.Errorf("Empty Project should fall through to env var: got %q, want %q",
			config.DefaultProject, "fallback-project")
	}
}
