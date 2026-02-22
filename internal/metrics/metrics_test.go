package metrics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

func TestMetricsServer(t *testing.T) {
	srv := Start(8888)
	// Give it a tiny bit of time to start up
	time.Sleep(100 * time.Millisecond)

	defer srv.Stop(context.Background())

	// Record a scrape to verify metrics format correctly
	res := &storage.ScrapeResult{
		StatusCode:  200,
		DetectedBot: false,
		Body:        []byte("hello world"), // 11 bytes
		Duration:    1 * time.Second,
	}

	RecordScrape("example.com", res)

	resp, err := http.Get("http://localhost:8888/metrics")
	if err != nil {
		t.Fatalf("failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	output := string(body)

	if !strings.Contains(output, "burr_scrape_requests_total") {
		t.Errorf("expected burr_scrape_requests_total metric")
	}

	if !strings.Contains(output, `burr_scrape_duration_seconds_bucket`) {
		t.Errorf("expected burr_scrape_duration_seconds metric")
	}

	if !strings.Contains(output, `burr_scrape_bytes_total{domain="example.com"}`) {
		t.Errorf("expected burr_scrape_bytes_total metric for example.com")
	}
}
