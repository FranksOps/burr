package serp

import (
    "context"
    "fmt"
)

// GoogleScrape is a placeholder implementation of SERPProvider that would
// perform a Google search via scraping. For now it returns an empty result set.
type GoogleScrape struct{}

// Search performs a search on Google and returns a slice of discovered domains.
// The current stub does not perform any network operations and simply returns an
// empty slice. It satisfies the SERPProvider interface.
func (g *GoogleScrape) Search(ctx context.Context, query string, limit int) ([]Domain, error) {
    // TODO: implement actual scraping with utls and UA rotation.
    // For now return empty slice to satisfy compilation.
    if limit < 0 {
        return nil, fmt.Errorf("limit cannot be negative: %d", limit)
    }
    return []Domain{}, nil
}
