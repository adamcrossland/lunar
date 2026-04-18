package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the CLI configuration persisted to ~/.config/lunar/config.yaml.
type Config struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "lunar", "config.yaml"), nil
}

// Load reads the config file, returning defaults on any error.
func Load() (*Config, error) {
	cfg := &Config{Server: "http://localhost:3000"}
	path, err := configPath()
	if err != nil {
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return cfg, nil
	}
	if cfg.Server == "" {
		cfg.Server = "http://localhost:3000"
	}
	return cfg, nil
}

// Save writes the config to disk.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
