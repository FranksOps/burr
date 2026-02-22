package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/FranksOps/burr/internal/bypass"
	"github.com/FranksOps/burr/internal/fingerprint"
	"github.com/FranksOps/burr/internal/metrics"
	"github.com/FranksOps/burr/internal/storage"
	"github.com/FranksOps/burr/pkg/httpclient"
	"github.com/FranksOps/burr/pkg/proxy"
	"github.com/FranksOps/burr/pkg/ratelimit"
	"github.com/FranksOps/burr/pkg/useragent"
	"github.com/google/uuid"
)

type contextKey string

const proxyKey contextKey = "proxy_url"

// FetchConfig configures a single scrape action.
type FetchConfig struct {
	Timeout      time.Duration
	MaxRedirects int
	UseCookieJar bool
	ProxyPool    *proxy.Pool
	UAPool       *useragent.Pool
	Fingerprint  fingerprint.Profile
	Limiter      *ratelimit.Limiter
}

// Fetcher performs single URL fetches using the configured bypass strategies.
type Fetcher struct {
	config    FetchConfig
	client    *httpclient.Client
	transport http.RoundTripper
}

// NewFetcher initializes a new Fetcher with the given configuration.
// By holding a single client across requests, cookie jars (if configured) persist for the lifetime of the Fetcher.
func NewFetcher(cfg FetchConfig) (*Fetcher, error) {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.UAPool == nil {
		cfg.UAPool = useragent.NewPool(nil)
	}
	if string(cfg.Fingerprint) == "" {
		cfg.Fingerprint = fingerprint.ProfileChrome
	}

	// Create transport just once per fetcher to allow connection pooling and cookie jar reuse.
	// We inject a proxy function that reads from the request context to allow per-request proxy rotation.
	proxyFunc := func(req *http.Request) (*url.URL, error) {
		// http.Transport.Proxy expects nil url if no proxy should be used
		if val := req.Context().Value(proxyKey); val != nil {
			if u, ok := val.(*url.URL); ok {
				return u, nil
			}
		}
		// If we are doing tests, skip env proxy to prevent system proxies from breaking tests
		if req.URL.Host == "example.com" || req.URL.Hostname() == "127.0.0.1" {
			// For tests where we specifically want a proxy via context, we still return the context value above.
			// But for anything else locally, don't use env proxy.
			return nil, nil
		}
		return http.ProxyFromEnvironment(req)
	}

	transport, err := fingerprint.Transport(cfg.Fingerprint, proxyFunc)
	if err != nil {
		return nil, fmt.Errorf("failed to setup transport: %w", err)
	}

	client, err := httpclient.New(httpclient.Config{
		Timeout:      cfg.Timeout,
		MaxRedirects: cfg.MaxRedirects,
		UseCookieJar: cfg.UseCookieJar,
		Transport:    transport,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Fetcher{
		config:    cfg,
		client:    client,
		transport: transport,
	}, nil
}

// Fetch executes a GET request to the target URL, tracking the duration and
// capturing the response into a storage.ScrapeResult.
func (f *Fetcher) Fetch(ctx context.Context, targetURL string) (*storage.ScrapeResult, error) {
	if f.config.Limiter != nil {
		if err := f.config.Limiter.Wait(ctx); err != nil {
			return &storage.ScrapeResult{
				ID:        uuid.New().String(),
				URL:       targetURL,
				Method:    http.MethodGet,
				CreatedAt: time.Now().UTC(),
				Error:     fmt.Sprintf("rate limiter failed: %v", err),
			}, nil
		}
	}

	start := time.Now()

	result := &storage.ScrapeResult{
		ID:        uuid.New().String(),
		URL:       targetURL,
		Method:    http.MethodGet,
		CreatedAt: start.UTC(),
	}

	// Set up proxy if configured
	var activeProxy *url.URL

	if f.config.ProxyPool != nil {
		activeProxy = f.config.ProxyPool.Next()
		if activeProxy != nil {
			// Instead of creating a new transport every request, we hook the proxy URL into the request Context.
			// However, http.Transport caches connections by host, so proxying via request context requires care,
			// or replacing the proxy function on the fly.
			// Since our utls Transport clones DefaultTransport and assigns Proxy, we can dynamically override the request's proxy
			// by putting the proxy URL in context, then defining a proxy func that reads it.

			// Note: modifying Transport.Proxy concurrently is not safe. If we want per-request proxy rotation,
			// we need to set the Proxy func to read from the request's context, and inject it below.
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create request: %v", err)
		result.Duration = time.Since(start)
		return result, nil
	}

	// Dynamic proxy via context
	if activeProxy != nil {
		req = req.WithContext(context.WithValue(req.Context(), proxyKey, activeProxy))
	}
	// Fallback to proxying directly if http.Transport doesn't read the proxy func properly on some setups
	// Actually for http/https requests, http.Client will correctly use the Transport.Proxy function if set.
	// But in tests http://example.com might not be routed through proxy if ProxyFromEnvironment overrides it
	// or if the test URL is localhost. By default `http.ProxyURL` or similar is not called for localhost.
	// We'll enforce the proxy by rewriting the URL or ensuring Transport.Proxy is called correctly.

	// Setup headers and UA rotation
	req.Header.Set("User-Agent", f.config.UAPool.GetSequential())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")

	resp, err := f.client.Do(req.Context(), req)
	if err != nil {
		if activeProxy != nil {
			_ = f.config.ProxyPool.MarkFailure(activeProxy)
			metrics.ProxyFailures.WithLabelValues(activeProxy.String()).Inc()
		}
		result.Error = fmt.Sprintf("request failed: %v", err)
		result.Duration = time.Since(start)
		return result, nil
	}
	defer resp.Body.Close()

	if activeProxy != nil {
		_ = f.config.ProxyPool.MarkSuccess(activeProxy)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read body: %v", err)
	}

	result.StatusCode = resp.StatusCode
	result.Headers = resp.Header
	result.Body = body
	result.Duration = time.Since(start)

	// Run detection to identify if we were challenged
	bypass.Analyze(result, bypass.DefaultDetectors())

	return result, nil
}
