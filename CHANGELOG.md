# Changelog

All notable changes to brain will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.2.14] - 2026-04-13

### Changed
- Dropped the last references to "qmd" in user-facing messages — the
  retrieval engine is surfaced as "search" / "index" in CLI output.
- README updated to match the current enrichment + deep-retrieval pipeline.

### Fixed
- Full-document enrichment correctly handles collections whose top chunk
  points at an empty file.
- Dead-code removal in `internal/retriever` so the grounding gate path is
  the only path.

### Internal
- Auto-install qmd on first use when it's missing and `npm` is available
  (prompts before running).

## [0.2.13] - 2026-04-13

### Added
- Full-document enrichment — the top retrieved chunks are re-fetched as
  complete documents before being handed to the LLM, so answers grounded
  in transcripts don't lose context that lived outside the highest-scoring
  chunk.
- `brain add --context "<description>"` to describe what a collection is
  about; the hint is passed through to the retrieval layer and measurably
  improves quality on domain-specific content.

## [0.2.12] - 2026-04-13

### Fixed
- Retrieval reliability on long / noisy corpora: removed the duplicate
  query-expansion pass that was diluting the candidate set.
- Hardened system prompts so the model stops hedging when the retrieved
  chunks are unambiguous.

## [0.2.11] - 2026-04-13

### Added
- `--deep` flag on `brain ask` (and `/deep` toggle in chat): two-pass
  retrieval where an LLM call filters the 20 retrieved chunks down to the
  8–10 most relevant before synthesis.
- Cross-source-tension directive in the system prompt — the model must
  call out disagreements between sources before synthesizing.
- Elapsed time is now shown on `brain search` results.

## [0.2.10] - 2026-04-13

### Changed
- Query expansion now uses Sonnet instead of Haiku. Haiku was producing
  too-literal paraphrases that didn't open the retrieval net.

## [0.2.9] - 2026-04-13

### Added
- Bordered input box with placeholder text in `brain chat` so the prompt
  is visible before you start typing.

## [0.2.8] - 2026-04-13

### Added
- `brain doctor` and the no-backend error path now print setup
  instructions for each supported backend (Anthropic, OpenAI-compatible,
  `claude` CLI).
- Regression tests covering everything shipped in v0.2.0–v0.2.7.

## [0.2.7] - 2026-04-13

### Added
- OpenAI-compatible fallback for multi-query expansion. The expansion
  step now works when only `OPENAI_API_KEY` is set (Ollama / OpenRouter
  / LM Studio / LiteLLM via `OPENAI_BASE_URL`).

### Changed
- README documents v0.2.2–v0.2.6 features that had shipped but weren't
  yet covered in docs.

## [0.2.6] - 2026-04-13

### Changed
- Replaced the hard `MinScore` cutoff with an adaptive filter: the floor
  is 40% of the top chunk's score, so weak-but-best results survive on
  difficult queries instead of returning "nothing found".

## [0.2.5] - 2026-04-13

### Changed
- `TopK` raised from 7 → 20 by default. With adaptive scoring and the
  grounding gate the extra recall lands as higher answer quality, not
  noise.

---

_Older releases (v0.2.0–v0.2.4, v0.1.x) predate this changelog; see
the [GitHub releases page](https://github.com/ugurcan-aytar/brain/releases)
for their release notes._
