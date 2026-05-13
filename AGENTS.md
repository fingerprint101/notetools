# Repository Guidelines

## Project Structure & Module Organization
`main.go` is the CLI entrypoint. User-facing commands live in `cmd/` and should stay thin: argument parsing, file I/O, and progress messages belong there. Core behavior lives in grouped `internal/` packages: `internal/notes` for note workflows, `internal/docs` for PDF explanation and rendering, `internal/providers` for LLM backends, and `internal/app` for runtime configuration and provider selection. Sample inputs and expected outputs for manual checks live in `tests/`.

## Coding Style & Naming Conventions
This is a Go 1.21+ project. Format all edited Go files with `gofmt -w`. Follow standard Go naming: exported identifiers use `CamelCase`, unexported helpers use `camelCase`, and package names stay short and lowercase. Keep command packages focused on orchestration and push prompt logic, parsing, and provider-specific behavior into `internal/`. Prefer explicit errors with context such as `fmt.Errorf("sectioning failed: %w", err)`.

## Commit & Pull Request Guidelines
Recent commits use short, imperative summaries such as `merge planning command` and `improved merge command with diff view`. Keep commit subjects concise and descriptive. PRs should explain the user-visible change, note any prompt or provider behavior changes, include the verification commands you ran, and attach sample output when formatting or generated Markdown changes.

## Configuration & Provider Notes
Runtime configuration is stored in `~/.config/nt/config.json`. Do not commit secrets, API keys, or machine-specific provider settings. When changing provider adapters, preserve consistent behavior across `opencode`, `claude`, and `codex`, especially for JSON output and image handling.
