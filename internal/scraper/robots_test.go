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

func TestRobotsTxtAuditor_IsAllowed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`
User-agent: *
Disallow: /admin/
Allow: /admin/public/

User-agent: BadBot
Disallow: /
		`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	auditor := NewRobotsTxtAuditor(fetcher, slog.Default())
	ctx := context.Background()

	// Test generic bot rules
	allowed, err := auditor.IsAllowed(ctx, ts.URL+"/public-page", "GoodBot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected /public-page to be allowed")
	}

	allowed, _ = auditor.IsAllowed(ctx, ts.URL+"/admin/secret", "GoodBot")
	if allowed {
		t.Errorf("expected /admin/secret to be disallowed")
	}

	allowed, _ = auditor.IsAllowed(ctx, ts.URL+"/admin/public/index.html", "GoodBot")
	if !allowed {
		t.Errorf("expected /admin/public/index.html to be allowed")
	}

	// Test specific bot rules
	allowed, _ = auditor.IsAllowed(ctx, ts.URL+"/public-page", "BadBot")
	if allowed {
		t.Errorf("expected /public-page to be disallowed for BadBot")
	}
}

func TestRobotsTxtAuditor_MissingRobots(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	auditor := NewRobotsTxtAuditor(fetcher, slog.Default())
	ctx := context.Background()

	allowed, err := auditor.IsAllowed(ctx, ts.URL+"/anything", "Bot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Errorf("expected missing robots.txt to default to allowed")
	}
}

func TestRobotsTxtAuditor_Sitemaps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`
User-agent: *
Sitemap: http://example.com/sitemap.xml
Sitemap: http://example.com/sitemap2.xml
		`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	auditor := NewRobotsTxtAuditor(fetcher, slog.Default())
	ctx := context.Background()

	tsHost := strings.TrimPrefix(ts.URL, "http://")
	sitemaps, err := auditor.SitemapExtracts(ctx, tsHost)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sitemaps) != 2 {
		t.Fatalf("expected 2 sitemaps, got %d", len(sitemaps))
	}

	if sitemaps[0] != "http://example.com/sitemap.xml" {
		t.Errorf("expected sitemap.xml, got %s", sitemaps[0])
	}
}
