package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

func TestPostgresBackend(t *testing.T) {
	// Only run this test if BURR_TEST_PG_DSN is set
	dsn := os.Getenv("BURR_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("Skipping Postgres backend test: BURR_TEST_PG_DSN not set")
	}

	ctx := context.Background()
	b, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("Failed to create Postgres backend: %v", err)
	}
	defer b.Close()

	now := time.Now().UTC()

	res := &storage.ScrapeResult{
		ID:           "testpg1234",
		URL:          "http://example-pg.com",
		Method:       "GET",
		StatusCode:   200,
		Headers:      map[string][]string{"Content-Type": {"application/json"}},
		Body:         []byte(`{"hello":"pg"}`),
		Duration:     50 * time.Millisecond,
		DetectedBot:  true,
		DetectionSrc: "DataDome",
		CreatedAt:    now,
		Error:        "",
	}

	err = b.Save(ctx, res)
	if err != nil {
		t.Fatalf("Failed to save result: %v", err)
	}

	// Test Query
	filter := storage.Filter{
		URL: "http://example-pg.com",
	}

	results, err := b.Query(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to query results: %v", err)
	}

	// Can be more than 1 if tests run repeatedly, so we just check the most recent
	if len(results) < 1 {
		t.Fatalf("Expected at least 1 result, got %d", len(results))
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

	// Postgres timestamps might differ slightly in sub-millisecond precision
	// compared to Go time.Now(), checking Unix seconds is usually safe enough
	if got.CreatedAt.Unix() != res.CreatedAt.Unix() {
		t.Errorf("Expected CreatedAt %v, got %v", res.CreatedAt, got.CreatedAt)
	}
	if got.Error != res.Error {
		t.Errorf("Expected Error %s, got %s", res.Error, got.Error)
	}

	// Test Since filter
	past := now.Add(-1 * time.Hour)
	filterSince := storage.Filter{URL: "http://example-pg.com", Since: &past}
	resultsSince, err := b.Query(ctx, filterSince)
	if err != nil {
		t.Fatalf("Failed to query results with Since: %v", err)
	}
	if len(resultsSince) < 1 {
		t.Fatalf("Expected at least 1 result, got %d", len(resultsSince))
	}
}
