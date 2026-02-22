package serp

import "context"

// Domain represents a discovered domain from a SERP search.
// Only the URL is required for now; additional metadata can be added later.
type Domain struct {
    URL string `json:"url"`
}

// SERPProvider abstracts a search engine provider that can return a list of domains
// for a given query. Implementations may use scraping, official APIs, or other
// mechanisms. The limit parameter caps the number of results returned.
type SERPProvider interface {
    Search(ctx context.Context, query string, limit int) ([]Domain, error)
}
