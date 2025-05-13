package pkg

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDirName  = ".doku"
	ConfigFileName = "config.yaml"
)

type Config struct {
	Dist     string            `yaml:"dist"`
	Driver   string            `yaml:"driver"`
	OS       string            `yaml:"os"`
	Arch     string            `yaml:"arch"`
	Settings map[string]string `yaml:"settings,omitempty"`
}

func (c *Config) AddValue(key, value string) {
	if c.Settings == nil {
		c.Settings = make(map[string]string)
	}
	c.Settings[key] = value
}

func (c *Config) SaveToFile(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func ConfigInit(overwrite bool, spinner *Spinner) error {

	if ConfigExists() && !overwrite {
		spinner.Info("Config already exists. Use --overwrite to replace it.")
		spinner.StopSilently()
		return nil
	}
	time.Sleep(1 * time.Second)

	cfg := &Config{
		Dist:   "minikube",
		Driver: "docker",
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		spinner.Error("failed to get home directory: %w", err)
		return err
	}

	configDir := filepath.Join(homeDir, ConfigDirName)
	err = os.MkdirAll(configDir, 0755)
	if err != nil {
		spinner.Error("failed to create config dir: %w", err)
		return err
	}

	// Full path to config.yaml
	configFilePath := filepath.Join(configDir, ConfigFileName)

	// Save to file
	err = cfg.SaveToFile(configFilePath)
	if err != nil {
		spinner.Error("failed to save config file: %w", err)
		return err
	}

	spinner.Info("Config file saved to: %s", configFilePath)
	return nil
}

func ConfigExists() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	configPath := filepath.Join(homeDir, ConfigDirName, ConfigFileName)
	_, err = os.Stat(configPath)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}
