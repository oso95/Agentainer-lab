package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Storage  StorageConfig  `mapstructure:"storage"`
	Docker   DockerConfig   `mapstructure:"docker"`
	Security SecurityConfig `mapstructure:"security"`
}

type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type StorageConfig struct {
	DataDir string `mapstructure:"data_dir"`
}

type DockerConfig struct {
	Host string `mapstructure:"host"`
}

type SecurityConfig struct {
	DefaultToken string `mapstructure:"default_token"`
}

func LoadConfig() (*Config, error) {
	config := &Config{}
	
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.agentainer")
	viper.AddConfigPath("/etc/agentainer")

	viper.SetDefault("server.host", "localhost")
	viper.SetDefault("server.port", 8081)
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)
	// Use home directory for data by default
	homeDir, _ := os.UserHomeDir()
	defaultDataDir := filepath.Join(homeDir, ".agentainer", "data")
	viper.SetDefault("storage.data_dir", defaultDataDir)
	viper.SetDefault("docker.host", "unix:///var/run/docker.sock")
	viper.SetDefault("security.default_token", "agentainer-default-token")

	viper.SetEnvPrefix("AGENTAINER")
	viper.AutomaticEnv()
	
	// Explicitly bind environment variables
	viper.BindEnv("redis.host", "AGENTAINER_REDIS_HOST")
	viper.BindEnv("redis.port", "AGENTAINER_REDIS_PORT")
	viper.BindEnv("server.host", "AGENTAINER_SERVER_HOST")
	viper.BindEnv("server.port", "AGENTAINER_SERVER_PORT")
	viper.BindEnv("storage.data_dir", "AGENTAINER_STORAGE_DATA_DIR")
	viper.BindEnv("docker.host", "AGENTAINER_DOCKER_HOST")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand tilde in data directory path
	if strings.HasPrefix(config.Storage.DataDir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		config.Storage.DataDir = filepath.Join(homeDir, config.Storage.DataDir[2:])
	}

	if err := os.MkdirAll(config.Storage.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	return config, nil
}

func (c *Config) GetAgentConfigPath() string {
	return filepath.Join(c.Storage.DataDir, "agents.json")
}