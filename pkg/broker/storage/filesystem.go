package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FilesystemBackend implements the Backend interface using the local filesystem.
type FilesystemBackend struct {
	rootDir string
	mu      sync.RWMutex
}

// NewFilesystemBackend creates a new filesystem storage backend.
func NewFilesystemBackend(rootDir string) (*FilesystemBackend, error) {
	// Ensure root directory exists
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create root directory: %w", err)
	}

	return &FilesystemBackend{
		rootDir: rootDir,
	}, nil
}

// Save stores data at the given key.
func (fs *FilesystemBackend) Save(ctx context.Context, key string, data []byte) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first for atomicity
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Rename for atomic update
	if err := os.Rename(tempPath, path); err != nil {
		os.Remove(tempPath) // Cleanup on failure
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// Load retrieves data from the given key.
func (fs *FilesystemBackend) Load(ctx context.Context, key string) ([]byte, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// List returns all keys matching the prefix.
func (fs *FilesystemBackend) List(ctx context.Context, prefix string) ([]string, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	basePath := fs.keyToPath(prefix)
	var keys []string

	// Check if basePath exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		return keys, nil
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Convert path back to key
			relPath, err := filepath.Rel(fs.rootDir, path)
			if err != nil {
				return err
			}
			key := "/" + filepath.ToSlash(relPath)
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return keys, nil
}

// Delete removes the data at the given key.
func (fs *FilesystemBackend) Delete(ctx context.Context, key string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	path := fs.keyToPath(key)
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Clean up empty parent directories
	fs.cleanEmptyDirs(filepath.Dir(path))

	return nil
}

// Exists checks if a key exists.
func (fs *FilesystemBackend) Exists(ctx context.Context, key string) (bool, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	path := fs.keyToPath(key)
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

// Transaction executes multiple operations atomically.
func (fs *FilesystemBackend) Transaction(ctx context.Context, ops []Operation) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// For filesystem, we'll do a best-effort transaction
	// Create temp files for saves, then commit all at once
	type pendingSave struct {
		tempPath string
		finalPath string
	}
	var pendingSaves []pendingSave

	// First pass: write to temp files
	for _, op := range ops {
		if op.Type == OpSave {
			path := fs.keyToPath(op.Key)
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				// Cleanup and return
				for _, ps := range pendingSaves {
					os.Remove(ps.tempPath)
				}
				return fmt.Errorf("failed to create directory for %s: %w", op.Key, err)
			}

			tempPath := path + ".tmp"
			if err := os.WriteFile(tempPath, op.Data, 0644); err != nil {
				for _, ps := range pendingSaves {
					os.Remove(ps.tempPath)
				}
				return fmt.Errorf("failed to write temp file for %s: %w", op.Key, err)
			}
			pendingSaves = append(pendingSaves, pendingSave{tempPath: tempPath, finalPath: path})
		}
	}

	// Second pass: commit saves by renaming
	for _, ps := range pendingSaves {
		if err := os.Rename(ps.tempPath, ps.finalPath); err != nil {
			// Note: at this point we're in an inconsistent state
			// In production, we'd need more sophisticated rollback
			return fmt.Errorf("failed to commit save: %w", err)
		}
	}

	// Third pass: execute deletes
	for _, op := range ops {
		if op.Type == OpDelete {
			path := fs.keyToPath(op.Key)
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to delete %s: %w", op.Key, err)
			}
			fs.cleanEmptyDirs(filepath.Dir(path))
		}
	}

	return nil
}

// keyToPath converts a storage key to a filesystem path.
func (fs *FilesystemBackend) keyToPath(key string) string {
	// Remove leading slash and convert to filesystem path
	cleanKey := strings.TrimPrefix(key, "/")
	return filepath.Join(fs.rootDir, filepath.FromSlash(cleanKey))
}

// cleanEmptyDirs removes empty parent directories up to rootDir.
func (fs *FilesystemBackend) cleanEmptyDirs(dir string) {
	for dir != fs.rootDir && dir != "." && dir != "/" {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			break
		}
		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}
}