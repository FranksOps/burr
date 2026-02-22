package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

func TestSQLiteBackend(t *testing.T) {
	// Use an in-memory database for testing
	dsn := "file::memory:?cache=shared"
	b, err := New(dsn)
	if err != nil {
		t.Fatalf("Failed to create SQLite backend: %v", err)
	}
	defer b.Close()

	ctx := context.Background()
	now := time.Now().UTC() // SQLite stores UTC well

	res := &storage.ScrapeResult{
		ID:           "test1234",
		URL:          "http://example.com",
		Method:       "GET",
		StatusCode:   200,
		Headers:      map[string][]string{"Content-Type": {"text/html"}},
		Body:         []byte("hello world"),
		Duration:     50 * time.Millisecond,
		DetectedBot:  true,
		DetectionSrc: "Cloudflare",
		CreatedAt:    now,
		Error:        "",
	}

	err = b.Save(ctx, res)
	if err != nil {
		t.Fatalf("Failed to save result: %v", err)
	}

	// Test Query
	filter := storage.Filter{
		URL: "http://example.com",
	}

	results, err := b.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query results: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	got := results[0]
	if got.ID != res.ID {
		t.Errorf("Expected ID %s, got %s", res.ID, got.ID)
	}
	if got.URL != res.URL {
		t.Errorf("Expected URL %s, got %s", res.URL, got.URL)
	}
	if got.Method != res.Method {
		t.Errorf("Expected Method %s, got %s", res.Method, got.Method)
	}
	if got.StatusCode != res.StatusCode {
		t.Errorf("Expected StatusCode %d, got %d", res.StatusCode, got.StatusCode)
	}
	if got.Headers["Content-Type"][0] != res.Headers["Content-Type"][0] {
		t.Errorf("Expected Headers %v, got %v", res.Headers, got.Headers)
	}
	if string(got.Body) != string(res.Body) {
		t.Errorf("Expected Body %s, got %s", string(res.Body), string(got.Body))
	}
	// Note: precision might be lost if we only store ms
	if got.Duration.Milliseconds() != res.Duration.Milliseconds() {
		t.Errorf("Expected Duration %v, got %v", res.Duration, got.Duration)
	}
	if got.DetectedBot != res.DetectedBot {
		t.Errorf("Expected DetectedBot %v, got %v", res.DetectedBot, got.DetectedBot)
	}
	if got.DetectionSrc != res.DetectionSrc {
		t.Errorf("Expected DetectionSrc %s, got %s", res.DetectionSrc, got.DetectionSrc)
	}
	if got.CreatedAt.Unix() != res.CreatedAt.Unix() {
		t.Errorf("Expected CreatedAt %v, got %v", res.CreatedAt, got.CreatedAt)
	}
	if got.Error != res.Error {
		t.Errorf("Expected Error %s, got %s", res.Error, got.Error)
	}

	// Test Since filter
	past := now.Add(-1 * time.Hour)
	filterSince := storage.Filter{Since: &past}
	resultsSince, err := b.Query(ctx, filterSince)
	if err != nil {
		t.Fatalf("Failed to query results with Since: %v", err)
	}
	if len(resultsSince) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resultsSince))
	}

	// Test DetectedBot filter
	boolTrue := true
	filterDetected := storage.Filter{DetectedBot: &boolTrue}
	resultsDetected, err := b.Query(ctx, filterDetected)
	if err != nil {
		t.Fatalf("Failed to query results with DetectedBot: %v", err)
	}
	if len(resultsDetected) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resultsDetected))
	}

	boolFalse := false
	filterNotDetected := storage.Filter{DetectedBot: &boolFalse}
	resultsNotDetected, err := b.Query(ctx, filterNotDetected)
	if err != nil {
		t.Fatalf("Failed to query results with DetectedBot=false: %v", err)
	}
	if len(resultsNotDetected) != 0 {
		t.Fatalf("Expected 0 results, got %d", len(resultsNotDetected))
	}
}
