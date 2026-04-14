# Contributing to brain

Thank you for considering contributing to brain! This document explains how
to set up your development environment, the code style we follow, and how
to submit changes.

## Development Setup

### Prerequisites

- **Go 1.24+**
- **[qmd](https://github.com/tobilu/qmd)** — the retrieval engine brain
  shells out to. `npm install -g @tobilu/qmd` (will be replaced by an
  in-process [recall](https://github.com/ugurcan-aytar/recall) dependency
  in a future version).
- **An LLM backend** — one of: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (with
  any OpenAI-compatible endpoint, including Ollama / OpenRouter / LM
  Studio / LiteLLM), or the [Claude Code CLI](https://claude.ai/download)
  on your `PATH`. `brain doctor` tells you which backend it sees.

### Build & Test

```sh
git clone https://github.com/ugurcan-aytar/brain.git
cd brain
go build ./...
go test ./...
```

### Run from source

```sh
go run ./cmd/brain --help
go run ./cmd/brain ask "what did I write about activation energy?"
```

## Code Style

- Always run `gofmt` before committing.
- **One file per CLI subcommand** in `internal/commands/`. Each file owns
  the handler *and* the `NewXxxCmd() *cobra.Command` constructor.
- **One responsibility per package.** `retriever` does retrieval, `prompt`
  classifies queries and builds system prompts, `llm` talks to backends —
  they don't call each other directly.
- Keep `cmd/brain/main.go` thin: signal handling, terminal restore, root
  command wiring. No business logic.
- No clever abstractions — straightforward code is preferred over elegant
  code. Three similar lines beat a premature interface.
- Keep function signatures simple and explicit. Handlers take plain
  `Options` structs, not `*cobra.Command`, so tests can call them directly.

## Commit Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): subject under ~70 chars, lowercase, imperative
```

- `feat: add Turkish thinking mode triggers` — new user-visible feature
- `fix: handle empty collection in search` — bug fix
- `docs: update README with new flags` — documentation only
- `refactor: extract scoring logic to separate function` — no behavior change
- `test: add BM25 ranking accuracy tests` — tests only
- `chore: update goreleaser config` — build / tooling / non-user-facing

**Body explains WHY, not WHAT.** The diff already shows what changed. Use
the body for the reason: the bug it fixes, the constraint it satisfies,
the decision it records.

For `fix:` commits, call out **symptom and root cause** in the body so
future readers grepping for a symptom can find the fix. Example:

```
fix(chat): restore visible input prompt on first render

Zero-value iota trap: the prompt style variable defaulted to the
"hidden" variant because iota started at 0 instead of 1. Shift the
iota block so the default (zero) value is the visible style.

Symptom: first keystroke in `brain chat` appears to vanish — the
prompt was rendered invisible, not the input.
```

For multi-line bodies, use a quoted HEREDOC so shells don't collapse blank
lines:

```sh
git commit -m "$(cat <<'EOF'
feat(chat): add /challenge re-scoring command

Re-runs the last Q&A against a different set of collections so users
can stress-test a conclusion against sources that weren't in the
original retrieval set. Output contrasts the two answers side by side.
EOF
)"
```

## Pull Request Process

1. Fork the repository.
2. Create a feature branch: `git checkout -b feat/my-feature`.
3. Make your changes with tests.
4. Ensure `go build ./...` and `go test ./...` pass.
5. Commit with a conventional-commit message.
6. Push and open a Pull Request.
7. Fill in the PR template.
8. Wait for CI to pass and review.

## Issue Guidelines

- **Bug reports** — include steps to reproduce, expected vs actual
  behavior, OS/arch, brain version (`brain --version` or commit hash),
  and which LLM backend you're using.
- **Feature requests** — describe the use case first, then the proposed
  solution. Alternatives considered are helpful.

See [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE/) for the full
templates that populate when you open an issue on GitHub.

## Architecture Overview

```
brain/
├── cmd/brain/           # Cobra entry point + subcommand wiring
├── internal/
│   ├── config/          # defaults, qmd env scrubber, output rewriter
│   ├── retriever/       # qmd subprocess wrapper, adaptive filtering,
│   │                      full-doc enrichment, deep filter, grounding gate
│   ├── prompt/          # query classifier + adaptive system prompt builder
│   ├── llm/             # Anthropic REST/SSE (prompt caching),
│   │                      OpenAI-compat, claude CLI fallback
│   ├── markdown/        # streaming terminal markdown renderer
│   ├── history/         # timestamped Q&A archive
│   ├── picker/          # interactive collection multi-select (huh)
│   ├── ui/              # logo, colors, source bars (lipgloss)
│   ├── version/         # version string injected by ldflags at build
│   └── commands/        # one file per CLI subcommand
```

Each package has its own `_test.go` files. New behavior should come with a
test.

## Test Coverage

**Current baseline: 17.2% overall (as of v0.2.14).**

Per-package breakdown:

| Package | Coverage |
|---|---|
| `internal/config` | 100.0% |
| `internal/prompt` | 98.3% |
| `internal/history` | 76.4% |
| `internal/markdown` | 68.3% |
| `internal/retriever` | 29.8% |
| `internal/llm` | 23.0% |
| `internal/commands` | 1.7% |
| `internal/ui` | 0.0% |
| `internal/picker` | 0.0% |

The "core reasoning" packages (`prompt`, `retriever`, `config`,
`markdown`, `history`) are where retrieval + grounding correctness lives
and where coverage matters most — keep those high. The low-coverage
tails (`ui`, `picker`, `commands`) are mostly bubbletea / huh TUI code
and Cobra wiring which is hard to unit-test without a PTY; incremental
coverage there is nice-to-have, not a gate.

**Target:** maintain or improve total coverage with every PR.

Run locally:

```sh
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out      # per-function summary
go tool cover -html=coverage.out      # interactive HTML report
```
