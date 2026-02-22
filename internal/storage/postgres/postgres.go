package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/FranksOps/burr/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ensure postgresBackend implements storage.Backend
var _ storage.Backend = (*postgresBackend)(nil)

type postgresBackend struct {
	pool *pgxpool.Pool
}

const schema = `
CREATE TABLE IF NOT EXISTS scrape_results (
	id TEXT PRIMARY KEY,
	url TEXT NOT NULL,
	method TEXT NOT NULL,
	status_code INTEGER NOT NULL,
	headers JSONB NOT NULL,
	body BYTEA,
	duration_ms BIGINT NOT NULL,
	detected_bot BOOLEAN NOT NULL,
	detection_src TEXT,
	created_at TIMESTAMPTZ NOT NULL,
	error TEXT
);
`

// New creates a new Postgres-backed storage.Backend.
func New(ctx context.Context, dsn string) (storage.Backend, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	_, err = pool.Exec(ctx, schema)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("context: %w", err)
	}

	return &postgresBackend{pool: pool}, nil
}

func (b *postgresBackend) Save(ctx context.Context, result *storage.ScrapeResult) error {
	headersJSON, err := json.Marshal(result.Headers)
	if err != nil {
		return fmt.Errorf("context: %w", err)
	}

	query := `
	INSERT INTO scrape_results (
		id, url, method, status_code, headers, body, duration_ms, detected_bot, detection_src, created_at, error
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err = b.pool.Exec(ctx, query,
		result.ID,
		result.URL,
		result.Method,
		result.StatusCode,
		headersJSON,
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

func (b *postgresBackend) Query(ctx context.Context, filter storage.Filter) ([]*storage.ScrapeResult, error) {
	query := `SELECT id, url, method, status_code, headers, body, duration_ms, detected_bot, detection_src, created_at, error FROM scrape_results WHERE 1=1`
	args := []any{}
	paramCount := 1

	if filter.URL != "" {
		query += fmt.Sprintf(` AND url = $%d`, paramCount)
		args = append(args, filter.URL)
		paramCount++
	}
	if filter.DetectedBot != nil {
		query += fmt.Sprintf(` AND detected_bot = $%d`, paramCount)
		args = append(args, *filter.DetectedBot)
		paramCount++
	}
	if filter.Since != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, paramCount)
		args = append(args, *filter.Since)
		paramCount++
	}

	query += ` ORDER BY created_at DESC`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, paramCount)
		args = append(args, filter.Limit)
		paramCount++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, paramCount)
		args = append(args, filter.Offset)
		paramCount++
	}

	rows, err := b.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}
	defer rows.Close()

	var results []*storage.ScrapeResult
	for rows.Next() {
		var r storage.ScrapeResult
		var headersJSON []byte
		var durationMs int64

		err := rows.Scan(
			&r.ID, &r.URL, &r.Method, &r.StatusCode, &headersJSON, &r.Body,
			&durationMs, &r.DetectedBot, &r.DetectionSrc, &r.CreatedAt, &r.Error,
		)
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		r.Duration = time.Duration(durationMs) * time.Millisecond
		if err := json.Unmarshal(headersJSON, &r.Headers); err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}

		results = append(results, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("context: %w", err)
	}

	return results, nil
}

func (b *postgresBackend) Close() error {
	b.pool.Close()
	return nil
}
