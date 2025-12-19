package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PostgresBackend implements the Backend interface using PostgreSQL.
type PostgresBackend struct {
	db        *sql.DB
	tableName string
}

// PostgresConfig configures the Postgres backend.
type PostgresConfig struct {
	DB        *sql.DB
	TableName string
}

// NewPostgresBackend creates a new Postgres storage backend.
func NewPostgresBackend(cfg PostgresConfig) (*PostgresBackend, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	tableName := cfg.TableName
	if tableName == "" {
		tableName = "covenant_storage"
	}

	backend := &PostgresBackend{
		db:        cfg.DB,
		tableName: tableName,
	}

	// Create table if not exists
	if err := backend.ensureTable(); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return backend, nil
}

// ensureTable creates the storage table if it doesn't exist.
func (p *PostgresBackend) ensureTable() error {
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			key TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`, p.tableName)

	_, err := p.db.Exec(query)
	return err
}

// Save stores data at the given key.
func (p *PostgresBackend) Save(ctx context.Context, key string, data []byte) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (key, data, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = NOW()
	`, p.tableName)

	_, err := p.db.ExecContext(ctx, query, key, data)
	if err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}

	return nil
}

// Load retrieves data from the given key.
func (p *PostgresBackend) Load(ctx context.Context, key string) ([]byte, error) {
	query := fmt.Sprintf(`SELECT data FROM %s WHERE key = $1`, p.tableName)

	var data []byte
	err := p.db.QueryRowContext(ctx, query, key).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load: %w", err)
	}

	return data, nil
}

// List returns all keys matching the prefix.
func (p *PostgresBackend) List(ctx context.Context, prefix string) ([]string, error) {
	query := fmt.Sprintf(`SELECT key FROM %s WHERE key LIKE $1 ORDER BY key`, p.tableName)

	// Convert prefix to LIKE pattern
	pattern := prefix + "%"

	rows, err := p.db.QueryContext(ctx, query, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate: %w", err)
	}

	return keys, nil
}

// Delete removes the data at the given key.
func (p *PostgresBackend) Delete(ctx context.Context, key string) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE key = $1`, p.tableName)

	result, err := p.db.ExecContext(ctx, query, key)
	if err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return ErrNotFound
	}

	return nil
}

// Exists checks if a key exists.
func (p *PostgresBackend) Exists(ctx context.Context, key string) (bool, error) {
	query := fmt.Sprintf(`SELECT 1 FROM %s WHERE key = $1`, p.tableName)

	var exists int
	err := p.db.QueryRowContext(ctx, query, key).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return true, nil
}

// Transaction executes multiple operations atomically.
func (p *PostgresBackend) Transaction(ctx context.Context, ops []Operation) error {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, op := range ops {
		var query string
		var args []any

		switch op.Type {
		case OpSave:
			query = fmt.Sprintf(`
				INSERT INTO %s (key, data, updated_at)
				VALUES ($1, $2, NOW())
				ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = NOW()
			`, p.tableName)
			args = []any{op.Key, op.Data}
		case OpDelete:
			query = fmt.Sprintf(`DELETE FROM %s WHERE key = $1`, p.tableName)
			args = []any{op.Key}
		}

		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to execute operation on %s: %w", op.Key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Query provides custom query capabilities.
func (p *PostgresBackend) Query(ctx context.Context, whereClause string, args ...any) ([]KeyValue, error) {
	query := fmt.Sprintf(`SELECT key, data FROM %s WHERE %s`, p.tableName, whereClause)

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}
	defer rows.Close()

	var results []KeyValue
	for rows.Next() {
		var kv KeyValue
		if err := rows.Scan(&kv.Key, &kv.Value); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		results = append(results, kv)
	}

	return results, rows.Err()
}

// KeyValue represents a key-value pair from a query.
type KeyValue struct {
	Key   string
	Value []byte
}

// Search searches for keys matching patterns in the data.
// Uses PostgreSQL's JSONB operators if the data is JSON.
func (p *PostgresBackend) Search(ctx context.Context, jsonPath string, value any) ([]string, error) {
	// This assumes data is stored as JSON
	query := fmt.Sprintf(`
		SELECT key FROM %s
		WHERE data::jsonb @> $1::jsonb
	`, p.tableName)

	pattern := fmt.Sprintf(`{"%s": %q}`, jsonPath, value)

	rows, err := p.db.QueryContext(ctx, query, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// CreateIndexes creates indexes for better query performance.
func (p *PostgresBackend) CreateIndexes() error {
	indexes := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_key_prefix ON %s (key text_pattern_ops)`,
			strings.ReplaceAll(p.tableName, ".", "_"), p.tableName),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_updated ON %s (updated_at)`,
			strings.ReplaceAll(p.tableName, ".", "_"), p.tableName),
	}

	for _, idx := range indexes {
		if _, err := p.db.Exec(idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// Vacuum runs VACUUM on the table for maintenance.
func (p *PostgresBackend) Vacuum(ctx context.Context) error {
	// VACUUM cannot run in a transaction
	_, err := p.db.ExecContext(ctx, fmt.Sprintf(`VACUUM ANALYZE %s`, p.tableName))
	return err
}

// Stats returns storage statistics.
func (p *PostgresBackend) Stats(ctx context.Context) (*StorageStats, error) {
	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as count,
			COALESCE(SUM(LENGTH(data)), 0) as total_size,
			COALESCE(AVG(LENGTH(data)), 0) as avg_size
		FROM %s
	`, p.tableName)

	var stats StorageStats
	err := p.db.QueryRowContext(ctx, query).Scan(&stats.KeyCount, &stats.TotalSize, &stats.AvgValueSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return &stats, nil
}

// StorageStats contains storage statistics.
type StorageStats struct {
	KeyCount     int64
	TotalSize    int64
	AvgValueSize float64
}
