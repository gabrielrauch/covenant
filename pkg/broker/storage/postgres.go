package storage

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// validTableName matches valid PostgreSQL identifiers (letters, digits, underscores).
var validTableName = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// PostgresBackend implements the Backend interface using PostgreSQL.
type PostgresBackend struct {
	db        *sql.DB
	tableName string
	queries   postgresQueries
}

// postgresQueries holds pre-built SQL queries to avoid fmt.Sprintf at runtime.
type postgresQueries struct {
	createTable    string
	save           string
	load           string
	list           string
	delete         string
	exists         string
	txSave         string
	txDelete       string
	vacuum         string
	stats          string
	indexKeyPrefix string
	indexUpdated   string
}

// PostgresConfig configures the Postgres backend.
type PostgresConfig struct {
	DB        *sql.DB
	TableName string
}

// buildPostgresQueries builds all SQL queries for a given table name.
// This is called once during construction after table name validation.
func buildPostgresQueries(tableName string) postgresQueries {
	return postgresQueries{
		createTable: `CREATE TABLE IF NOT EXISTS ` + tableName + ` (
			key TEXT PRIMARY KEY,
			data BYTEA NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,
		save: `INSERT INTO ` + tableName + ` (key, data, updated_at)
			VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = NOW()`,
		load:           `SELECT data FROM ` + tableName + ` WHERE key = $1`,
		list:           `SELECT key FROM ` + tableName + ` WHERE key LIKE $1 ORDER BY key`,
		delete:         `DELETE FROM ` + tableName + ` WHERE key = $1`,
		exists:         `SELECT 1 FROM ` + tableName + ` WHERE key = $1`,
		txSave:         `INSERT INTO ` + tableName + ` (key, data, updated_at) VALUES ($1, $2, NOW()) ON CONFLICT (key) DO UPDATE SET data = $2, updated_at = NOW()`,
		txDelete:       `DELETE FROM ` + tableName + ` WHERE key = $1`,
		vacuum:         `VACUUM ANALYZE ` + tableName,
		stats:          `SELECT COUNT(*) as count, COALESCE(SUM(LENGTH(data)), 0) as total_size, COALESCE(AVG(LENGTH(data)), 0) as avg_size FROM ` + tableName,
		indexKeyPrefix: `CREATE INDEX IF NOT EXISTS idx_` + strings.ReplaceAll(tableName, ".", "_") + `_key_prefix ON ` + tableName + ` (key text_pattern_ops)`,
		indexUpdated:   `CREATE INDEX IF NOT EXISTS idx_` + strings.ReplaceAll(tableName, ".", "_") + `_updated ON ` + tableName + ` (updated_at)`,
	}
}

// NewPostgresBackend creates a new Postgres storage backend.
func NewPostgresBackend(ctx context.Context, cfg PostgresConfig) (*PostgresBackend, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}

	// Verify database connection is valid
	if err := cfg.DB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	tableName := cfg.TableName
	if tableName == "" {
		tableName = "covenant_storage"
	}

	// Validate table name to prevent SQL injection
	if !validTableName.MatchString(tableName) {
		return nil, fmt.Errorf("invalid table name: must contain only letters, digits, and underscores")
	}

	backend := &PostgresBackend{
		db:        cfg.DB,
		tableName: tableName,
		queries:   buildPostgresQueries(tableName),
	}

	// Create table if not exists
	if err := backend.ensureTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return backend, nil
}

// ensureTable creates the storage table if it doesn't exist.
func (p *PostgresBackend) ensureTable(ctx context.Context) error {
	_, err := p.db.ExecContext(ctx, p.queries.createTable)
	return err
}

// Save stores data at the given key.
func (p *PostgresBackend) Save(ctx context.Context, key string, data []byte) error {
	_, err := p.db.ExecContext(ctx, p.queries.save, key, data)
	if err != nil {
		return fmt.Errorf("failed to save: %w", err)
	}
	return nil
}

// Load retrieves data from the given key.
func (p *PostgresBackend) Load(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	err := p.db.QueryRowContext(ctx, p.queries.load, key).Scan(&data)
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
	// Convert prefix to LIKE pattern
	pattern := prefix + "%"

	rows, err := p.db.QueryContext(ctx, p.queries.list, pattern)
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
	result, err := p.db.ExecContext(ctx, p.queries.delete, key)
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
	var exists int
	err := p.db.QueryRowContext(ctx, p.queries.exists, key).Scan(&exists)
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
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(); rbErr != nil {
				// Log rollback error but don't override the original error
				_ = rbErr
			}
		}
	}()

	for _, op := range ops {
		var query string
		var args []any

		switch op.Type {
		case OpSave:
			query = p.queries.txSave
			args = []any{op.Key, op.Data}
		case OpDelete:
			query = p.queries.txDelete
			args = []any{op.Key}
		}

		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("failed to execute operation on %s: %w", op.Key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	committed = true

	return nil
}

// KeyValue represents a key-value pair.
type KeyValue struct {
	Key   string
	Value []byte
}

// CreateIndexes creates indexes for better query performance.
func (p *PostgresBackend) CreateIndexes(ctx context.Context) error {
	indexes := []string{p.queries.indexKeyPrefix, p.queries.indexUpdated}
	for _, idx := range indexes {
		if _, err := p.db.ExecContext(ctx, idx); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	return nil
}

// Vacuum runs VACUUM on the table for maintenance.
func (p *PostgresBackend) Vacuum(ctx context.Context) error {
	// VACUUM cannot run in a transaction
	_, err := p.db.ExecContext(ctx, p.queries.vacuum)
	return err
}

// Stats returns storage statistics.
func (p *PostgresBackend) Stats(ctx context.Context) (*Stats, error) {
	var stats Stats
	err := p.db.QueryRowContext(ctx, p.queries.stats).Scan(&stats.KeyCount, &stats.TotalSize, &stats.AvgValueSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return &stats, nil
}

// Stats contains storage statistics.
type Stats struct {
	KeyCount     int64
	TotalSize    int64
	AvgValueSize float64
}
