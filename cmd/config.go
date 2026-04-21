package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/fingerprint/notetools/internal/app"
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
  gemini   - Gemini CLI

Examples:
  nt config set clean opencode opencode-go/glm-5.1
  nt config set execute opencode opencode-go/glm-5.1
  nt config set merge claude sonnet
  nt config set explain codex gpt-5-codex
  nt config set plan gemini auto`,
	Args: cobra.ExactArgs(3),
	RunE: runConfigSet,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := app.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	names := app.KnownCommandNames()
	sort.Strings(names)

	for _, n := range names {
		cc := app.GetCommandConfig(cfg, n)
		fmt.Printf("  %-10s  provider=%-10s  model=%s\n", n, cc.Provider, cc.Model)
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	cmdName, provider, model := args[0], args[1], args[2]

	if !app.IsKnownCommand(cmdName) {
		return fmt.Errorf("unknown command %q: must be one of %s", cmdName, strings.Join(app.KnownCommandNames(), ", "))
	}
	if !app.IsKnownProvider(provider) {
		return fmt.Errorf("unknown provider %q: must be one of %s", provider, strings.Join(app.KnownProviders(), ", "))
	}

	cfg, err := app.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cc := app.CommandConfig{Provider: provider, Model: model}
	if cfg.Commands == nil {
		cfg.Commands = map[string]app.CommandConfig{}
	}
	cfg.Commands[cmdName] = cc

	if err := app.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	b, _ := json.MarshalIndent(cc, "", "  ")
	fmt.Fprintf(os.Stderr, "Updated %s: %s\n", cmdName, string(b))
	return nil
}
