# nt (notetools)

Local AI CLI for OCR, document review, and note merging — powered by [llama.cpp](https://github.com/ggml-org/llama.cpp) and [Claude Code](https://claude.com/claude-code).

- **`nt ocr`** (`o`) — Convert PDFs to Markdown using GLM-OCR
- **`nt review`** (`r`) — Review Markdown for consistency using Claude Code
- **`nt preview`** (`p`) — Preview a file with line numbers (for selecting merge ranges)
- **`nt merge`** (`m`) — Merge snippets from two notes into one using Claude Code

## Requirements

- Go 1.21+ (to build)
- [llama.cpp](https://github.com/ggml-org/llama.cpp) built with multimodal support (`llama-mtmd-cli`)
- A supported GPU: NVIDIA (CUDA), AMD (ROCm), or Apple Silicon (Metal)
- `claude` CLI (for `review` and `merge` subcommands)

### GPU VRAM

| Model | Size (Q8_0) | Purpose |
|---|---|---|
| GLM-OCR | ~950 MB + ~460 MB mmproj | PDF to Markdown |

## Installation

### 1. Install llama.cpp

Clone and build llama.cpp with GPU acceleration for your platform.

#### AMD GPU (ROCm)

```bash
# Install build dependencies (Fedora)
sudo dnf install cmake gcc-c++ hipcc rocblas-devel hipblas-devel

# On Ubuntu/Debian:
# sudo apt install cmake g++ hipcc librocblas-dev libhipblas-dev

git clone https://github.com/ggml-org/llama.cpp
cd llama.cpp
cmake -B build -DGGML_HIP=ON
cmake --build build --config Release -j$(nproc)
sudo cp build/bin/llama-mtmd-cli /usr/local/bin/
```

#### NVIDIA GPU (CUDA)

```bash
# Requires CUDA toolkit installed
git clone https://github.com/ggml-org/llama.cpp
cd llama.cpp
cmake -B build -DGGML_CUDA=ON
cmake --build build --config Release -j$(nproc)
sudo cp build/bin/llama-mtmd-cli /usr/local/bin/
```

#### Apple Silicon (Metal)

```bash
# Metal is enabled by default on macOS
git clone https://github.com/ggml-org/llama.cpp
cd llama.cpp
cmake -B build
cmake --build build --config Release -j$(sysctl -n hw.ncpu)
sudo cp build/bin/llama-mtmd-cli /usr/local/bin/
```

Verify the installation:

```bash
llama-mtmd-cli --help
```

If you don't want to copy to `/usr/local/bin`, you can point notetools to the binary directly:

```bash
export NOTETOOLS_LLAMA_BIN=/path/to/llama.cpp/build/bin/llama-mtmd-cli
```

### 2. Build nt

```bash
git clone https://github.com/fingerprint/notetools
cd notetools
make build
sudo make install   # installs to /usr/local/bin/nt
```

### 3. Download models

```bash
nt models pull
```

This downloads the GGUF models from HuggingFace:
- `GLM-OCR-Q8_0.gguf` + `mmproj-GLM-OCR-Q8_0.gguf` (~1.4 GB)

Models are stored in `~/.local/share/notetools/models/` by default.

Check download status:

```bash
nt models list
```

### 4. Claude Code (for `review` and `merge` subcommands)

```bash
npm install -g @anthropic-ai/claude-code
claude auth
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

### Global flags

```bash
nt --verbose ocr notes.pdf        # Show llama.cpp stderr output
nt --no-overwrite ocr notes.pdf   # Skip if output file exists
nt --gpu-layers 20 ocr notes.pdf  # Control GPU offloading (-1 = all, default)
```

## Environment variables

| Variable | Description |
|---|---|
| `NOTETOOLS_LLAMA_BIN` | Path to `llama-mtmd-cli` binary |
| `NOTETOOLS_DATA_DIR` | Override model storage directory |
| `XDG_DATA_HOME` | Respected for model storage (default: `~/.local/share`) |
