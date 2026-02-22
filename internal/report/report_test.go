package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

func TestGenerateSummary(t *testing.T) {
	now := time.Now()

	results := []*storage.ScrapeResult{
		{
			StatusCode: 200,
			Body:       []byte("123"),
			CreatedAt:  now,
		},
		{
			StatusCode:   403,
			Body:         []byte("1234"),
			CreatedAt:    now.Add(1 * time.Second),
			DetectedBot:  true,
			DetectionSrc: "Cloudflare",
		},
		{
			StatusCode: 0,
			Body:       []byte(""),
			CreatedAt:  now.Add(2 * time.Second),
			Error:      "timeout",
		},
	}

	summary := GenerateSummary(results)

	if summary.TotalRequests != 3 {
		t.Errorf("expected 3 total requests, got %d", summary.TotalRequests)
	}

	if summary.TotalErrors != 1 {
		t.Errorf("expected 1 error, got %d", summary.TotalErrors)
	}

	if summary.TotalDetections != 1 {
		t.Errorf("expected 1 detection, got %d", summary.TotalDetections)
	}

	if summary.DetectionsBySrc["Cloudflare"] != 1 {
		t.Errorf("expected 1 CF detection, got %d", summary.DetectionsBySrc["Cloudflare"])
	}

	if summary.StatusCodes[200] != 1 {
		t.Errorf("expected 1 200 OK, got %d", summary.StatusCodes[200])
	}

	if summary.StatusCodes[403] != 1 {
		t.Errorf("expected 1 403 Forbidden, got %d", summary.StatusCodes[403])
	}

	if summary.TotalBytes != 7 {
		t.Errorf("expected 7 total bytes, got %d", summary.TotalBytes)
	}

	if summary.Duration != 2*time.Second {
		t.Errorf("expected 2s duration, got %v", summary.Duration)
	}
}

func TestWriteJSON(t *testing.T) {
	summary := Summary{
		TotalRequests: 5,
	}
	var buf bytes.Buffer
	err := WriteJSON(&buf, summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), `"TotalRequests": 5`) {
		t.Errorf("expected JSON to contain TotalRequests: 5")
	}
}

func TestWriteText(t *testing.T) {
	summary := Summary{
		TotalRequests: 5,
		TotalErrors:   1,
		StatusCodes: map[int]int{
			200: 4,
			500: 1,
		},
	}
	var buf bytes.Buffer
	err := WriteText(&buf, summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Total Fetch:   5 requests") {
		t.Errorf("expected text to contain Total Fetch: 5")
	}
	if !strings.Contains(out, "200: 4") {
		t.Errorf("expected text to contain 200: 4")
	}
}

func TestWriteHTML(t *testing.T) {
	summary := Summary{
		TotalRequests:   10,
		TotalDetections: 2,
		DetectionsBySrc: map[string]int{
			"DataDome": 2,
		},
	}
	var buf bytes.Buffer
	err := WriteHTML(&buf, summary)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "<title>Burr Audit Report</title>") {
		t.Errorf("expected HTML title")
	}
	if !strings.Contains(out, "DataDome") {
		t.Errorf("expected HTML to contain DataDome")
	}
}
