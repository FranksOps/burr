package jsonbackend

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/FranksOps/burr/internal/storage"
)

// ensure jsonBackend implements storage.Backend
var _ storage.Backend = (*jsonBackend)(nil)

type jsonBackend struct {
	mu   sync.Mutex
	file *os.File
}

// New creates a new NDJSON-backed storage.Backend.
func New(filePath string) (storage.Backend, error) {
	// Open file for appending, create if it doesn't exist
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	return &jsonBackend{
		file: f,
	}, nil
}

func (b *jsonBackend) Save(ctx context.Context, result *storage.ScrapeResult) error {
	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	_, err = b.file.Write(append(data, '\n'))
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return nil
}

func (b *jsonBackend) Query(ctx context.Context, filter storage.Filter) ([]*storage.ScrapeResult, error) {
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

	scanner := bufio.NewScanner(b.file)

	// In a real DB, offset/limit and ordering is handled by the engine.
	// For NDJSON, we read everything, filter in memory, and then slice/reverse.
	var allFiltered []*storage.ScrapeResult

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var r storage.ScrapeResult
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		// Apply filters
		if filter.URL != "" && r.URL != filter.URL {
			continue
		}
		if filter.DetectedBot != nil && r.DetectedBot != *filter.DetectedBot {
			continue
		}
		if filter.Since != nil && r.CreatedAt.Before(*filter.Since) {
			continue
		}

		allFiltered = append(allFiltered, &r)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("context: %w", err)
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

func (b *jsonBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.file.Close()
}
