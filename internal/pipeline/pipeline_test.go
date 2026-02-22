package pipeline

import (
    "context"
    "testing"
    "time"

    "github.com/FranksOps/burr/internal/serp"
    "github.com/FranksOps/burr/internal/scraper"
    "github.com/FranksOps/burr/internal/fingerprint"
)

// mockSERP implements SERPProvider for testing.
type mockSERP struct{}

func (m *mockSERP) Search(ctx context.Context, query string, limit int) ([]serp.Domain, error) {
    return []serp.Domain{{URL: "https://example.com"}}, nil
}

func TestPipeline_Run(t *testing.T) {
    ctx := context.Background()
    p := Pipeline{SERPProvider: &mockSERP{}}
    // The Scraper field is required for Run; provide a minimal stub.
    // We'll use the real fetcher with minimal config that won't be used because we don't
    // actually crawl in this test. Provide nil to trigger expected error if not handled.
    // To keep the test passing, set a non-nil pointer to avoid the error.
    // create a real fetcher with minimal config
    fetcher, err := scraper.NewFetcher(scraper.FetchConfig{Timeout: 1 * time.Second, Fingerprint: fingerprint.ProfileGo})
    if err != nil {
        t.Fatalf("failed to create fetcher: %v", err)
    }
    defer fetcher.Close()
    p.Scraper = fetcher
    if err := p.Run(ctx, "test query", []string{"term"}, 1); err != nil {
        t.Fatalf("pipeline run failed: %v", err)
    }
}
