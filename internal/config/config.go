// Package config provides configuration loading for KubeDriftGuard.
// Configuration is loaded with the following precedence (highest to lowest):
//   - CLI flags
//   - Environment variables (prefixed with KUBEDRIFTGUARD_)
//   - config.yaml file
//   - Default values
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	// LogLevel controls the verbosity of logging (debug, info, warn, error).
	LogLevel string `mapstructure:"log_level"`

	// ScanInterval is how frequently drift scans are performed.
	ScanInterval time.Duration `mapstructure:"scan_interval"`

	// MetricsAddr is the address to bind the Prometheus metrics endpoint.
	MetricsAddr string `mapstructure:"metrics_addr"`

	// HealthAddr is the address to bind the health probe endpoint.
	HealthAddr string `mapstructure:"health_addr"`

	// GitSync holds configuration for the Git source synchronization.
	GitSync GitSyncConfig `mapstructure:"git_sync"`

	// Controller holds configuration for the Kubernetes controller.
	Controller ControllerConfig `mapstructure:"controller"`

	// Remediation holds configuration for the self-healing engine.
	Remediation RemediationConfig `mapstructure:"remediation"`

	// AgentsFile is the path to the AGENTS.md file for context engineering.
	AgentsFile string `mapstructure:"agents_file"`
}

// GitSyncConfig holds Git-related configuration.
type GitSyncConfig struct {
	// CacheDir is the local directory for caching cloned repositories.
	CacheDir string `mapstructure:"cache_dir"`

	// SyncInterval is how often to pull updates from Git remotes.
	SyncInterval time.Duration `mapstructure:"sync_interval"`

	// Timeout is the maximum time allowed for a single Git operation.
	Timeout time.Duration `mapstructure:"timeout"`
}

// ControllerConfig holds Kubernetes controller configuration.
type ControllerConfig struct {
	// MaxConcurrentReconciles is the number of concurrent reconcile loops.
	MaxConcurrentReconciles int `mapstructure:"max_concurrent_reconciles"`

	// Namespace restricts the controller to watch a specific namespace.
	// Leave empty to watch all namespaces.
	Namespace string `mapstructure:"namespace"`
}

// RemediationConfig holds self-healing configuration.
type RemediationConfig struct {
	// Enabled controls whether automatic remediation is active.
	Enabled bool `mapstructure:"enabled"`

	// DryRun logs remediation actions without applying them.
	DryRun bool `mapstructure:"dry_run"`

	// MaxRetries is the maximum number of remediation attempts per drift event.
	MaxRetries int `mapstructure:"max_retries"`
}

// Load reads configuration from the given file path, environment, and defaults.
func Load(cfgFile string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Read config file
	if cfgFile != "" {
		v.SetConfigFile(cfgFile)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("/etc/kubedriftguard")
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		// Config file not found is acceptable — we use defaults
	}

	// Read environment variables
	v.SetEnvPrefix("KUBEDRIFTGUARD")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets the default configuration values.
func setDefaults(v *viper.Viper) {
	v.SetDefault("log_level", "info")
	v.SetDefault("scan_interval", 30*time.Second)
	v.SetDefault("metrics_addr", ":8080")
	v.SetDefault("health_addr", ":8081")

	v.SetDefault("git_sync.cache_dir", "/tmp/kubedriftguard/repos")
	v.SetDefault("git_sync.sync_interval", 5*time.Minute)
	v.SetDefault("git_sync.timeout", 2*time.Minute)

	v.SetDefault("controller.max_concurrent_reconciles", 3)
	v.SetDefault("controller.namespace", "")

	v.SetDefault("remediation.enabled", true)
	v.SetDefault("remediation.dry_run", false)
	v.SetDefault("remediation.max_retries", 3)

	v.SetDefault("agents_file", "AGENTS.md")
}
