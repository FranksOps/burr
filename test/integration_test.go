//go:build integration

package test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/fingerprint"
	"github.com/FranksOps/burr/internal/scraper"
	"github.com/FranksOps/burr/internal/storage"
	"github.com/FranksOps/burr/pkg/proxy"
	"github.com/FranksOps/burr/pkg/ratelimit"
	"github.com/FranksOps/burr/pkg/useragent"
	"log/slog"
)

// mockBackend is an in-memory storage.Backend for verifying results
type mockBackend struct {
	mu      sync.Mutex
	results []*storage.ScrapeResult
}

func (m *mockBackend) Save(ctx context.Context, res *storage.ScrapeResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.results = append(m.results, res)
	return nil
}
func (m *mockBackend) Query(ctx context.Context, filter storage.Filter) ([]*storage.ScrapeResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.results, nil
}
func (m *mockBackend) Close() error { return nil }

func TestIntegration_BasicCrawl(t *testing.T) {
	// 1. Setup mock target server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>
			<a href="/page1">Page 1</a>
			<a href="/page2">Page 2</a>
		</body></html>`)
	})
	mux.HandleFunc("/page1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>Page 1 content</body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		// Simulate a bot defense page from Cloudflare
		w.Header().Set("Server", "cloudflare")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `<html><body>cf-browser-verification</body></html>`)
	})

	targetServer := httptest.NewServer(mux)
	defer targetServer.Close()

	// 2. Setup Crawler dependencies
	backend := &mockBackend{}
	
	// Create a fetcher with no proxy, default UA
	cfg := scraper.FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo, // Use Go profile for stdlib HTTP tests internally
		Limiter:     ratelimit.NewLimiter(0, 0), // No rate limiting
	}
	fetcher, err := scraper.NewFetcher(cfg)
	if err != nil {
		t.Fatalf("failed to create fetcher: %v", err)
	}

	u, _ := url.Parse(targetServer.URL)
	domains := []string{u.Hostname()}

	crawlCfg := scraper.CrawlConfig{
		MaxDepth:    1, // fetch root and links found on root
		Concurrency: 2,
		Domains:     domains,
		Backend:     backend,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	crawler := scraper.NewCrawler(crawlCfg, fetcher, logger)

	// 3. Execute Crawl
	err = crawler.Run(context.Background(), []string{targetServer.URL})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	// 4. Verify Results
	if len(backend.results) != 3 {
		t.Fatalf("expected 3 results (root, page1, page2), got %d", len(backend.results))
	}

	var rootFound, page1Found, page2Found bool
	for _, res := range backend.results {
		if res.URL == targetServer.URL || res.URL == targetServer.URL+"/" {
			rootFound = true
			if res.StatusCode != 200 {
				t.Errorf("expected 200 for root, got %d", res.StatusCode)
			}
		} else if strings.HasSuffix(res.URL, "/page1") {
			page1Found = true
			if res.StatusCode != 200 {
				t.Errorf("expected 200 for page1, got %d", res.StatusCode)
			}
		} else if strings.HasSuffix(res.URL, "/page2") {
			page2Found = true
			if res.StatusCode != 403 {
				t.Errorf("expected 403 for page2, got %d", res.StatusCode)
			}
			if !res.DetectedBot || res.DetectionSrc != "Cloudflare" {
				t.Errorf("expected Cloudflare bot detection for page2")
			}
		}
	}

	if !rootFound || !page1Found || !page2Found {
		t.Errorf("missing expected pages in crawl results: root=%v, page1=%v, page2=%v", rootFound, page1Found, page2Found)
	}
}

func TestIntegration_ProxyRotation(t *testing.T) {
	var proxyHits int32
	// 1. Setup mock proxy server
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&proxyHits, 1)
		// Proxy should return a unique header to prove it was used
		w.Header().Set("X-Proxied", "true")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "proxied content")
	}))
	defer proxySrv.Close()

	// 2. Setup mock target server (we will bypass proxying locally in test fetcher but we actually want the proxy to intercept it)
	// Because of how http.Transport.Proxy works with localhost URLs in tests (often skipped),
	// we use a remote IP structure if necessary, or just rely on the proxy server returning directly.
	// For this test, we just configure the crawler to fetch ANY url, and verify proxy intercepts it.

	backend := &mockBackend{}
	pPool := proxy.NewPool(proxy.Config{})
	pPool.Add(proxySrv.URL) // Route through our mock proxy

	// Provide a specific UA
	uaPool := useragent.NewPool([]string{"IntegrationTest-UA"})

	cfg := scraper.FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo, 
		ProxyPool:   pPool,
		UAPool:      uaPool,
	}
	fetcher, err := scraper.NewFetcher(cfg)
	if err != nil {
		t.Fatalf("failed to create fetcher: %v", err)
	}

	// We crawl a "remote" URL so it forces proxy usage
	targetURL := "http://example.com/testproxy"
	crawlCfg := scraper.CrawlConfig{
		MaxDepth:    0,
		Concurrency: 1,
		Domains:     []string{"example.com"},
		Backend:     backend,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	crawler := scraper.NewCrawler(crawlCfg, fetcher, logger)

	err = crawler.Run(context.Background(), []string{targetURL})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if atomic.LoadInt32(&proxyHits) == 0 {
		t.Errorf("expected proxy server to be hit, got 0")
	}

	if len(backend.results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(backend.results))
	}

	res := backend.results[0]
	if res.StatusCode != 200 {
		t.Errorf("expected status 200, got %d: error %s", res.StatusCode, res.Error)
	}
	
	proxiedHeader := ""
	if vals, ok := res.Headers["X-Proxied"]; ok && len(vals) > 0 {
		proxiedHeader = vals[0]
	}
	if proxiedHeader != "true" {
		t.Errorf("expected X-Proxied header from proxy server")
	}
}

func TestIntegration_CookieJarPersistence(t *testing.T) {
	// Server sets a cookie on /login, and checks for it on /protected
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:  "session_id",
			Value: "123456",
			Path:  "/",
		})
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><a href="/protected">Protected</a></body></html>`)
	})

	mux.HandleFunc("/protected", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil || cookie.Value != "123456" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body>Protected content</body></html>`)
	})

	targetServer := httptest.NewServer(mux)
	defer targetServer.Close()

	backend := &mockBackend{}
	
	// Create a fetcher with Cookie Jar explicitly enabled
	cfg := scraper.FetchConfig{
		Timeout:      5 * time.Second,
		Fingerprint:  fingerprint.ProfileGo,
		UseCookieJar: true,
	}
	fetcher, err := scraper.NewFetcher(cfg)
	if err != nil {
		t.Fatalf("failed to create fetcher: %v", err)
	}

	u, _ := url.Parse(targetServer.URL)
	domains := []string{u.Hostname()}

	crawlCfg := scraper.CrawlConfig{
		MaxDepth:    1, // Fetch /login -> extract links -> Fetch /protected using same cookie jar
		Concurrency: 1, // Keep concurrency 1 so cookie logic is predictable
		Domains:     domains,
		Backend:     backend,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	crawler := scraper.NewCrawler(crawlCfg, fetcher, logger)

	err = crawler.Run(context.Background(), []string{targetServer.URL + "/login"})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if len(backend.results) != 2 {
		t.Fatalf("expected 2 results (login and protected), got %d", len(backend.results))
	}

	for _, res := range backend.results {
		if strings.HasSuffix(res.URL, "/protected") {
			if res.StatusCode != http.StatusOK {
				t.Errorf("expected 200 OK for /protected due to cookie jar, got %d", res.StatusCode)
			}
		}
	}
}
