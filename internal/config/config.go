package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type CommandConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type Config struct {
	Commands map[string]CommandConfig `json:"commands"`
}

func Defaults() Config {
	return Config{
		Commands: map[string]CommandConfig{
			"ocr":    {Provider: "local", Model: "glm-ocr"},
			"review": {Provider: "local", Model: "gemma4:e4b"},
			"clean":  {Provider: "local", Model: "gemma4:e4b"},
			"merge":  {Provider: "local", Model: "gemma4:e4b"},
		},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}
	return filepath.Join(home, ".config", "nt", "config.json"), nil
}

func Load() (Config, error) {
	cfg := Defaults()

	path, err := configPath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config: %w", err)
	}

	// Fill in any missing defaults.
	defaultCfg := Defaults()
	for name, def := range defaultCfg.Commands {
		if _, ok := cfg.Commands[name]; !ok {
			cfg.Commands[name] = def
		}
	}

	return cfg, nil
}

func Save(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func GetCommandConfig(cfg Config, cmdName string) CommandConfig {
	if cc, ok := cfg.Commands[cmdName]; ok {
		return cc
	}
	defaultCfg := Defaults()
	if def, ok := defaultCfg.Commands[cmdName]; ok {
		return def
	}
	return CommandConfig{Provider: "local", Model: ""}
}
