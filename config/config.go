package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/CIDgravity/snakelet"
)

// Config will store the application config from config.toml file
type Config struct {
	API    APIConfig    `mapstructure:"API"`
	Github GithubConfig `mapstructure:"GITHUB"`
	Tasks  TasksConfig  `mapstructure:"TASKS"`
	Logs   LogsConfig   `mapstructure:"LOGS"`
}

type APIConfig struct {
	ListenPort string `mapstructure:"ListenPort"`
}

type TasksConfig struct {
	MaxParallelTasksAllowed int `mapstructure:"MaxParallelTasksAllowed"`
}

type GithubConfig struct {
	Token string `mapstructure:"Token"`
}

type LogsConfig struct {
	Level            string `mapstructure:"Level"` // error | warn | info - case insensitive
	OutputLogsAsJSON bool   `mapstructure:"OutputLogsAsJSON"`
}

// Load will open and parse config.toml content to Config struct instance
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

// GetDefault will convert a string to a valid Logrus level
func GetDefault() *Config {
	return &Config{
		API: APIConfig{
			ListenPort: "5000",
		},
		Github: GithubConfig{
			Token: "",
		},
		Tasks: TasksConfig{
			MaxParallelTasksAllowed: 8,
		},
		Logs: LogsConfig{
			Level:            "debug",
			OutputLogsAsJSON: false,
		},
	}
}
