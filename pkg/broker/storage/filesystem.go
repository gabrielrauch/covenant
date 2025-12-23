package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gabrielrauch/covenant/pkg/pathutil"
)

// FilesystemBackend implements the Backend interface using the local filesystem.
type FilesystemBackend struct {
	rootDir string
	mu      sync.RWMutex
}

// NewFilesystemBackend creates a new filesystem storage backend.
func NewFilesystemBackend(rootDir string) (*FilesystemBackend, error) {
	// Ensure root directory exists
	if err := os.MkdirAll(rootDir, 0750); err != nil {
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

	path, err := fs.keyToPath(key)
	if err != nil {
		return err
	}

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to temp file first for atomicity
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
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

	path, err := fs.keyToPath(key)
	if err != nil {
		return nil, err
	}

	data, err := pathutil.ReadValidated(path)
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

	basePath, err := fs.keyToPath(prefix)
	if err != nil {
		return nil, err
	}

	var keys []string

	// Check if basePath exists
	if _, statErr := os.Stat(basePath); os.IsNotExist(statErr) {
		return keys, nil
	}

	err = filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
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

	path, err := fs.keyToPath(key)
	if err != nil {
		return err
	}

	err = os.Remove(path)
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

	path, err := fs.keyToPath(key)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check file: %w", err)
	}

	return true, nil
}

// pendingSave represents a file to be committed during transaction.
type pendingSave struct {
	tempPath  string
	finalPath string
}

// Transaction executes multiple operations atomically.
func (fs *FilesystemBackend) Transaction(ctx context.Context, ops []Operation) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.validateTransactionKeys(ops); err != nil {
		return err
	}

	pendingSaves, err := fs.prepareSaveOperations(ops)
	if err != nil {
		return err
	}

	if err := fs.commitSaveOperations(pendingSaves); err != nil {
		return err
	}

	return fs.executeDeleteOperations(ops)
}

// validateTransactionKeys validates all keys before making changes.
func (fs *FilesystemBackend) validateTransactionKeys(ops []Operation) error {
	for _, op := range ops {
		if _, err := fs.keyToPath(op.Key); err != nil {
			return err
		}
	}
	return nil
}

// prepareSaveOperations writes data to temp files for atomic commit.
func (fs *FilesystemBackend) prepareSaveOperations(ops []Operation) ([]pendingSave, error) {
	pendingSaves := make([]pendingSave, 0, len(ops))

	for _, op := range ops {
		if op.Type != OpSave {
			continue
		}

		ps, err := fs.prepareSingleSave(op)
		if err != nil {
			fs.cleanupTempFiles(pendingSaves)
			return nil, err
		}
		pendingSaves = append(pendingSaves, ps)
	}
	return pendingSaves, nil
}

// prepareSingleSave prepares a single save operation.
func (fs *FilesystemBackend) prepareSingleSave(op Operation) (pendingSave, error) {
	path, err := fs.keyToPath(op.Key)
	if err != nil {
		return pendingSave{}, fmt.Errorf("invalid key %s: %w", op.Key, err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return pendingSave{}, fmt.Errorf("failed to create directory for %s: %w", op.Key, err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, op.Data, 0600); err != nil {
		return pendingSave{}, fmt.Errorf("failed to write temp file for %s: %w", op.Key, err)
	}

	return pendingSave{tempPath: tempPath, finalPath: path}, nil
}

// commitSaveOperations commits all pending saves by renaming temp files.
func (fs *FilesystemBackend) commitSaveOperations(pendingSaves []pendingSave) error {
	for _, ps := range pendingSaves {
		if err := os.Rename(ps.tempPath, ps.finalPath); err != nil {
			return fmt.Errorf("failed to commit save: %w", err)
		}
	}
	return nil
}

// executeDeleteOperations executes all delete operations.
func (fs *FilesystemBackend) executeDeleteOperations(ops []Operation) error {
	for _, op := range ops {
		if op.Type != OpDelete {
			continue
		}
		path, err := fs.keyToPath(op.Key)
		if err != nil {
			return fmt.Errorf("invalid key %s: %w", op.Key, err)
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete %s: %w", op.Key, err)
		}
		fs.cleanEmptyDirs(filepath.Dir(path))
	}
	return nil
}

// cleanupTempFiles removes temp files on error (best effort).
func (fs *FilesystemBackend) cleanupTempFiles(pendingSaves []pendingSave) {
	for _, ps := range pendingSaves {
		_ = os.Remove(ps.tempPath)
	}
}

// validateKey checks if a key is safe to use (no path traversal attempts).
func validateKey(key string) error {
	// Reject keys containing path traversal sequences
	if strings.Contains(key, "..") {
		return ErrInvalidKey
	}
	return nil
}

// keyToPath converts a storage key to a filesystem path.
// Returns an error if the key contains path traversal attempts or
// resolves to a path outside the root directory.
func (fs *FilesystemBackend) keyToPath(key string) (string, error) {
	if err := validateKey(key); err != nil {
		return "", err
	}

	// Remove leading slash and convert to filesystem path
	cleanKey := strings.TrimPrefix(key, "/")
	resolvedPath := filepath.Join(fs.rootDir, filepath.FromSlash(cleanKey))

	// Verify the resolved path is within rootDir using pathutil
	validPath, err := pathutil.ValidateWithinRoot(fs.rootDir, resolvedPath)
	if err != nil {
		return "", ErrInvalidKey
	}

	return validPath, nil
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
