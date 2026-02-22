package csvbackend

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/FranksOps/burr/internal/storage"
)

// ensure csvBackend implements storage.Backend
var _ storage.Backend = (*csvBackend)(nil)

type csvBackend struct {
	mu   sync.Mutex
	file *os.File
}

// headers defines the CSV column order
var headers = []string{
	"id",
	"url",
	"method",
	"status_code",
	"headers_json",
	"body_base64",
	"duration_ms",
	"detected_bot",
	"detection_src",
	"created_at",
	"error",
}

// New creates a new CSV-backed storage.Backend.
func New(filePath string) (storage.Backend, error) {
	// Open file for appending, create if it doesn't exist
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	// Check if file is empty to write headers
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("context: %w", err)
	}

	if info.Size() == 0 {
		w := csv.NewWriter(f)
		if err := w.Write(headers); err != nil {
			f.Close()
			return nil, fmt.Errorf("context: %w", err)
		}
		w.Flush()
		if err := w.Error(); err != nil {
			f.Close()
			return nil, fmt.Errorf("context: %w", err)
		}
	}

	return &csvBackend{
		file: f,
	}, nil
}

func (b *csvBackend) Save(ctx context.Context, result *storage.ScrapeResult) error {
	headersJSON, err := json.Marshal(result.Headers)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	bodyBase64 := base64.StdEncoding.EncodeToString(result.Body)

	record := []string{
		result.ID,
		result.URL,
		result.Method,
		strconv.Itoa(result.StatusCode),
		string(headersJSON),
		bodyBase64,
		strconv.FormatInt(result.Duration.Milliseconds(), 10),
		strconv.FormatBool(result.DetectedBot),
		result.DetectionSrc,
		result.CreatedAt.Format(time.RFC3339Nano),
		result.Error,
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Ensure we're at the end of the file for appending (just in case)
	if _, err := b.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	w := csv.NewWriter(b.file)
	if err := w.Write(record); err != nil {
		return fmt.Errorf("context: %w", err)
	}
	w.Flush()

	if err := w.Error(); err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return nil
}

func (b *csvBackend) Query(ctx context.Context, filter storage.Filter) ([]*storage.ScrapeResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Seek to the beginning of the file to read all entries
	if _, err := b.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}
	defer func() {
		// Restore pointer to end for writing
		_, _ = b.file.Seek(0, io.SeekEnd)
	}()

	r := csv.NewReader(b.file)

	// Read headers
	_, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return []*storage.ScrapeResult{}, nil
		}
		return nil, fmt.Errorf("context: %w", err)
	}

	var allFiltered []*storage.ScrapeResult

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		if len(record) != len(headers) {
			continue // skip malformed rows
		}

		statusCode, _ := strconv.Atoi(record[3])
		var reqHeaders map[string][]string
		if err := json.Unmarshal([]byte(record[4]), &reqHeaders); err != nil {
			// fallback to empty if parse fails
			reqHeaders = map[string][]string{}
		}
		body, _ := base64.StdEncoding.DecodeString(record[5])
		durationMs, _ := strconv.ParseInt(record[6], 10, 64)
		detectedBot, _ := strconv.ParseBool(record[7])
		createdAt, _ := time.Parse(time.RFC3339Nano, record[9])

		res := &storage.ScrapeResult{
			ID:           record[0],
			URL:          record[1],
			Method:       record[2],
			StatusCode:   statusCode,
			Headers:      reqHeaders,
			Body:         body,
			Duration:     time.Duration(durationMs) * time.Millisecond,
			DetectedBot:  detectedBot,
			DetectionSrc: record[8],
			CreatedAt:    createdAt,
			Error:        record[10],
		}

		// Apply filters
		if filter.URL != "" && res.URL != filter.URL {
			continue
		}
		if filter.DetectedBot != nil && res.DetectedBot != *filter.DetectedBot {
			continue
		}
		if filter.Since != nil && res.CreatedAt.Before(*filter.Since) {
			continue
		}

		allFiltered = append(allFiltered, res)
	}

	// Order by created_at DESC (reverse the slice)
	for i, j := 0, len(allFiltered)-1; i < j; i, j = i+1, j-1 {
		allFiltered[i], allFiltered[j] = allFiltered[j], allFiltered[i]
	}

	// Apply Offset
	if filter.Offset > 0 {
		if filter.Offset >= len(allFiltered) {
			return []*storage.ScrapeResult{}, nil
		}
		allFiltered = allFiltered[filter.Offset:]
	}

	// Apply Limit
	if filter.Limit > 0 && filter.Limit < len(allFiltered) {
		allFiltered = allFiltered[:filter.Limit]
	}

	return allFiltered, nil
}

func (b *csvBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.file.Close()
}
