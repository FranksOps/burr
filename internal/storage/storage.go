package storage

import (
	"context"
	"time"
)

// ScrapeResult represents the outcome of a single scrape action.
type ScrapeResult struct {
	ID           string
	URL          string
	Method       string
	StatusCode   int
	Headers      map[string][]string
	Body         []byte
	Duration     time.Duration
	DetectedBot  bool
	DetectionSrc string // e.g. "Cloudflare", "Akamai", "PerimeterX", "DataDome"
	CreatedAt    time.Time
	Error        string // non-empty if the scrape failed before HTTP response
}

// Filter allows querying for specific ScrapeResults.
type Filter struct {
	URL         string
	DetectedBot *bool
	Since       *time.Time
	Limit       int
	Offset      int
}

// Backend defines the interface for storing and querying scrape results.
type Backend interface {
	Save(ctx context.Context, result *ScrapeResult) error
	Query(ctx context.Context, filter Filter) ([]*ScrapeResult, error)
	Close() error
}
