// Package update reports when a newer Pyahu CLI release is available.
//
// The check never blocks a command and never fails one: it runs in the
// background, answers from a cache most of the time, and stays silent on any
// error. A developer offline, behind a proxy, or rate limited by GitHub sees
// exactly what they saw before.
package update

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
)

const (
	// DefaultReleaseURL is the public releases endpoint of the CLI repository.
	DefaultReleaseURL = "https://api.github.com/repos/pyahu/cli/releases/latest"

	// OptOutEnv silences the check entirely when set to any non-empty value.
	OptOutEnv = "PYAHU_NO_UPDATE_CHECK"

	cacheTTL     = 24 * time.Hour
	fetchTimeout = 2 * time.Second
)

// Result carries the two versions, already normalized (no leading "v").
type Result struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

// Checker resolves the latest published release. The zero value is not usable;
// use New.
type Checker struct {
	URL       string
	CachePath string
	HTTP      *http.Client
	Now       func() time.Time
}

func New() *Checker {
	return &Checker{
		URL:       DefaultReleaseURL,
		CachePath: defaultCachePath(),
		HTTP:      &http.Client{Timeout: fetchTimeout},
		Now:       time.Now,
	}
}

// Start begins the check in the background and returns a function that reports
// an available upgrade, waiting up to timeout for the answer.
//
// The work starts before the command runs and is harvested after, so the check
// costs no wall-clock time on the command itself. When the answer is not ready
// in time it is dropped: the cache it writes makes the next invocation instant.
func (c *Checker) Start(ctx context.Context, current string) func(time.Duration) (Result, bool) {
	if !Enabled(current) {
		return func(time.Duration) (Result, bool) { return Result{}, false }
	}
	results := make(chan string, 1)
	go func() {
		defer close(results)
		latest, err := c.latest(ctx)
		if err != nil {
			return
		}
		results <- latest
	}()

	return func(timeout time.Duration) (Result, bool) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case latest, ok := <-results:
			if !ok || !Outdated(current, latest) {
				return Result{}, false
			}
			return Result{Current: normalize(current), Latest: normalize(latest)}, true
		case <-timer.C:
			return Result{}, false
		}
	}
}

// Enabled reports whether the check should run at all. A binary built outside a
// release has no version to compare, and the opt-out is absolute.
func Enabled(current string) bool {
	if os.Getenv(OptOutEnv) != "" {
		return false
	}
	return parse(current) != nil
}

// latest answers from the cache while it is fresh, and otherwise asks GitHub and
// refreshes the cache.
func (c *Checker) latest(ctx context.Context) (string, error) {
	if cached, ok := c.readCache(); ok {
		return cached, nil
	}
	latest, err := c.fetch(ctx)
	if err != nil {
		return "", err
	}
	c.writeCache(latest)
	return latest, nil
}

func (c *Checker) fetch(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("releases endpoint returned %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if parse(payload.TagName) == nil {
		return "", fmt.Errorf("unrecognized release tag %q", payload.TagName)
	}
	return payload.TagName, nil
}

type cacheEntry struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

func (c *Checker) readCache() (string, bool) {
	if c.CachePath == "" {
		return "", false
	}
	data, err := os.ReadFile(c.CachePath)
	if err != nil {
		return "", false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return "", false
	}
	if entry.Latest == "" || c.Now().Sub(entry.CheckedAt) > cacheTTL {
		return "", false
	}
	return entry.Latest, true
}

// writeCache is best effort: a read-only or missing config directory only means
// the next command asks GitHub again.
func (c *Checker) writeCache(latest string) {
	if c.CachePath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.CachePath), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(cacheEntry{CheckedAt: c.Now(), Latest: latest})
	if err != nil {
		return
	}
	_ = os.WriteFile(c.CachePath, data, 0o644)
}

func defaultCachePath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "pyahu", "version-check.json")
}

// Outdated reports whether current is behind latest. A version it cannot parse
// is never outdated: guessing would nag a user it cannot help.
func Outdated(current string, latest string) bool {
	left, right := parse(current), parse(latest)
	if left == nil || right == nil {
		return false
	}
	return compare(left, right) < 0
}

// parse accepts "v0.6.1", "0.6.1" and "0.6.1-rc.1", and rejects anything else —
// "dev", the default of a locally built binary, included.
func parse(version string) []int {
	version = normalize(version)
	if version == "" {
		return nil
	}
	if index := strings.IndexAny(version, "-+"); index >= 0 {
		version = version[:index]
	}
	fields := strings.Split(version, ".")
	if len(fields) != 3 {
		return nil
	}
	parts := make([]int, 0, 3)
	for _, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 {
			return nil
		}
		parts = append(parts, value)
	}
	return parts
}

func compare(left []int, right []int) int {
	for i := range left {
		switch {
		case left[i] < right[i]:
			return -1
		case left[i] > right[i]:
			return 1
		}
	}
	return 0
}

func normalize(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}
