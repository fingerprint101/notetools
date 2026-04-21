package app

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
			"clean":   {Provider: "opencode", Model: "opencode-go/glm-5.1"},
			"merge":   {Provider: "opencode", Model: "opencode-go/glm-5.1"},
			"explain": {Provider: "opencode", Model: "opencode-go/glm-5.1"},
			"plan":    {Provider: "opencode", Model: "opencode-go/glm-5.1"},
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
	return CommandConfig{Provider: "opencode", Model: ""}
}
