package report

import (
	"encoding/json"
	"fmt"
	"io"
	"text/template"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

// Summary contains aggregated metrics about a crawl/scrape session.
type Summary struct {
	TotalRequests   int
	TotalErrors     int
	TotalDetections int
	StatusCodes     map[int]int
	DetectionsBySrc map[string]int
	TotalBytes      int64
	StartTime       time.Time
	EndTime         time.Time
	Duration        time.Duration
}

// GenerateSummary processes a slice of scrape results to generate summary metrics.
func GenerateSummary(results []*storage.ScrapeResult) Summary {
	s := Summary{
		StatusCodes:     make(map[int]int),
		DetectionsBySrc: make(map[string]int),
	}

	if len(results) == 0 {
		return s
	}

	s.StartTime = results[0].CreatedAt
	s.EndTime = results[0].CreatedAt

	for _, r := range results {
		s.TotalRequests++
		if r.Error != "" {
			s.TotalErrors++
		}
		if r.DetectedBot {
			s.TotalDetections++
			s.DetectionsBySrc[r.DetectionSrc]++
		}
		if r.StatusCode > 0 {
			s.StatusCodes[r.StatusCode]++
		}
		s.TotalBytes += int64(len(r.Body))

		if r.CreatedAt.Before(s.StartTime) {
			s.StartTime = r.CreatedAt
		}
		if r.CreatedAt.After(s.EndTime) {
			s.EndTime = r.CreatedAt
		}
	}

	s.Duration = s.EndTime.Sub(s.StartTime)
	return s
}

// WriteJSON writes the summary to the provided writer in JSON format.
func WriteJSON(w io.Writer, summary Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	return nil
}

// WriteText writes a human-readable text summary to the provided writer.
func WriteText(w io.Writer, summary Summary) error {
	const textTmpl = `Burr Audit Summary
------------------
Time:          {{.StartTime.Format "2006-01-02 15:04:05"}} - {{.EndTime.Format "2006-01-02 15:04:05"}}
Duration:      {{.Duration}}
Total Fetch:   {{.TotalRequests}} requests
Total Bytes:   {{.TotalBytes}} bytes
Total Errors:  {{.TotalErrors}}

Status Codes:
{{- range $code, $count := .StatusCodes}}
  {{$code}}: {{$count}}
{{- else}}
  None
{{- end}}

Detections: {{.TotalDetections}}
{{- range $src, $count := .DetectionsBySrc}}
  {{$src}}: {{$count}}
{{- else}}
  None
{{- end}}
`

	t, err := template.New("textReport").Parse(textTmpl)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	if err := t.Execute(w, summary); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return nil
}

// WriteHTML writes a basic HTML report to the provided writer.
func WriteHTML(w io.Writer, summary Summary) error {
	const htmlTmpl = `<!DOCTYPE html>
<html>
<head>
<title>Burr Audit Report</title>
<style>
  body { font-family: sans-serif; margin: 40px; color: #333; }
  h1 { border-bottom: 2px solid #ccc; padding-bottom: 10px; }
  .stat-card { display: inline-block; padding: 20px; margin: 10px 10px 10px 0; background: #f4f4f4; border-radius: 5px; min-width: 150px; }
  .stat-val { font-size: 24px; font-weight: bold; }
  table { border-collapse: collapse; margin-top: 10px; }
  th, td { padding: 8px 12px; border: 1px solid #ccc; text-align: left; }
  th { background: #eaeaea; }
</style>
</head>
<body>
  <h1>Burr Audit Report</h1>
  <p><strong>Time:</strong> {{.StartTime.Format "2006-01-02 15:04:05"}} to {{.EndTime.Format "2006-01-02 15:04:05"}} ({{.Duration}})</p>
  
  <div class="stat-card">
    <div>Total Requests</div>
    <div class="stat-val">{{.TotalRequests}}</div>
  </div>
  <div class="stat-card">
    <div>Errors</div>
    <div class="stat-val">{{.TotalErrors}}</div>
  </div>
  <div class="stat-card">
    <div>Detections</div>
    <div class="stat-val" style="color: {{if gt .TotalDetections 0}}red{{else}}green{{end}};">{{.TotalDetections}}</div>
  </div>
  <div class="stat-card">
    <div>Total Bytes</div>
    <div class="stat-val">{{.TotalBytes}}</div>
  </div>

  <h3>Status Codes</h3>
  <table>
    <tr><th>Code</th><th>Count</th></tr>
    {{- range $code, $count := .StatusCodes}}
    <tr><td>{{$code}}</td><td>{{$count}}</td></tr>
    {{- else}}
    <tr><td colspan="2">None</td></tr>
    {{- end}}
  </table>

  <h3>Detections By Source</h3>
  <table>
    <tr><th>Source</th><th>Count</th></tr>
    {{- range $src, $count := .DetectionsBySrc}}
    <tr><td>{{$src}}</td><td>{{$count}}</td></tr>
    {{- else}}
    <tr><td colspan="2">None</td></tr>
    {{- end}}
  </table>
</body>
</html>
`
	t, err := template.New("htmlReport").Parse(htmlTmpl)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	if err := t.Execute(w, summary); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return nil
}
