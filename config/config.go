package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/CIDgravity/snakelet"
)

// config structure
type Config struct {
	API   APIConfig   `mapstructure:"API"`
	Tasks TasksConfig `mapstructure:"TASKS"`
	Logs  LogsConfig  `mapstructure:"LOGS"`
}

type APIConfig struct {
	ListenPort string `mapstructure:"ListenPort"`
}

type TasksConfig struct {
	MaxParallelTasksAllowed int `mapstructure:"MaxParallelTasksAllowed"`
}

type LogsConfig struct {
	Level            string `mapstructure:"Level"` // error | warn | info - case insensitive
	OutputLogsAsJson bool   `mapstructure:"OutputLogsAsJson"`
}

// Load
func Load() (*Config, error) {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))

	if err != nil {
		return nil, err
	}

	// check config file exists
	configFilePath := dir + "/config/config.toml"

	if _, err := os.Stat(dir + "/config/config.toml"); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat("config/config.toml"); errors.Is(err, os.ErrNotExist) {
			return nil, err
		} else {
			configFilePath = "config/config.toml"
		}
	}

	// load default and config file content
	cfg := GetDefault()
	_, err = snakelet.InitAndLoad(cfg, configFilePath)

	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// GetDefault
func GetDefault() *Config {
	return &Config{
		API: APIConfig{
			ListenPort: "5000",
		},
		Tasks: TasksConfig{
			MaxParallelTasksAllowed: 8,
		},
		Logs: LogsConfig{
			Level:            "debug",
			OutputLogsAsJson: false,
		},
	}
}
