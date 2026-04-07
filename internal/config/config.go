package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	ConfigDir = ".config/docketeer"
	DataDir   = ".local/share/docketeer"
)

type Config struct {
	Sqlite struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"sqlite"`
	Postgres struct {
		ConnectionString string `mapstructure:"connection_string"`
	} `mapstructure:"postgres"`
}

func DefaultConfig() {
	home, _ := os.UserHomeDir()
	viper.SetDefault("sqlite.path", filepath.Join(home, DataDir, "docketeer.db"))
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	DefaultConfig()

	configFile := filepath.Join(home, ConfigDir, "config.yaml")
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")

	cfg := &Config{}

	if err := viper.ReadInConfig(); err == nil {
		viper.Unmarshal(cfg)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read config.yaml: %w", err)
	}

	viper.SetEnvPrefix("DOCKTEER")
	viper.AutomaticEnv()
	viper.Unmarshal(cfg)

	return cfg, nil
}

func Ensure() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	configDir := filepath.Join(home, ConfigDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")

	cfg, err := Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Println("\nNo configuration found.")

		fmt.Println("Enter PostgreSQL connection string or press Enter to use SQLite only:")
		fmt.Print("> ")

		var connString string
		fmt.Scanln(&connString)

		if connString != "" {
			cfg.Postgres.ConnectionString = connString
		}

		if err := writeConfig(configFile, cfg); err != nil {
			return err
		}
	}

	return nil
}

func writeConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

func (c *Config) GetBackend() string {
	if c.Postgres.ConnectionString != "" {
		return "postgres"
	}
	return "sqlite"
}

func (c *Config) GetPostgresConn() string {
	return c.Postgres.ConnectionString
}

func (c *Config) GetSQLitePath() string {
	return c.Sqlite.Path
}
