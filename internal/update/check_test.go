package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fakeReleases(t *testing.T, tag string, hits *int) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			*hits++
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"tag_name": tag})
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func testChecker(t *testing.T, url string) *Checker {
	t.Helper()
	return &Checker{
		URL:       url,
		CachePath: filepath.Join(t.TempDir(), "version-check.json"),
		HTTP:      &http.Client{Timeout: 2 * time.Second},
		Now:       time.Now,
	}
}

func TestStartReportsANewerRelease(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	checker := testChecker(t, fakeReleases(t, "v0.6.1", nil))

	result, ok := checker.Start(context.Background(), "0.4.0")(2 * time.Second)
	if !ok {
		t.Fatal("expected an upgrade to be reported")
	}
	if result.Current != "0.4.0" || result.Latest != "0.6.1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestStartStaysSilentWhenCurrent(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	checker := testChecker(t, fakeReleases(t, "v0.6.1", nil))

	if _, ok := checker.Start(context.Background(), "0.6.1")(2 * time.Second); ok {
		t.Fatal("a current version must not be nagged")
	}
	// Ahead of the published release (a local build of an unreleased commit).
	if _, ok := checker.Start(context.Background(), "0.7.0")(2 * time.Second); ok {
		t.Fatal("a version ahead of the release must not be nagged")
	}
}

func TestStartSkipsDevelopmentBuilds(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	hits := 0
	checker := testChecker(t, fakeReleases(t, "v0.6.1", &hits))

	if _, ok := checker.Start(context.Background(), "dev")(2 * time.Second); ok {
		t.Fatal("a dev build has no version to compare")
	}
	if hits != 0 {
		t.Fatalf("a dev build must not reach the network: %d request(s)", hits)
	}
}

func TestStartHonorsTheOptOut(t *testing.T) {
	t.Setenv(OptOutEnv, "1")
	hits := 0
	checker := testChecker(t, fakeReleases(t, "v0.6.1", &hits))

	if _, ok := checker.Start(context.Background(), "0.1.0")(2 * time.Second); ok {
		t.Fatal("the opt-out must suppress the check")
	}
	if hits != 0 {
		t.Fatalf("the opt-out must not reach the network: %d request(s)", hits)
	}
}

func TestStartIsSilentWhenTheEndpointFails(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)
	checker := testChecker(t, server.URL)

	// Being rate limited, offline or behind a proxy must never surface to the user.
	if _, ok := checker.Start(context.Background(), "0.1.0")(2 * time.Second); ok {
		t.Fatal("a failed lookup must stay silent")
	}
}

func TestSecondCheckAnswersFromTheCache(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	hits := 0
	checker := testChecker(t, fakeReleases(t, "v0.6.1", &hits))

	if _, ok := checker.Start(context.Background(), "0.4.0")(2 * time.Second); !ok {
		t.Fatal("first check should report the upgrade")
	}
	if _, ok := checker.Start(context.Background(), "0.4.0")(2 * time.Second); !ok {
		t.Fatal("second check should report the upgrade")
	}
	if hits != 1 {
		t.Fatalf("expected one network request, got %d", hits)
	}
}

func TestStaleCacheIsRefreshed(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	hits := 0
	checker := testChecker(t, fakeReleases(t, "v0.6.1", &hits))
	stale, err := json.Marshal(cacheEntry{CheckedAt: time.Now().Add(-cacheTTL - time.Hour), Latest: "v0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(checker.CachePath, stale, 0o644); err != nil {
		t.Fatal(err)
	}

	result, ok := checker.Start(context.Background(), "0.4.0")(2 * time.Second)
	if !ok || result.Latest != "0.6.1" {
		t.Fatalf("stale cache was not refreshed: ok=%t result=%#v", ok, result)
	}
	if hits != 1 {
		t.Fatalf("expected one network request, got %d", hits)
	}
}

func TestOutdated(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"0.4.0", "v0.6.1", true},
		{"v0.6.0", "v0.6.1", true},
		{"0.6.1", "0.6.1", false},
		{"0.7.0", "0.6.1", false},
		{"1.0.0", "0.9.9", false},
		{"0.9.9", "1.0.0", true},
		// Unparseable versions are never nagged: a guess helps nobody.
		{"dev", "v0.6.1", false},
		{"", "v0.6.1", false},
		{"0.6", "v0.6.1", false},
		{"0.4.0", "nightly", false},
		// A pre-release compares by its numeric core.
		{"0.6.0-rc.1", "v0.6.1", true},
	}
	for _, tc := range cases {
		if got := Outdated(tc.current, tc.latest); got != tc.want {
			t.Errorf("Outdated(%q, %q) = %t, want %t", tc.current, tc.latest, got, tc.want)
		}
	}
}

func TestSlowEndpointDoesNotHoldTheCommand(t *testing.T) {
	t.Setenv(OptOutEnv, "")
	released := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-released // hangs until the test lets it go
	}))
	t.Cleanup(func() {
		close(released)
		server.Close()
	})
	checker := testChecker(t, server.URL)

	// The harvest window is the ceiling on what a hung endpoint can cost a
	// command, regardless of the HTTP client timeout behind it.
	start := time.Now()
	_, ok := checker.Start(context.Background(), "0.1.0")(200 * time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("a hung endpoint must not report an upgrade")
	}
	if elapsed > time.Second {
		t.Fatalf("waited %s for a hung endpoint; the harvest window must bound it", elapsed)
	}
}
