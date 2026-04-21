package app

import (
	"github.com/fingerprint/notetools/internal/llm"
	"github.com/fingerprint/notetools/internal/providers"
)

func ProviderFor(cfg Config, cmdName string) (llm.Provider, string) {
	cc := GetCommandConfig(cfg, cmdName)
	switch cc.Provider {
	case "claude":
		return providers.NewClaude(), cc.Model
	case "codex":
		return providers.NewCodex(), cc.Model
	case "opencode":
		return providers.NewOpenCode(), cc.Model
	default:
		return providers.NewOpenCode(), cc.Model
	}
}
