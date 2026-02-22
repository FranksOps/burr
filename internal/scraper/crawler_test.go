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

func TestCrawler_Crawl(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/page2">Page 2</a><a href="/out-of-scope">Out</a></body></html>`))
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/page3">Page 3</a></body></html>`))
	})
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>No more links</body></html>`))
	})
	mux.HandleFunc("/out-of-scope", func(w http.ResponseWriter, r *http.Request) {
		// Should not be requested because of domain filter, we'll verify it's not in results
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Extract domain from httptest URL
	tsURL := ts.URL
	tsHost := strings.TrimPrefix(tsURL, "http://")
	// Hostname without port for Domains filter
	if strings.Contains(tsHost, ":") {
		tsHost = strings.Split(tsHost, ":")[0]
	}

	cfg := CrawlConfig{
		MaxDepth:    2,
		Concurrency: 2,
		Domains:     []string{tsHost},
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

	// Normalization strips trailing slash for root URL typically? Let's check exactly what we expect.
	// Actually URL parsing might not strip trailing slash, but our normalization does strip fragment.
	expected := []string{
		ts.URL + "/",
		ts.URL + "/page2",
		ts.URL + "/page3",
		ts.URL + "/out-of-scope", // It gets added to queue because it's same host! My bad in test setup.
	}

	if len(crawler.visited) != 4 {
		t.Errorf("expected 4 visited URLs, got %d", len(crawler.visited))
		for k := range crawler.visited {
			t.Logf("visited: %s", k)
		}
	}

	for _, u := range expected {
		if _, ok := crawler.visited[u]; !ok {
			t.Errorf("expected to visit %s", u)
		}
	}
}

func TestCrawler_ExternalDomainScope(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="http://external.com">External</a></body></html>`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	tsHost := strings.TrimPrefix(ts.URL, "http://")

	cfg := CrawlConfig{
		MaxDepth:    1,
		Concurrency: 1,
		Domains:     []string{tsHost},
	}

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	crawler := NewCrawler(cfg, fetcher, slog.Default())
	ctx := context.Background()
	_ = crawler.Run(ctx, []string{ts.URL + "/"})

	crawler.visitedMu.RLock()
	defer crawler.visitedMu.RUnlock()

	// We should NOT have visited external.com
	for v := range crawler.visited {
		if strings.Contains(v, "external.com") {
			t.Errorf("visited out-of-scope URL: %s", v)
		}
	}
}

func TestCrawler_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // Ensure it takes some time
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/page2">Next</a></body></html>`))
	}))
	defer ts.Close()

	cfg := CrawlConfig{
		MaxDepth:    5,
		Concurrency: 1,
	}

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	crawler := NewCrawler(cfg, fetcher, slog.Default())
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)
	go func() {
		errCh <- crawler.Run(ctx, []string{ts.URL + "/"})
	}()

	// Cancel shortly after start
	time.Sleep(10 * time.Millisecond)
	cancel()

	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got %v", err)
	}
}
