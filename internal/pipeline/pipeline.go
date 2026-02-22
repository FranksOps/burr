package pipeline

import (
    "context"
    "fmt"

    "github.com/FranksOps/burr/internal/serp"
    "github.com/FranksOps/burr/internal/scraper"
)

// Pipeline orchestrates the three stages of the intel command: SERP search,
// crawling fetched domains, and term analysis. This stub wires the components
// together without performing real work, satisfying compilation.
type Pipeline struct {
    SERPProvider serp.SERPProvider
    Scraper      *scraper.Fetcher // placeholder; real implementation may differ
}

// Run executes the pipeline steps. It returns an error only if a required component
// is missing. The actual processing logic is to be implemented later.
func (p *Pipeline) Run(ctx context.Context, query string, terms []string, limit int) error {
    if p.SERPProvider == nil {
        return fmt.Errorf("SERPProvider is nil")
    }
    if p.Scraper == nil {
        return fmt.Errorf("Scraper is nil")
    }
    // Stage 1: search SERP
    domains, err := p.SERPProvider.Search(ctx, query, limit)
    if err != nil {
        return fmt.Errorf("search failed: %w", err)
    }
    // Stage 2 & 3 are placeholders (crawl domains, analyze content).
    _ = len(domains) // avoid unused variable error
    _ = terms        // silence unused warning
    return nil
}
