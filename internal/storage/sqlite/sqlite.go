package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FranksOps/burr/internal/storage"
	_ "modernc.org/sqlite"
)

// ensure sqliteBackend implements storage.Backend
var _ storage.Backend = (*sqliteBackend)(nil)

type sqliteBackend struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS scrape_results (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	method TEXT NOT NULL,
	status_code INTEGER NOT NULL,
	headers TEXT NOT NULL,
	body BLOB,
	duration_ms INTEGER NOT NULL,
	detected_bot BOOLEAN NOT NULL,
	detection_src TEXT,
	created_at DATETIME NOT NULL,
	error TEXT
);
`

// New creates a new SQLite-backed storage.Backend.
func New(dsn string) (storage.Backend, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("context: %w", err)
	}

	return &sqliteBackend{db: db}, nil
}

func (b *sqliteBackend) Save(ctx context.Context, result *storage.ScrapeResult) error {
	headersJSON, err := json.Marshal(result.Headers)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	query := `
	INSERT INTO scrape_results (
		id, url, method, status_code, headers, body, duration_ms, detected_bot, detection_src, created_at, error
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = b.db.ExecContext(ctx, query,
		result.ID,
		result.URL,
		result.Method,
		result.StatusCode,
		string(headersJSON),
		result.Body,
		result.Duration.Milliseconds(),
		result.DetectedBot,
		result.DetectionSrc,
		result.CreatedAt,
		result.Error,
	)

	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	return nil
}

func (b *sqliteBackend) Query(ctx context.Context, filter storage.Filter) ([]*storage.ScrapeResult, error) {
	query := `SELECT id, url, method, status_code, headers, body, duration_ms, detected_bot, detection_src, created_at, error FROM scrape_results WHERE 1=1`
	args := []any{}

	if filter.URL != "" {
		query += ` AND url = ?`
		args = append(args, filter.URL)
	}
	if filter.DetectedBot != nil {
		query += ` AND detected_bot = ?`
		args = append(args, *filter.DetectedBot)
	}
	if filter.Since != nil {
		query += ` AND created_at >= ?`
		args = append(args, *filter.Since)
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, filter.Offset)
	}

	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}
	defer rows.Close()

	var results []*storage.ScrapeResult
	for rows.Next() {
		var r storage.ScrapeResult
		var headersJSON string
		var durationMs int64

		err := rows.Scan(
			&r.ID, &r.URL, &r.Method, &r.StatusCode, &headersJSON, &r.Body,
			&durationMs, &r.DetectedBot, &r.DetectionSrc, &r.CreatedAt, &r.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		r.Duration = time.Duration(durationMs) * time.Millisecond
		if err := json.Unmarshal([]byte(headersJSON), &r.Headers); err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		results = append(results, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	return results, nil
}

func (b *sqliteBackend) Close() error {
	return b.db.Close()
}
