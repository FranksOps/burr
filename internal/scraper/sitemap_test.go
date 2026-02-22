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

func TestSitemapFetcher_FlatSitemap(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
   <url>
      <loc>http://example.com/</loc>
      <lastmod>2023-01-01</lastmod>
      <changefreq>monthly</changefreq>
      <priority>0.8</priority>
   </url>
   <url>
      <loc>http://example.com/page1</loc>
   </url>
</urlset>`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	sf := NewSitemapFetcher(fetcher, slog.Default())
	ctx := context.Background()

	urls, err := sf.FetchSitemap(ctx, ts.URL+"/sitemap.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}

	if urls[0] != "http://example.com/" {
		t.Errorf("expected first url to be http://example.com/, got %s", urls[0])
	}
	if urls[1] != "http://example.com/page1" {
		t.Errorf("expected second url to be http://example.com/page1, got %s", urls[1])
	}
}

func TestSitemapFetcher_SitemapIndex(t *testing.T) {
	mux := http.NewServeMux()

	// Track the base URL to inject into the test XML
	var baseURL string

	mux.HandleFunc("/sitemap_index.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
   <sitemap>
      <loc>` + baseURL + `/sitemap1.xml</loc>
   </sitemap>
   <sitemap>
      <loc>` + baseURL + `/sitemap2.xml</loc>
   </sitemap>
</sitemapindex>`))
	})

	mux.HandleFunc("/sitemap1.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
   <url><loc>http://example.com/s1-1</loc></url>
</urlset>`))
	})

	mux.HandleFunc("/sitemap2.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
   <url><loc>http://example.com/s2-1</loc></url>
   <url><loc>http://example.com/s2-2</loc></url>
</urlset>`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()
	baseURL = ts.URL

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	sf := NewSitemapFetcher(fetcher, slog.Default())
	ctx := context.Background()

	urls, err := sf.FetchSitemap(ctx, ts.URL+"/sitemap_index.xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs from nested sitemaps, got %d", len(urls))
	}

	expected := map[string]bool{
		"http://example.com/s1-1": true,
		"http://example.com/s2-1": true,
		"http://example.com/s2-2": true,
	}

	for _, u := range urls {
		if !expected[u] {
			t.Errorf("unexpected URL parsed: %s", u)
		}
	}
}

func TestSitemapFetcher_InvalidXML(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`this is not xml`))
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	fetcher, _ := NewFetcher(FetchConfig{
		Timeout:     5 * time.Second,
		Fingerprint: fingerprint.ProfileGo,
	})

	sf := NewSitemapFetcher(fetcher, slog.Default())
	ctx := context.Background()

	_, err := sf.FetchSitemap(ctx, ts.URL+"/sitemap.xml")
	if err == nil || !strings.Contains(err.Error(), "failed to parse as sitemap or index") {
		t.Errorf("expected parsing error, got: %v", err)
	}
}
