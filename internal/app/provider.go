package app

import (
	"github.com/fingerprint/notetools/internal/llm"
	"github.com/fingerprint/notetools/internal/providers"
)

var knownProviders = []string{"claude", "codex", "opencode"}

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

func IsKnownProvider(name string) bool {
	for _, provider := range knownProviders {
		if provider == name {
			return true
		}
	}
	return false
}

func KnownProviders() []string {
	out := make([]string, len(knownProviders))
	copy(out, knownProviders)
	return out
}
