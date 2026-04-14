# Security Policy

brain handles API keys for LLM providers and reads (but never writes)
files in the folders you register. Please report security issues
responsibly.

## Reporting a Vulnerability

1. **Do NOT open a public GitHub issue.**
2. Email: **ugurcan.aytar@gmail.com** with subject line
   `brain security:` and a short description.
3. Include:
   - a clear description of the vulnerability,
   - steps to reproduce (ideally a minimal repro against a fresh
     `~/.brain/` / `~/.qmd/`),
   - the version of brain affected (`brain --version`),
   - potential impact (what an attacker could achieve).

## Response Timeline

- **Acknowledgment:** within 48 hours.
- **Assessment:** within 1 week.
- **Fix:** depends on severity. Targeting within 2 weeks for anything
  rated high / critical; lower severity fixes roll into the next patch
  release.

We'll coordinate a disclosure timeline with you before publishing the
fix. Reporters who want credit will be named in the release notes.

## Scope

Security issues we care about:

- **API key leakage** — keys appearing in logs, history files, command
  output, error messages, or crash dumps. brain intentionally scrubs
  `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` from qmd subprocess output; a
  bypass of that scrubber is in scope.
- **Path traversal** — any flag / argument / config value that lets
  brain read a file outside the collections registered by `brain add`.
- **Command injection** — collection names, file paths, query strings,
  or history-file contents being interpreted as shell commands (brain
  shells out to `qmd` and the `claude` CLI, so injection surfaces exist
  and matter).
- **Sensitive data in history files** — brain archives every Q&A to
  `~/.brain/history/` as plain markdown. Anything that widens this
  surface (e.g. writing API keys, raw tokens, or file contents never
  surfaced to the user) is in scope.
- **Supply-chain issues** — compromised dependency updates, install.sh
  integrity (the script verifies SHA-256 against `checksums.txt`, so any
  bypass of that check is in scope), Homebrew cask tampering.

**Out of scope:**

- LLM jailbreaks or prompt-injection-via-note-content. brain grounds
  answers in the retrieved chunks and forwards them verbatim to the
  model; that's the intended design. A malicious note inducing the
  model to emit specific text is a prompt-layer concern, not a brain
  vulnerability.
- Denial-of-service by feeding gigabyte-scale collections. brain is
  single-user / local-first and not hardened against malicious input
  volumes.

## Best Practices for Users

- **Never commit your `.env` or API keys to version control.** Use a
  secrets manager or shell rc file scoped to your user account.
- **Treat `~/.brain/history/` as sensitive.** Every question and every
  answer is archived there in plain markdown. If you query notes about
  private topics, your history has that text too.
- **Use `brain history rm <id>`** to delete sensitive entries. `brain
  history path` prints the directory if you want to audit or clear it.
- **Scope collections intentionally.** `brain add ~/Documents/private`
  grants brain read access to every file matching the glob under that
  path; keep the register list tight.
- **Keep brain updated.** Security fixes land in patch releases
  (`brew upgrade brain` or `brain upgrade` depending on install path).
