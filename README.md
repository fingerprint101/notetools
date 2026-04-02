# nt (notetools)

Local AI CLI for OCR, document review, and note merging — powered by [ollama](https://ollama.com).

- **`nt ocr`** (`o`) — Convert PDFs to Markdown using GLM-OCR
- **`nt review`** (`r`) — Review Markdown for consistency
- **`nt preview`** (`p`) — Preview a file with line numbers (for selecting merge ranges)
- **`nt merge`** (`m`) — Merge snippets from two notes into one
- **`nt clean`** (`c`) — Section and clean a raw transcript

## Requirements

- Go 1.21+ (to build)
- [ollama](https://ollama.com) 0.19+ with `glm-ocr` and `gemma4:e4b` models pulled

## Installation

### 1. Install ollama

Download and install ollama from [ollama.com](https://ollama.com), then pull the required models:

```bash
ollama pull glm-ocr
ollama pull gemma4:e4b
```

### 2. Build nt

```bash
git clone https://github.com/fingerprint/notetools
cd notetools
make build
sudo make install   # installs to /usr/local/bin/nt
```

## Usage

All commands have short aliases shown in parentheses.

### OCR a PDF

```bash
nt ocr lecture_notes.pdf    # or: nt o lecture_notes.pdf
# => lecture_notes.md
```

### Review a Markdown file

```bash
nt review lecture_notes.md  # or: nt r lecture_notes.md
# => lecture_notes_review.md (also printed to stdout)
```

### Preview a file with line numbers

```bash
nt preview notes.md         # or: nt p notes.md
nt p notes.md:10-30         # lines 10 through 30
```

### Merge two notes

Use `preview` to find the line ranges you want, then merge them:

```bash
# Merge specific ranges
nt m alice_notes.md:10-85 bob_notes.md:30-120

# Merge full files
nt m alice_notes.md bob_notes.md

# Custom output path
nt m alice_notes.md:10-85 bob_notes.md:30-120 -o combined.md

# Add instructions to guide the merge
nt m alice_notes.md:10-85 bob_notes.md:30-120 -i "Focus on the chemistry section"
# => alice_notes_bob_notes_merged.md (also printed to stdout)
```

The merge preserves all details from both snippets without summarizing. Contradictions between sources are marked with `<!-- CONFLICT -->` comments.

### Clean a transcript

```bash
nt clean lecture_transcript.txt    # or: nt c lecture_transcript.txt
# => lecture_transcript_cleaned.md
```

Splits the transcript into thematic sections and cleans each one individually.

### Global flags

```bash
nt --no-overwrite ocr notes.pdf              # Skip if output file exists
nt --ollama-host http://remote:11434 ocr notes.pdf  # Use a remote ollama instance
```

## Environment variables

| Variable | Description |
|---|---|
| `OLLAMA_HOST` | ollama base URL (default: `http://localhost:11434`) |
