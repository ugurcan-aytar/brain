<div align="center">

<img src="assets/logo.png" alt="brain" width="640">

### Conversational knowledge base over your local notes

[![Go Version](https://img.shields.io/badge/go-1.24%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-lightgrey)](#requirements)
[![Interactive TUI](https://img.shields.io/badge/interactive-TUI-8a2be2?logo=gnubash&logoColor=white)](#chat-mode)
[![Optimized for Claude](https://img.shields.io/badge/optimized%20for-Claude-d97757?logo=anthropic&logoColor=white)](https://www.anthropic.com)
[![Built with qmd](https://img.shields.io/badge/retrieval-qmd-8A2BE2)](https://github.com/tobilu/qmd)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](#contributing)

</div>

---

`brain` turns a folder of markdown and text files into a queryable knowledge base. Ask it a question and it retrieves the most relevant chunks from your notes, then streams a grounded answer with citations back to your terminal. When your notes don't cover the question, it tells you — no hallucinated filler.

It's a **TUI-first app**, not a thin CLI wrapper around an API call. You get an interactive multi-select collection picker, a readline REPL with tab-completion and unique-prefix slash commands, a streaming markdown renderer that colors headings/code/lists live as tokens arrive, mid-response Ctrl+C cancellation, and model/mode pickers you can invoke mid-session. Built on Cobra, [charmbracelet/huh](https://github.com/charmbracelet/huh) (pickers), [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) (styling), and [chzyer/readline](https://github.com/chzyer/readline) (REPL). See [Chat mode](#chat-mode) for the full slash command surface.

This is the Go port of the original [Second Brain CLI](https://github.com/ugurcan-aytar/brain) (written in TypeScript/Bun). Same features, same design, single static binary, no runtime to install.

## Table of contents

- [Core principle](#core-principle)
- [Features](#features)
- [Requirements](#requirements)
- [Install](#install)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Chat mode](#chat-mode)
- [Thinking modes](#thinking-modes)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Core principle

> **No context → no LLM call → no hallucination.**

Every answer `brain` gives is grounded in chunks retrieved from your own notes. If the retrieval step returns nothing relevant, the LLM call is skipped entirely and you get a clear "nothing found" message instead of a confident-sounding fabrication.

## Features

- **`brain ask "<question>"`** — one-shot Q&A, cited sources, streaming answer
- **`brain chat`** — interactive multi-turn REPL with slash commands, tab completion, and mid-response cancellation
- **`brain search "<query>"`** — raw retrieval, no LLM, for verifying your index
- **`/challenge`** — re-score an answer against a different set of sources to check it
- **Adaptive prompt system** — questions are classified into `recall`, `analysis`, `decision`, or `synthesis` modes, each with a different response structure
- **Collection picker** — multi-select UI to scope a question to specific note folders
- **Model switching** — swap between `opus`, `sonnet`, and `haiku` mid-session
- **Q&A history** — every exchange is saved as a timestamped markdown file you can grep later
- **Ctrl+C everywhere** — cancel retrieval or streaming at any time without leaving your terminal in a broken state
- **Pluggable backend** — native Anthropic API, any OpenAI-compatible endpoint (OpenAI, Ollama, OpenRouter, LM Studio, LiteLLM, Groq, Together…), or the local `claude` CLI as a fallback

## Requirements

- **macOS, Linux, or Windows** — the `install.sh` script supports macOS and Linux; Windows users can grab the `.zip` directly from [Releases](https://github.com/ugurcan-aytar/brain/releases).
- **[qmd](https://github.com/tobilu/qmd)** — the local embeddings + retrieval engine that powers the search layer. The installer picks this up automatically if `npm` is available; otherwise run `npm install -g @tobilu/qmd` yourself.
- **At least one LLM backend.** brain picks the first one it finds, in this order:
  1. `ANTHROPIC_API_KEY` — native Claude API, the fastest and cheapest path (recommended).
  2. `OPENAI_API_KEY` — any OpenAI-compatible endpoint. Works out of the box with OpenAI, and via `OPENAI_BASE_URL` also with Ollama, OpenRouter, LM Studio, LiteLLM, Groq, Together, Fireworks, etc. See [Configuration](#configuration) for examples.
  3. The [Claude Code CLI](https://claude.ai/download) on your PATH — useful if you have a Claude subscription but no API key. Override the binary name with `BRAIN_CLAUDE_BIN` to point at a fork (e.g. `opencode`).
- **Go 1.24+** — only needed if you're building from source.

## Install

### One-liner (macOS & Linux)

```sh
curl -sSfL https://raw.githubusercontent.com/ugurcan-aytar/brain/main/install.sh | sh
```

The script downloads the right prebuilt binary for your OS/arch, verifies its SHA-256 against `checksums.txt`, drops it into `/usr/local/bin` (or `~/.local/bin` as a fallback), offers to `npm install -g @tobilu/qmd` if qmd is missing, and runs `brain doctor` at the end to confirm everything works.

Environment overrides: `BRAIN_VERSION=v1.2.3` to pin a release, `BRAIN_PREFIX=$HOME/.local` to change the install prefix, `BRAIN_NO_QMD=1` to skip the qmd step.

### From source

```sh
git clone https://github.com/ugurcan-aytar/brain.git
cd brain
go build -o brain ./cmd/brain
sudo mv brain /usr/local/bin/
```

### With `go install`

```sh
go install github.com/ugurcan-aytar/brain/cmd/brain@latest
```

After any install path, run `brain doctor` to check that qmd + a Claude backend are both wired up.

## Quick start

```sh
# 1. Register a folder of notes (auto-runs `brain index` afterward)
brain add ~/Documents/my-notes

# 2. Ask a question
brain ask "What did I write about activation energy?"

# 3. Or start an interactive conversation
brain chat
```

## Commands

### Query

| Command | Description |
|---|---|
| `brain ask "<question>"` | One-shot Q&A with cited sources |
| `brain chat` | Interactive multi-turn conversation |
| `brain search "<query>"` | Raw retrieval results (no LLM) |

**Flags on `ask`:**

- `-c, --collection <name>` — scope to a single collection (skips the picker)
- `-m, --model <model>` — `opus` (default), `sonnet`, `haiku`, or a full Anthropic model ID
- `-M, --mode <mode>` — override the auto-detected thinking mode: `auto`, `recall`, `analysis`, `decision`, `synthesis`

### Collections

| Command | Description |
|---|---|
| `brain add <path>` | Register a folder as a collection (runs `brain index` after) |
| `brain remove <name>` | Remove a collection and clean up its embeddings |
| `brain collections` | List registered collections |
| `brain files [-c name]` | List indexed files, optionally filtered by collection |

**Flags on `add`:**

- `--name <name>` — override the default collection name (folder basename)
- `--mask <glob>` — override the default file glob (`**/*.{txt,md}`)

### Maintenance

| Command | Description |
|---|---|
| `brain index` | Re-scan files and regenerate embeddings |
| `brain status` | Show index health and brain config |
| `brain doctor` | Verify qmd + LLM backend are installed and configured |

## Chat mode

`brain chat` is a full REPL with slash commands, rolling conversation history, and Tab-to-complete.

| Slash command | Description |
|---|---|
| `/help` | Show command list and current model/mode/collections |
| `/mode [name]` | View or change thinking mode (`auto`, `recall`, `analysis`, `decision`, `synthesis`) |
| `/model [name]` | View or switch Claude model; bare `/model` opens a picker |
| `/collections` | Re-run the collection picker |
| `/sources` | Show the sources from the last answer |
| `/challenge` | Re-score the last Q&A against a different set of collections |
| `/clear` | Reset conversation history |
| `/quit` | Exit chat (also: `Ctrl+C` twice) |

Slash commands support unique-prefix matching: typing `/col` resolves to `/collections`. Tab auto-completes partial commands.

Press `Ctrl+C` once during streaming to cancel the in-flight request. Press it twice within two seconds on an empty prompt to exit.

## Thinking modes

Every question gets a system prompt with one of four response structures. Auto-classification picks one based on regex heuristics (English + Turkish), or you can force one with `-M` / `/mode`.

| Mode | Trigger examples | Response structure |
|---|---|---|
| **recall** | "what did my notes say about…", "list…", "what is…" | **Direct answer** → **Related context** |
| **analysis** | "why…", "compare…", "how does X relate to Y", "explain…" | **Key findings** → **Connections** → **Gaps** → **Synthesis** |
| **decision** | "should I…", "pros and cons", "recommend…", "worth it" | **Relevant frameworks** → **Arguments** → **Blind spots** → **Recommendation** |
| **synthesis** | "plan…", "how can I build/scale/launch…", "roadmap" | **Building blocks** → **Integration** → **Action plan** → **Assumptions & gaps** |

Analysis is the default when nothing matches — it's the most generally useful.

## Configuration

Defaults live in [`internal/config/config.go`](internal/config/config.go). The interesting knobs:

| Setting | Default | Purpose |
|---|---|---|
| `Model` | `claude-opus-4-6` | Default Claude model |
| `MaxTokens` | `16384` | Response length cap |
| `TopK` | `7` | Chunks to retrieve per question |
| `MinScore` | `0.2` | Minimum relevance to include a chunk |
| `MinChunksToCallLLM` | `1` | Grounding gate threshold |
| `MaxConversationTurns` | `10` | Chat history cap (user + assistant per turn) |
| `DefaultMask` | `**/*.{txt,md}` | Files to index when adding a collection |

**Environment variables:**

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Use the native Anthropic API (highest priority backend) |
| `OPENAI_API_KEY` | Use any OpenAI-compatible `/v1/chat/completions` endpoint |
| `OPENAI_BASE_URL` | Override the OpenAI endpoint — point this at Ollama, OpenRouter, LM Studio, LiteLLM, etc. Defaults to `https://api.openai.com/v1` |
| `OPENAI_MODEL` | Model name to send to the OpenAI-compatible endpoint. Defaults to `gpt-4o`. Also honors `-m` / `/model` when the value doesn't look like a Claude alias, so `brain ask -m llama3.1 "…"` works on Ollama |
| `BRAIN_CLAUDE_BIN` | Name of the Claude CLI binary brain shells out to. Defaults to `claude`. Set to `opencode` (or another fork that speaks the same `stream-json` protocol) to reuse the CLI fallback without rebuilding |
| `BRAIN_HISTORY_DIR` | Override where Q&A history is written (defaults to `~/.brain/history`) |

### Using a different backend

**Ollama (local, free, offline-capable):**

```sh
export OPENAI_API_KEY=ollama          # any non-empty string works
export OPENAI_BASE_URL=http://localhost:11434/v1
export OPENAI_MODEL=llama3.1
brain ask "what did I write about activation energy?"
```

**OpenRouter (one key, every model):**

```sh
export OPENAI_API_KEY=sk-or-…
export OPENAI_BASE_URL=https://openrouter.ai/api/v1
brain ask -m meta-llama/llama-3.1-70b-instruct "…"
```

**OpenAI proper:**

```sh
export OPENAI_API_KEY=sk-…
brain ask "…"                         # uses gpt-4o by default
```

**`opencode` instead of `claude`:**

```sh
export BRAIN_CLAUDE_BIN=opencode
brain ask "…"
```

> **Note:** brain's adaptive prompts and thinking-mode directives are tuned for Claude. Non-Claude models will work — the retrieval gate ("no chunks → no LLM call") is model-agnostic — but response quality, especially for `synthesis` and `decision` modes, varies. Run `brain doctor` to see which backend is active.

## Architecture

```
cmd/brain/            # Cobra entry point + subcommand wiring
internal/
├── config/           # defaults, qmd env scrubber, output rewriter
├── retriever/        # qmd subprocess wrapper, JSON parsing, dedup, grounding gate
├── prompt/           # query classifier + adaptive system prompt builder
├── llm/              # Anthropic REST/SSE backend + claude CLI fallback
├── markdown/         # streaming terminal markdown renderer
├── history/          # timestamped Q&A archive
├── picker/           # interactive collection multi-select (charmbracelet/huh)
├── ui/               # logo, colors, source bars (charmbracelet/lipgloss)
└── commands/         # one file per CLI subcommand
```

### Retrieval → grounding → synthesis

```
question ──▶ qmd query (subprocess)
         ──▶ JSON parse + dedupe across collections
         ──▶ grounding gate (skip LLM if no chunks)
         ──▶ classify query → pick mode directive
         ──▶ build adaptive system prompt
         ──▶ stream response into markdown renderer
         ──▶ print sources + save history
```

### Why `qmd` as a subprocess?

`qmd` already has a battle-tested embeddings pipeline and a stable on-disk format in `~/.qmd`. Rewriting it in Go would break compatibility with anyone already using it, and give no measurable win. Shelling out is fast enough (single-digit ms overhead) and keeps the dependency surface small.

### Why direct HTTP instead of the Anthropic SDK?

The official Go SDK is still in beta and its public API shape changes across minor releases. The REST + SSE surface is stable and documented, so we talk to `api.anthropic.com` directly with `net/http`. ~150 lines, no dependencies, no version churn.

## Development

```sh
# Build
go build ./...

# Run the full test suite
go test ./...

# Smoke test the binary
./brain --help
```

Each package that has non-trivial logic ships with its own `_test.go` file — see `internal/config`, `internal/retriever`, `internal/prompt`, `internal/llm`, `internal/markdown`, and `internal/history`.

## Contributing

PRs welcome. The code is deliberately straightforward — one file per command, one responsibility per package, no clever abstractions. Please keep it that way.

Before opening a PR:

1. `go build ./...` must succeed.
2. `go test ./...` must pass.
3. New behavior should come with a test.

## License

MIT. See [LICENSE](LICENSE).
