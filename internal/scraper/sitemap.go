package scraper

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	"github.com/oxffaa/gopher-parse-sitemap"
)

// SitemapFetcher is responsible for fetching and parsing sitemaps to discover seed URLs.
type SitemapFetcher struct {
	fetcher *Fetcher
	logger  *slog.Logger
}

// NewSitemapFetcher initializes a new SitemapFetcher.
func NewSitemapFetcher(fetcher *Fetcher, logger *slog.Logger) *SitemapFetcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &SitemapFetcher{
		fetcher: fetcher,
		logger:  logger,
	}
}

// FetchSitemap fetches a sitemap XML or sitemap index and recursively extracts all URLs.
func (s *SitemapFetcher) FetchSitemap(ctx context.Context, sitemapURL string) ([]string, error) {
	s.logger.Debug("fetching sitemap", "url", sitemapURL)

	// We use the same evasion fetcher as the crawler
	result, err := s.fetcher.Fetch(ctx, sitemapURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch sitemap: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("fetch error: %s", result.Error)
	}

	if result.StatusCode >= 400 {
		return nil, fmt.Errorf("bad status code: %d", result.StatusCode)
	}

	// parse sitemap
	var urls []string

	err = sitemap.Parse(bytes.NewReader(result.Body), func(e sitemap.Entry) error {
		urls = append(urls, e.GetLocation())
		return nil
	})

	if err != nil || len(urls) == 0 {
		// It might be a sitemap index or invalid XML
		var nestedSitemaps []string
		indexErr := sitemap.ParseIndex(bytes.NewReader(result.Body), func(e sitemap.IndexEntry) error {
			nestedSitemaps = append(nestedSitemaps, e.GetLocation())
			return nil
		})

		// If both parsing attempts fail, or if it parsed as an index but has 0 nested maps, assume it's invalid.
		if indexErr != nil || (len(urls) == 0 && len(nestedSitemaps) == 0) {
			return nil, fmt.Errorf("failed to parse as sitemap or index: %w", err)
		}

		// Recursively fetch nested sitemaps
		for _, nestedURL := range nestedSitemaps {
			nestedURLs, fetchErr := s.FetchSitemap(ctx, nestedURL)
			if fetchErr != nil {
				s.logger.Warn("failed to fetch nested sitemap", "url", nestedURL, "err", fetchErr)
				continue
			}
			urls = append(urls, nestedURLs...)
		}
	}

	return urls, nil
}
