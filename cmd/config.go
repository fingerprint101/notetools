package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

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
	Use:   "set <command> <provider> [model]",
	Short: "Set provider and model for a command",
	Long: `Set the inference provider and model for a specific command.

Providers:
  local    - Use local ollama models (default)
  opencode - Use the opencode CLI (must be installed)

Examples:
  nt config set ocr local glm-ocr
  nt config set clean opencode opencode-go/glm-5.1
  nt config set review local              (uses default model)`,
	Args: cobra.MinimumNArgs(2),
	RunE: runConfigSet,
}

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
	cmdName := args[0]
	provider := args[1]

	if provider != "local" && provider != "opencode" {
		return fmt.Errorf("unknown provider %q: must be \"local\" or \"opencode\"", provider)
	}

	validCmds := map[string]bool{"ocr": true, "review": true, "clean": true, "merge": true}
	if !validCmds[cmdName] {
		return fmt.Errorf("unknown command %q: must be one of ocr, review, clean, merge", cmdName)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	var model string
	switch {
	case len(args) >= 3:
		model = args[2]
	case provider == "local":
		defaults := config.Defaults()
		if def, ok := defaults.Commands[cmdName]; ok {
			model = def.Model
		}
	default:
		return fmt.Errorf("model is required when provider is \"opencode\"; usage: nt config set %s opencode <model>", cmdName)
	}

	if provider == "opencode" {
		if err := validateOpenCodeModel(model); err != nil {
			return err
		}
	}

	cc := config.CommandConfig{
		Provider: provider,
		Model:    model,
	}
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

func validateOpenCodeModel(model string) error {
	out, err := exec.Command("opencode", "models").Output()
	if err != nil {
		return fmt.Errorf("failed to list opencode models (is opencode installed?): %w", err)
	}
	available := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, m := range available {
		m = strings.TrimSpace(m)
		if m == model {
			return nil
		}
	}
	return fmt.Errorf("model %q not found in opencode models; available models:\n  %s", model, strings.Join(available, "\n  "))
}
