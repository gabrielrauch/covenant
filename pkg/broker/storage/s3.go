package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// S3Backend implements the Backend interface using S3-compatible storage.
// This implementation uses an S3Client interface for flexibility.
type S3Backend struct {
	client S3Client
	bucket string
	prefix string
	mu     sync.RWMutex
}

// S3Client defines the interface for S3 operations.
// This allows using AWS SDK, MinIO, or any S3-compatible client.
type S3Client interface {
	// PutObject uploads an object to S3.
	PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64) error

	// GetObject downloads an object from S3.
	GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// DeleteObject deletes an object from S3.
	DeleteObject(ctx context.Context, bucket, key string) error

	// ListObjects lists objects with the given prefix.
	ListObjects(ctx context.Context, bucket, prefix string) ([]string, error)

	// HeadObject checks if an object exists.
	HeadObject(ctx context.Context, bucket, key string) (bool, error)
}

// S3Config configures the S3 backend.
type S3Config struct {
	Client S3Client
	Bucket string
	Prefix string
}

// NewS3Backend creates a new S3 storage backend.
func NewS3Backend(cfg S3Config) (*S3Backend, error) {
	if cfg.Client == nil {
		return nil, fmt.Errorf("S3 client is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("bucket name is required")
	}

	return &S3Backend{
		client: cfg.Client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// Save stores data at the given key.
func (s *S3Backend) Save(ctx context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s3Key := s.toS3Key(key)
	return s.client.PutObject(ctx, s.bucket, s3Key, bytes.NewReader(data), int64(len(data)))
}

// Load retrieves data from the given key.
func (s *S3Backend) Load(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s3Key := s.toS3Key(key)
	reader, err := s.client.GetObject(ctx, s.bucket, s3Key)
	if err != nil {
		// Check if it's a not found error
		if isNotFoundError(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// List returns all keys matching the prefix.
func (s *S3Backend) List(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s3Prefix := s.toS3Key(prefix)
	objects, err := s.client.ListObjects(ctx, s.bucket, s3Prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	// Convert S3 keys back to storage keys
	keys := make([]string, len(objects))
	for i, obj := range objects {
		keys[i] = s.fromS3Key(obj)
	}

	return keys, nil
}

// Delete removes the data at the given key.
func (s *S3Backend) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s3Key := s.toS3Key(key)

	// Check if exists first
	exists, err := s.client.HeadObject(ctx, s.bucket, s3Key)
	if err != nil {
		return fmt.Errorf("failed to check object: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	return s.client.DeleteObject(ctx, s.bucket, s3Key)
}

// Exists checks if a key exists.
func (s *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s3Key := s.toS3Key(key)
	return s.client.HeadObject(ctx, s.bucket, s3Key)
}

// Transaction executes multiple operations.
// Note: S3 doesn't support true transactions, so this is best-effort.
func (s *S3Backend) Transaction(ctx context.Context, ops []Operation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Execute all operations
	// Note: This is not truly atomic - failures leave partial state
	for _, op := range ops {
		s3Key := s.toS3Key(op.Key)

		switch op.Type {
		case OpSave:
			if err := s.client.PutObject(ctx, s.bucket, s3Key, bytes.NewReader(op.Data), int64(len(op.Data))); err != nil {
				return fmt.Errorf("failed to save %s: %w", op.Key, err)
			}
		case OpDelete:
			if err := s.client.DeleteObject(ctx, s.bucket, s3Key); err != nil {
				// Ignore not found errors for delete
				if !isNotFoundError(err) {
					return fmt.Errorf("failed to delete %s: %w", op.Key, err)
				}
			}
		}
	}

	return nil
}

// toS3Key converts a storage key to an S3 key.
func (s *S3Backend) toS3Key(key string) string {
	// Remove leading slash and add prefix
	cleanKey := strings.TrimPrefix(key, "/")
	if s.prefix != "" {
		return s.prefix + "/" + cleanKey
	}
	return cleanKey
}

// fromS3Key converts an S3 key back to a storage key.
func (s *S3Backend) fromS3Key(s3Key string) string {
	// Remove prefix and add leading slash
	key := s3Key
	if s.prefix != "" {
		key = strings.TrimPrefix(key, s.prefix+"/")
	}
	return "/" + key
}

// isNotFoundError checks if an error indicates a missing object.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "NotFound") ||
		strings.Contains(errStr, "NoSuchKey") ||
		strings.Contains(errStr, "not found")
}

// MockS3Client is an in-memory S3 client for testing.
type MockS3Client struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMockS3Client creates a new mock S3 client.
func NewMockS3Client() *MockS3Client {
	return &MockS3Client{
		objects: make(map[string][]byte),
	}
}

// PutObject stores an object.
func (c *MockS3Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	fullKey := bucket + "/" + key
	c.objects[fullKey] = data
	return nil
}

// GetObject retrieves an object.
func (c *MockS3Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fullKey := bucket + "/" + key
	data, ok := c.objects[fullKey]
	if !ok {
		return nil, fmt.Errorf("NoSuchKey: %s", key)
	}

	return io.NopCloser(bytes.NewReader(data)), nil
}

// DeleteObject deletes an object.
func (c *MockS3Client) DeleteObject(ctx context.Context, bucket, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	fullKey := bucket + "/" + key
	delete(c.objects, fullKey)
	return nil
}

// ListObjects lists objects with prefix.
func (c *MockS3Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fullPrefix := bucket + "/" + prefix
	var keys []string

	for key := range c.objects {
		if strings.HasPrefix(key, fullPrefix) {
			// Return key without bucket prefix
			keys = append(keys, strings.TrimPrefix(key, bucket+"/"))
		}
	}

	return keys, nil
}

// HeadObject checks if an object exists.
func (c *MockS3Client) HeadObject(ctx context.Context, bucket, key string) (bool, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fullKey := bucket + "/" + key
	_, ok := c.objects[fullKey]
	return ok, nil
}
