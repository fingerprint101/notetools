# nt (notetools)

AI CLI for document explanation, transcript cleaning, and note merging. Routes requests through your choice of LLM CLI: [opencode](https://opencode.ai), [Claude Code](https://claude.ai/code), or [Codex](https://github.com/openai/codex).

- **`nt explain`** (`e`) — Identify sections in a PDF and explain each page
- **`nt preview`** (`p`) — Preview a file with line numbers (for selecting merge ranges)
- **`nt merge`** (`m`) — Merge snippets from two notes into one
- **`nt clean`** (`c`) — Section and clean a raw transcript

## Requirements

- Go 1.21+ (to build)
- At least one of: `opencode`, `claude` (Claude Code), or `codex` installed and authenticated

## Installation

```bash
git clone https://github.com/fingerprint/notetools
cd notetools
make build
sudo make install   # installs to /usr/local/bin/nt
```

## Configuration

Each command has a configurable provider/model. Defaults use `opencode-go/glm-5.1` via opencode.

```bash
nt config show
nt config set clean opencode opencode-go/glm-5.1
nt config set merge claude sonnet
nt config set explain codex gpt-5-codex
```

Supported providers:

| Provider   | Invoked as               | Notes |
|------------|--------------------------|-------|
| `opencode` | `opencode run ...`       | Multi-provider router |
| `claude`   | `claude -p ...`          | Claude Code CLI |
| `codex`    | `codex exec ...`         | Codex CLI |

For `claude` and `codex`, image inputs are passed as file paths in the prompt — the agent reads them itself.

## Usage

### Explain a PDF

```bash
nt explain lecture.pdf      # or: nt e lecture.pdf
# => lecture_explained.md
```

### Preview a file with line numbers

```bash
nt preview notes.md         # or: nt p notes.md
nt p notes.md:10-30         # lines 10 through 30
```

### Merge two notes

Use `preview` to find line ranges, then merge:

```bash
nt m alice_notes.md:10-85 bob_notes.md:30-120
nt m alice_notes.md bob_notes.md                       # full files
nt m alice_notes.md:10-85 bob_notes.md:30-120 -o combined.md
nt m alice_notes.md:10-85 bob_notes.md:30-120 -i "Focus on the chemistry section"
```

Merge preserves all details from both snippets without summarizing. Contradictions are marked with `<!-- CONFLICT -->` comments.

### Clean a transcript

```bash
nt clean lecture_transcript.txt    # or: nt c lecture_transcript.txt
# => lecture_transcript_cleaned.md
```

Splits the transcript into thematic sections and cleans each one individually.

### Global flags

```bash
nt --no-overwrite explain notes.pdf   # skip if output file exists
```
