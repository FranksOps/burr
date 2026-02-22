package storage

import (
	"context"
	"testing"
	"time"
)

// ensure ScrapeResult compiles and has the fields expected
func TestScrapeResult_Types(t *testing.T) {
	_ = ScrapeResult{
		ID:           "test1234",
		URL:          "http://example.com",
		Method:       "GET",
		StatusCode:   200,
		Headers:      map[string][]string{"X-Test": {"true"}},
		Body:         []byte("hello"),
		Duration:     10 * time.Millisecond,
		DetectedBot:  false,
		DetectionSrc: "",
		CreatedAt:    time.Now(),
		Error:        "",
	}

	boolTrue := true
	now := time.Now()
	_ = Filter{
		URL:         "http://example.com",
		DetectedBot: &boolTrue,
		Since:       &now,
		Limit:       10,
		Offset:      0,
	}
}

// Ensure Backend interface exists and is implementable
type mockBackend struct{}

func (m *mockBackend) Save(ctx context.Context, result *ScrapeResult) error { return nil }
func (m *mockBackend) Query(ctx context.Context, filter Filter) ([]*ScrapeResult, error) {
	return nil, nil
}
func (m *mockBackend) Close() error { return nil }

func TestBackendInterface(t *testing.T) {
	var b Backend = &mockBackend{}
	_ = b
}
