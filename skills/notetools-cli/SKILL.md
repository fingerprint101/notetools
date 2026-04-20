---
name: notetools-cli
description: Use when a coding agent should operate the `nt` CLI for PDF explanation, transcript cleaning, note preview, merge planning, merge execution, or provider configuration. Trigger this skill for requests involving `nt explain`, `nt clean`, `nt preview`, `nt plan`, `nt merge`, or `nt config`, especially when the agent should identify sections to merge, choose safe command arguments, and verify generated Markdown outputs.
---

# Notetools CLI

## Overview

This skill helps Codex use the `nt` CLI correctly and with minimal guesswork. It covers the main command flows, the expected inputs and outputs, and the checks to run before and after invoking the tool.

## When To Use

Use this skill when the user wants to:

- explain a PDF into Markdown with `nt explain` or `nt e`
- clean a raw transcript with `nt clean` or `nt c`
- preview a text or Markdown file with line numbers using `nt preview` or `nt p`
- plan which source sections map to which target sections before merging, using `nt plan`
- merge two note sources with `nt merge` or `nt m`
- inspect or change which provider and model `nt` uses via `nt config`
- debug an `nt` command failure and patch the Go project if the CLI behavior is wrong

Do not use this skill for general PDF editing, generic OCR, or note-processing tasks that do not involve the `nt` binary.

## Command Map

Choose the subcommand from the user’s intent:

- `nt e <pdf>`: identify sections in a PDF, render pages, and write `{stem}_explained.md` unless `-o` is provided
- `nt c <transcript>`: split and clean a raw transcript into `{stem}_cleaned.md`
- `nt p <file>` or `nt p <file:start-end>`: print a file with line numbers, optionally constrained to a range
- `nt plan <source.md> <target.md>`: identify which sections in the source already exist in the target and where missing sections should be inserted
- `nt m <left> <right>`: merge two note inputs, where each input may be either a full file path or a `file:start-end` slice
- `nt config show`: inspect provider and model bindings
- `nt config set <command> <provider> <model>`: switch providers or models for a command

## Operating Procedure

1. Confirm the relevant input file exists before running `nt`.
2. If the request implies a generated file, infer the default output path from the input stem unless the user supplied `-o`.
3. Prefer `./nt` in the repo root when working inside the project and testing local changes. Use installed `nt` only when the task is explicitly about the installed binary.
4. For note-integration work, use `nt plan` before `nt merge` when the job is to identify which sections should be merged or where missing content belongs.
5. After the command runs, verify that the expected output file exists and inspect the first section or first lines for obvious formatting failures.

## Common Flows

### Explain A PDF

Run:

```bash
./nt e lecture.pdf
./nt e lecture.pdf -o lecture_explained.md
```

Checks:

- the PDF path exists
- the output Markdown file was created
- stderr progress does not stop at section identification or page rendering

If explanation fails under the Codex provider, inspect `internal/codex/client.go` and `internal/explain/explain.go` first.

### Preview Before Merge

Run:

```bash
./nt p notes_a.md:1-80
./nt p notes_b.md:40-140
```

Use preview output to choose concrete merge slices, then run:

```bash
./nt m notes_a.md:10-85 notes_b.md:30-120
./nt m notes_a.md:10-85 notes_b.md:30-120 -o combined.md
```

If the user gives extra guidance, pass it with `-i`.

### Plan A Merge

Run:

```bash
./nt plan source.md target.md
./nt plan source.md target.md -o plan-source-target.md
```

Use this when the task is to identify sections to merge rather than to perform the merge immediately.

Checks:

- the plan file was created
- each source section is mapped either to a target range or to an insertion point
- the output is usable as input to a later `preview` and `merge` workflow

### Clean A Transcript

Run:

```bash
./nt c lecture_transcript.txt
```

Checks:

- the cleaned Markdown file exists
- sections are coherent and not duplicated
- the output is not truncated or empty

## Provider Notes

- Runtime configuration lives in `~/.config/nt/config.json`.
- Inspect active settings with `nt config show` before debugging provider-specific failures.
- `opencode`, `claude`, and `codex` are supported LLM backends.
- For `claude` and `codex`, image paths are passed in prompts and read by the external agent CLI.
- When patching provider adapters, keep behavior aligned across providers where practical, especially around JSON extraction and image handling.

## Debugging Checklist

When an `nt` command fails:

1. Re-run the exact command and capture stderr.
2. Check whether the failure is input-related, provider-related, or output-path-related.
3. Inspect the matching command file in `cmd/` and the backing logic in `internal/`.
4. For Codex-specific issues, verify the external CLI invocation still matches the installed `codex --help` and `codex exec --help` surface.
5. After a fix, rebuild `./nt` and run the smallest command that reproduces the original issue.

## Repo Pointers

- `cmd/`: command wiring, arguments, progress messages, output file handling
- `internal/plan/`: section extraction and semantic mapping for merge planning
- `internal/explain/`: section identification and section explanation prompts
- `internal/clean/`: transcript sectioning and cleaning flow
- `internal/merge/`: note merge logic
- `internal/pdf/`: PDF page rendering
- `internal/opencode/`, `internal/claude/`, `internal/codex/`: provider adapters

## Validation

Use these checks after edits:

```bash
env GOCACHE=/tmp/go-build go test ./...
env GOCACHE=/tmp/go-build go build -o nt .
```

For manual validation, run one relevant end-to-end CLI command tied to the changed path.
