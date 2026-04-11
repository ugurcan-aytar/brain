package commands

// Minimal-viable update check. On every invocation brain fires a
// background goroutine that queries GitHub's releases/latest endpoint
// and caches the answer to ~/.brain/.update-check. The next invocation
// reads the cache and, if a newer tag exists, prints a dimmed banner
// at the end of ask/chat/search output. No binary download — the
// `brain upgrade` command tells the user which package manager command
// to run. This keeps the feature additive and zero-risk: skipping the
// banner never breaks anything.
//
// Opt-out: set BRAIN_NO_UPDATE_CHECK=1. We also skip the check when
// stdout is not a terminal (piped output, CI, redirected).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/ugurcan-aytar/brain/internal/ui"
	"github.com/ugurcan-aytar/brain/internal/version"
)

const (
	updateCheckFile     = ".update-check"
	updateCheckInterval = 24 * time.Hour
	updateCheckTimeout  = 5 * time.Second
	updateCheckURL      = "https://api.github.com/repos/ugurcan-aytar/brain/releases/latest"
	updateCheckUA       = "brain-update-check"
)

type updateCache struct {
	CheckedAt time.Time `json:"checked_at"`
	LatestTag string    `json:"latest_tag"`
}

// CheckForUpdate fires a background goroutine to refresh the cached
// latest-version check if it's older than updateCheckInterval. Returns
// immediately. Guarded by the TTY check and BRAIN_NO_UPDATE_CHECK so
// scripted/CI usage doesn't make a surprise network call.
func CheckForUpdate() {
	if updateCheckDisabled() {
		return
	}
	path, err := updateCachePath()
	if err != nil {
		return
	}
	if cache, err := readUpdateCache(path); err == nil && time.Since(cache.CheckedAt) < updateCheckInterval {
		return
	}
	go refreshUpdateCache(path)
}

// PrintUpdateBanner prints a one-line dimmed notice below whatever
// output the caller just produced IF the cached latest release is
// strictly newer than version.Current. Callers are the user-facing
// commands (ask, chat startup, search) — everything else intentionally
// stays silent to keep diagnostic output unpolluted.
func PrintUpdateBanner() {
	if updateCheckDisabled() {
		return
	}
	path, err := updateCachePath()
	if err != nil {
		return
	}
	cache, err := readUpdateCache(path)
	if err != nil {
		return
	}
	if !isNewerTag(cache.LatestTag, version.Current) {
		return
	}
	fmt.Println()
	fmt.Println(ui.Dim.Render(fmt.Sprintf("  ↑ Update available: v%s → %s", version.Current, cache.LatestTag)))
	fmt.Println(ui.Dim.Render("    Run `brain upgrade` to see the install command for your platform."))
}

func updateCheckDisabled() bool {
	if strings.TrimSpace(os.Getenv("BRAIN_NO_UPDATE_CHECK")) != "" {
		return true
	}
	// Non-TTY (piped, redirected, CI) never sees the banner and
	// therefore never needs the background check.
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

func updateCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".brain")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(dir, updateCheckFile), nil
}

func readUpdateCache(path string) (updateCache, error) {
	var c updateCache
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

// refreshUpdateCache performs the actual GitHub API call and writes
// the result. Ignores every error path — update checks are a
// best-effort nicety, not a correctness-critical operation.
func refreshUpdateCache(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	tag, err := fetchLatestTag(ctx)
	if err != nil || tag == "" {
		return
	}
	data, err := json.Marshal(updateCache{
		CheckedAt: time.Now(),
		LatestTag: tag,
	})
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

// fetchLatestTag hits the GitHub releases/latest endpoint and returns
// the tag name (e.g. "v0.1.4"). Errors bubble up so the synchronous
// `brain upgrade` path can surface network failures.
func fetchLatestTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, updateCheckURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", updateCheckUA+"/"+version.Current)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// isNewerTag reports whether `tag` (e.g. "v0.1.4") names a semver
// version strictly greater than `current` (e.g. "0.1.3"). Unparseable
// versions compare false so garbage cache data never produces a nag.
func isNewerTag(tag, current string) bool {
	latest, ok := parseSemver(strings.TrimPrefix(tag, "v"))
	if !ok {
		return false
	}
	curr, ok := parseSemver(current)
	if !ok {
		return false
	}
	for i := 0; i < 3; i++ {
		if latest[i] != curr[i] {
			return latest[i] > curr[i]
		}
	}
	return false
}

// parseSemver handles MAJOR.MINOR.PATCH plus any pre-release or build
// metadata suffix (which we discard — brain only ships stable releases
// so a pre-release is "lower than stable of the same MMP").
func parseSemver(s string) ([3]int, bool) {
	var out [3]int
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
