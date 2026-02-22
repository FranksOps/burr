package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/temoto/robotstxt"
)

// RobotsTxtAuditor manages robots.txt fetching and enforcement.
type RobotsTxtAuditor struct {
	fetcher *Fetcher
	logger  *slog.Logger
	mu      sync.RWMutex
	cache   map[string]*robotstxt.RobotsData
}

// NewRobotsTxtAuditor creates a new instance.
func NewRobotsTxtAuditor(fetcher *Fetcher, logger *slog.Logger) *RobotsTxtAuditor {
	if logger == nil {
		logger = slog.Default()
	}
	return &RobotsTxtAuditor{
		fetcher: fetcher,
		logger:  logger,
		cache:   make(map[string]*robotstxt.RobotsData),
	}
}

// IsAllowed determines if the given URL is allowed by the host's robots.txt for the provided User-Agent.
func (r *RobotsTxtAuditor) IsAllowed(ctx context.Context, targetURL string, userAgent string) (bool, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return false, fmt.Errorf("invalid url: %w", err)
	}

	host := u.Scheme + "://" + u.Host

	data, err := r.getOrFetch(ctx, host)
	if err != nil {
		r.logger.Debug("robots.txt fetch failed, defaulting to allow", "host", host, "err", err)
		return true, nil
	}

	if data == nil {
		return true, nil
	}

	group := data.FindGroup(userAgent)
	return group.Test(u.Path), nil
}

func (r *RobotsTxtAuditor) getOrFetch(ctx context.Context, host string) (*robotstxt.RobotsData, error) {
	r.mu.RLock()
	data, exists := r.cache[host]
	r.mu.RUnlock()

	if exists {
		return data, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	data, exists = r.cache[host]
	if exists {
		return data, nil
	}

	robotsURL := fmt.Sprintf("%s/robots.txt", host)

	originalRedirects := r.fetcher.config.MaxRedirects
	r.fetcher.config.MaxRedirects = 5

	result, err := r.fetcher.Fetch(ctx, robotsURL)

	r.fetcher.config.MaxRedirects = originalRedirects

	if err != nil {
		r.cache[host] = nil
		return nil, fmt.Errorf("fetch error: %w", err)
	}

	if result.Error != "" {
		r.cache[host] = nil
		return nil, fmt.Errorf("fetch error: %s", result.Error)
	}

	if result.StatusCode >= 400 {
		r.cache[host] = nil
		return nil, nil
	}

	parsed, err := robotstxt.FromBytes(result.Body)
	if err != nil {
		r.cache[host] = nil
		return nil, fmt.Errorf("parse error: %w", err)
	}

	r.cache[host] = parsed
	return parsed, nil
}

// SitemapExtracts returns a list of sitemap URLs defined in the cached robots.txt for the given host.
func (r *RobotsTxtAuditor) SitemapExtracts(ctx context.Context, host string) ([]string, error) {
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}

	data, err := r.getOrFetch(ctx, host)
	if err != nil || data == nil {
		return nil, nil
	}

	return data.Sitemaps, nil
}
