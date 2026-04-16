package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/fingerprint/notetools/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage nt configuration",
	Long:  "View or modify notetools configuration, including provider and model settings per command.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <command> <provider> <model>",
	Short: "Set provider and model for a command",
	Long: `Set the inference provider and model for a specific command.

Providers:
  opencode - opencode CLI
  claude   - Claude Code CLI
  codex    - Codex CLI

Examples:
  nt config set clean opencode opencode-go/glm-5.1
  nt config set merge claude sonnet
  nt config set explain codex gpt-5-codex`,
	Args: cobra.ExactArgs(3),
	RunE: runConfigSet,
}

var validCmds = map[string]bool{"clean": true, "merge": true, "explain": true, "plan": true}
var validProviders = map[string]bool{"opencode": true, "claude": true, "codex": true}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	names := make([]string, 0, len(cfg.Commands))
	for n := range cfg.Commands {
		names = append(names, n)
	}
	sort.Strings(names)

	for _, n := range names {
		cc := cfg.Commands[n]
		fmt.Printf("  %-10s  provider=%-10s  model=%s\n", n, cc.Provider, cc.Model)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cmdName, provider, model := args[0], args[1], args[2]

	if !validCmds[cmdName] {
		return fmt.Errorf("unknown command %q: must be one of clean, merge, explain", cmdName)
	}
	if !validProviders[provider] {
		return fmt.Errorf("unknown provider %q: must be one of opencode, claude, codex", provider)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cc := config.CommandConfig{Provider: provider, Model: model}
	if cfg.Commands == nil {
		cfg.Commands = map[string]config.CommandConfig{}
	}
	cfg.Commands[cmdName] = cc

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	b, _ := json.MarshalIndent(cc, "", "  ")
	fmt.Fprintf(os.Stderr, "Updated %s: %s\n", cmdName, string(b))
	return nil
}
