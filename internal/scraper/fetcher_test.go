package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/fingerprint"
	"github.com/FranksOps/burr/pkg/proxy"
	"github.com/FranksOps/burr/pkg/useragent"
)

func TestFetcher_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("expected User-Agent header, got none")
		}
		w.Header().Set("X-Test", "true")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
		UAPool:      useragent.NewPool([]string{"TestBrowser/1.0"}),
	})

	ctx := context.Background()
	res, err := fetcher.Fetch(ctx, ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Error != "" {
		t.Fatalf("expected no fetch error, got %s", res.Error)
	}

	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	if string(res.Body) != "ok" {
		t.Errorf("expected body 'ok', got %s", string(res.Body))
	}

	if len(res.Headers["X-Test"]) == 0 || res.Headers["X-Test"][0] != "true" {
		t.Errorf("expected X-Test header 'true', got %v", res.Headers["X-Test"])
	}

	if res.Duration == 0 {
		t.Errorf("expected non-zero duration")
	}

	if res.Method != http.MethodGet {
		t.Errorf("expected GET method, got %s", res.Method)
	}

	if res.ID == "" {
		t.Errorf("expected non-empty UUID")
	}
}

func TestFetcher_Timeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     10 * time.Millisecond,
		Fingerprint: fingerprint.ProfileGo,
	})

	ctx := context.Background()
	res, _ := fetcher.Fetch(ctx, ts.URL)

	if res.Error == "" || !strings.Contains(res.Error, "request failed") {
		t.Errorf("expected timeout error, got %v", res.Error)
	}
}

func TestFetcher_Proxy(t *testing.T) {
	// A server acting as a proxy (we'll just use it to see if we get routed there)
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer proxyServer.Close()

	// Ensure our test client forces the proxy
	pPool := proxy.NewPool(proxy.Config{MaxFailures: 1, Cooldown: 1 * time.Second})
	err := pPool.Add(proxyServer.URL)
	if err != nil {
		t.Fatalf("failed to add proxy: %v", err)
	}

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
		ProxyPool:   pPool,
	})

	// Start a dummy server to hit so it doesn't fail DNS before the proxy handles it.
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer targetServer.Close()

	ctx := context.Background()
	// Target URL doesn't matter, it should hit the proxy which returns 418 Teapot instead of proxying
	res, _ := fetcher.Fetch(ctx, targetServer.URL)

	// The problem is 127.0.0.1:0 fails DNS/TCP dial BEFORE proxy for HTTP unless configured right,
	// wait, proxy should connect to proxy IP. But if we want HTTP proxy to reply with teapot:
	// A teapot response from a proxy acting as a server is technically a proxy error or server response.
	// But let's actually make the proxy just a regular server handling the request.
	if res.StatusCode != http.StatusTeapot {
		t.Errorf("expected 418 Teapot from proxy, got %d, err: %v", res.StatusCode, res.Error)
	}
}
