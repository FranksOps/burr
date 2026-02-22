package csvbackend

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

func TestCSVBackend(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "burr.csv")

	b, err := New(filePath)
	if err != nil {
		t.Fatalf("Failed to create CSV backend: %v", err)
	}
	defer b.Close()

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond) // Format truncates precision

	res1 := &storage.ScrapeResult{
		ID:           "csv1",
		URL:          "http://example.com/csv1",
		Method:       "GET",
		StatusCode:   200,
		Headers:      map[string][]string{"X-Test": {"true"}},
		Body:         []byte("csv1 body"),
		Duration:     10 * time.Millisecond,
		DetectedBot:  false,
		DetectionSrc: "",
		CreatedAt:    now.Add(-2 * time.Hour),
		Error:        "",
	}

	res2 := &storage.ScrapeResult{
		ID:           "csv2",
		URL:          "http://example.com/csv2",
		Method:       "GET",
		StatusCode:   403,
		Headers:      map[string][]string{"Server": {"cloudflare"}},
		Body:         []byte("cf challenge"),
		Duration:     20 * time.Millisecond,
		DetectedBot:  true,
		DetectionSrc: "Cloudflare",
		CreatedAt:    now.Add(-1 * time.Hour),
		Error:        "",
	}

	err = b.Save(ctx, res1)
	if err != nil {
		t.Fatalf("Failed to save result 1: %v", err)
	}
	err = b.Save(ctx, res2)
	if err != nil {
		t.Fatalf("Failed to save result 2: %v", err)
	}

	// Test URL Filter
	filterURL := storage.Filter{URL: "http://example.com/csv2"}
	resultsURL, err := b.Query(ctx, filterURL)
	if err != nil {
		t.Fatalf("Failed to query by URL: %v", err)
	}
	if len(resultsURL) != 1 {
		t.Fatalf("Expected 1 result for URL filter, got %d", len(resultsURL))
	}
	if resultsURL[0].ID != "csv2" {
		t.Errorf("Expected ID csv2, got %s", resultsURL[0].ID)
	}

	// Test DetectedBot Filter
	boolTrue := true
	filterBot := storage.Filter{DetectedBot: &boolTrue}
	resultsBot, err := b.Query(ctx, filterBot)
	if err != nil {
		t.Fatalf("Failed to query by DetectedBot: %v", err)
	}
	if len(resultsBot) != 1 {
		t.Fatalf("Expected 1 result for DetectedBot filter, got %d", len(resultsBot))
	}

	boolFalse := false
	filterNotBot := storage.Filter{DetectedBot: &boolFalse}
	resultsNotBot, err := b.Query(ctx, filterNotBot)
	if err != nil {
		t.Fatalf("Failed to query by DetectedBot=false: %v", err)
	}
	if len(resultsNotBot) != 1 {
		t.Fatalf("Expected 1 result for DetectedBot=false filter, got %d", len(resultsNotBot))
	}

	// Test Since Filter
	past := now.Add(-90 * time.Minute)
	filterSince := storage.Filter{Since: &past}
	resultsSince, err := b.Query(ctx, filterSince)
	if err != nil {
		t.Fatalf("Failed to query by Since: %v", err)
	}
	if len(resultsSince) != 1 {
		t.Fatalf("Expected 1 result for Since filter, got %d", len(resultsSince))
	}
	if resultsSince[0].ID != "csv2" {
		t.Errorf("Expected ID csv2, got %s", resultsSince[0].ID)
	}

	// Test no filters, ordering
	resultsAll, err := b.Query(ctx, storage.Filter{})
	if err != nil {
		t.Fatalf("Failed to query all: %v", err)
	}
	if len(resultsAll) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(resultsAll))
	}
	// Order should be descending (newest first)
	if resultsAll[0].ID != "csv2" {
		t.Errorf("Expected csv2 first, got %s", resultsAll[0].ID)
	}

	// Test body roundtrip base64
	if string(resultsAll[0].Body) != "cf challenge" {
		t.Errorf("Expected body 'cf challenge', got %s", string(resultsAll[0].Body))
	}

	// Test limit
	resultsLimit, err := b.Query(ctx, storage.Filter{Limit: 1})
	if err != nil {
		t.Fatalf("Failed to query limit: %v", err)
	}
	if len(resultsLimit) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resultsLimit))
	}

	// Test offset
	resultsOffset, err := b.Query(ctx, storage.Filter{Offset: 1})
	if err != nil {
		t.Fatalf("Failed to query offset: %v", err)
	}
	if len(resultsOffset) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(resultsOffset))
	}
	if resultsOffset[0].ID != "csv1" {
		t.Errorf("Expected csv1 for offset 1, got %s", resultsOffset[0].ID)
	}
}
