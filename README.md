# notetools

Local AI CLI for OCR, transcription, and document review — powered by [llama.cpp](https://github.com/ggml-org/llama.cpp).

- **`notetools ocr`** — Convert PDFs to Markdown using GLM-OCR
- **`notetools transcribe`** — Transcribe audio to Markdown using Voxtral
- **`notetools review`** — Review Markdown for consistency using Claude Code

## Requirements

- Go 1.21+ (to build notetools)
- [llama.cpp](https://github.com/ggml-org/llama.cpp) built with multimodal support (`llama-mtmd-cli`)
- A supported GPU: NVIDIA (CUDA), AMD (ROCm), or Apple Silicon (Metal)
- `claude` CLI (only for the `review` subcommand)

### GPU VRAM

| Model | Size (Q8_0) | Purpose |
|---|---|---|
| GLM-OCR | ~950 MB + ~460 MB mmproj | PDF to Markdown |
| Voxtral Mini 4B Realtime | ~4.7 GB | Audio transcription |

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

### 2. Build notetools

```bash
git clone https://github.com/fingerprint/notetools
cd notetools
make build
sudo cp notetools-bin /usr/local/bin/notetools
```

### 3. Download models

```bash
notetools models pull
```

This downloads the GGUF models from HuggingFace (~6 GB total):
- `GLM-OCR-Q8_0.gguf` + `mmproj-GLM-OCR-Q8_0.gguf` (~1.4 GB)
- `Q8_0.gguf` (Voxtral Mini 4B Realtime, ~4.7 GB)

Models are stored in `~/.local/share/notetools/models/` by default.

Check download status:

```bash
notetools models list
```

### 4. Claude Code (optional, for `review` subcommand)

```bash
npm install -g @anthropic-ai/claude-code
claude auth
```

## Usage

### OCR a PDF

```bash
notetools ocr lecture_notes.pdf
# => lecture_notes.md
```

### Transcribe audio

```bash
notetools transcribe lecture_audio.mp3
# => lecture_audio_transcript.md
```

Supported formats: `.wav`, `.mp3`, `.m4a`

### Review a Markdown file

```bash
notetools review lecture_notes.md
# => lecture_notes_review.md (also printed to stdout)
```

### Global flags

```bash
notetools --verbose ocr notes.pdf        # Show llama.cpp stderr output
notetools --no-overwrite ocr notes.pdf   # Skip if output file exists
notetools --gpu-layers 20 ocr notes.pdf  # Control GPU offloading (-1 = all, default)
```

## Environment variables

| Variable | Description |
|---|---|
| `NOTETOOLS_LLAMA_BIN` | Path to `llama-mtmd-cli` binary |
| `NOTETOOLS_DATA_DIR` | Override model storage directory |
| `XDG_DATA_HOME` | Respected for model storage (default: `~/.local/share`) |
