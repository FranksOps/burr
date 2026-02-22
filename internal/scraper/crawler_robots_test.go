package scraper

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/fingerprint"
)

func TestCrawler_RobotsTxt(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /blocked\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/allowed">Allowed</a><a href="/blocked">Blocked</a></body></html>`))
	})
	mux.HandleFunc("/allowed", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/blocked", func(w http.ResponseWriter, r *http.Request) {
		t.Error("requested /blocked but should be forbidden by robots.txt")
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	tsHost := strings.TrimPrefix(ts.URL, "http://")

	cfg := CrawlConfig{
		MaxDepth:      1,
		Concurrency:   1,
		Domains:       []string{tsHost},
		RespectRobots: true,
		UserAgent:     "TestBot",
	}

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	crawler := NewCrawler(cfg, fetcher, slog.Default())
	ctx := context.Background()

	err := crawler.Run(ctx, []string{ts.URL + "/"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	crawler.visitedMu.RLock()
	defer crawler.visitedMu.RUnlock()

	// It should have visited /, /allowed, and /blocked
	// Wait, processJob checks robots.txt BEFORE fetching, but the URL is added to the queue
	// and marked as visited. Let's see if /blocked is in visited.
	// Yes, markVisited happens before processJob.

	// Let's add an explicit check to see what was actually fetched.
	// Wait, the test fails if /blocked is requested, so we're covered by the mux.
}
