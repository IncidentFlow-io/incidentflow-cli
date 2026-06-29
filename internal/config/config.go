package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

const (
	DefaultGatewayURL = "wss://gateway.incidentflow.io/agent-gateway/agents/ws"
	DefaultChartRef   = "oci://ghcr.io/incidentflow-io/charts/incidentflow-k8s-agent"
	DefaultNamespace  = "incidentflow-agent"
	DefaultRelease    = "incidentflow-k8s-agent"
)

// Env names
const (
	EnvProd  = "prod"
	EnvDev   = "dev"
	EnvLocal = "local"
)

type EnvConfig struct {
	AppURL string
	APIURL string
}

var envMap = map[string]EnvConfig{
	EnvProd: {
		AppURL: "https://app.incidentflow.io",
		APIURL: "https://platform-api.incidentflow.io",
	},
	EnvDev: {
		AppURL: "https://app-dev.incidentflow.io",
		APIURL: "https://dev.platform-api.incidentflow.io",
	},
	EnvLocal: {
		AppURL: "http://localhost:3001",
		APIURL: "http://localhost:8000",
	},
}

// EnvURLs returns the AppURL and APIURL for the given env name.
// Returns an error if the env name is not recognized.
func EnvURLs(env string) (EnvConfig, error) {
	e, ok := envMap[env]
	if !ok {
		return EnvConfig{}, fmt.Errorf("unknown environment %q — valid values: prod, dev, local", env)
	}
	return e, nil
}

type Config struct {
	Env       string `mapstructure:"env"       yaml:"env"`
	AppURL    string `mapstructure:"app_url"   yaml:"app_url"`
	APIURL    string `mapstructure:"api_url"   yaml:"api_url"`
	Workspace string `mapstructure:"workspace" yaml:"workspace"`
	Token     string `mapstructure:"token"     yaml:"token"`
}

// DefaultAPIURL returns the production API URL (used as fallback by other packages).
func DefaultAPIURL() string {
	return envMap[EnvProd].APIURL
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".incidentflow"), nil
}

func Load() (*Config, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(dir)
	viper.AutomaticEnv()
	viper.SetEnvPrefix("INCIDENTFLOW")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Back-fill defaults for configs written before env support
	if cfg.APIURL == "" {
		cfg.APIURL = envMap[EnvProd].APIURL
	}
	if cfg.AppURL == "" {
		cfg.AppURL = envMap[EnvProd].AppURL
	}
	if cfg.Env == "" {
		cfg.Env = EnvProd
	}

	return &cfg, nil
}

func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	path := filepath.Join(dir, "config.yaml")

	viper.Set("env", cfg.Env)
	viper.Set("app_url", cfg.AppURL)
	viper.Set("api_url", cfg.APIURL)
	viper.Set("workspace", cfg.Workspace)
	viper.Set("token", cfg.Token)

	if err := viper.WriteConfigAs(path); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return os.Chmod(path, 0600)
}

func (c *Config) IsAuthenticated() bool {
	return c.Token != "" && c.Workspace != ""
}
