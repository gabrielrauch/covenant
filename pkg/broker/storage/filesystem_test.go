package storage

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesystemBackend_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	key := "/test/data.json"
	data := []byte(`{"name":"test"}`)

	// Save
	if err = backend.Save(ctx, key, data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := backend.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !bytes.Equal(loaded, data) {
		t.Errorf("loaded data mismatch: expected %q, got %q", data, loaded)
	}
}

func TestFilesystemBackend_LoadNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	_, err = backend.Load(ctx, "/nonexistent/key.json")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFilesystemBackend_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	key := "/test/delete.json"
	data := []byte(`{"delete":"me"}`)

	// Save first
	if err = backend.Save(ctx, key, data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify exists
	exists, err := backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected key to exist after save")
	}

	// Delete
	if err = backend.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deleted
	exists, err = backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key to not exist after delete")
	}
}

func TestFilesystemBackend_DeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	err = backend.Delete(ctx, "/nonexistent/key.json")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestFilesystemBackend_List(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	// Save multiple items
	keys := []string{
		"/contracts/consumer1/provider1/1.0.0.json",
		"/contracts/consumer1/provider1/1.1.0.json",
		"/contracts/consumer2/provider1/1.0.0.json",
		"/verifications/contract1/1.0.0.json",
	}

	for _, key := range keys {
		if err = backend.Save(ctx, key, []byte(`{}`)); err != nil {
			t.Fatalf("Save failed for %s: %v", key, err)
		}
	}

	// List with prefix
	listed, err := backend.List(ctx, "/contracts/consumer1")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 2 {
		t.Errorf("expected 2 keys, got %d: %v", len(listed), listed)
	}
}

func TestFilesystemBackend_ListEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	listed, err := backend.List(ctx, "/nonexistent")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 0 {
		t.Errorf("expected empty list, got: %v", listed)
	}
}

func TestFilesystemBackend_Transaction(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	// Create a key to delete
	deleteKey := "/to-delete.json"
	if err = backend.Save(ctx, deleteKey, []byte(`{}`)); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Execute transaction
	ops := []Operation{
		{Type: OpSave, Key: "/tx/file1.json", Data: []byte(`{"file":1}`)},
		{Type: OpSave, Key: "/tx/file2.json", Data: []byte(`{"file":2}`)},
		{Type: OpDelete, Key: deleteKey},
	}

	if err = backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify saves
	for _, key := range []string{"/tx/file1.json", "/tx/file2.json"} {
		var exists bool
		exists, err = backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Errorf("expected %s to exist after transaction", key)
		}
	}

	// Verify delete
	exists, err := backend.Exists(ctx, deleteKey)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected deleted key to not exist after transaction")
	}
}

// Security tests for path traversal protection

func TestFilesystemBackend_PathTraversal_Rejected(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	// Test cases for path traversal attempts
	maliciousKeys := []string{
		"../../../etc/passwd",
		"/contracts/../../../etc/passwd",
		"/contracts/consumer/../../../etc/passwd",
		"/../secret",
		"/test/..\\..\\windows\\system32",
		"/test/foo/../../bar/../../../secret",
	}

	for _, key := range maliciousKeys {
		t.Run("Save_"+key, func(t *testing.T) {
			err := backend.Save(ctx, key, []byte("malicious"))
			if err != ErrInvalidKey {
				t.Errorf("expected ErrInvalidKey for %q, got: %v", key, err)
			}
		})

		t.Run("Load_"+key, func(t *testing.T) {
			_, err := backend.Load(ctx, key)
			if err != ErrInvalidKey {
				t.Errorf("expected ErrInvalidKey for %q, got: %v", key, err)
			}
		})

		t.Run("Delete_"+key, func(t *testing.T) {
			err := backend.Delete(ctx, key)
			if err != ErrInvalidKey {
				t.Errorf("expected ErrInvalidKey for %q, got: %v", key, err)
			}
		})

		t.Run("Exists_"+key, func(t *testing.T) {
			_, err := backend.Exists(ctx, key)
			if err != ErrInvalidKey {
				t.Errorf("expected ErrInvalidKey for %q, got: %v", key, err)
			}
		})
	}
}

func TestFilesystemBackend_PathTraversal_TransactionRejected(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	// Transaction with malicious key should fail before making any changes
	ops := []Operation{
		{Type: OpSave, Key: "/valid/key.json", Data: []byte(`{}`)},
		{Type: OpSave, Key: "/../../../etc/passwd", Data: []byte(`malicious`)},
	}

	err = backend.Transaction(ctx, ops)
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got: %v", err)
	}

	// Verify no files were created (transaction should be all-or-nothing)
	exists, err := backend.Exists(ctx, "/valid/key.json")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected no files to be created when transaction contains invalid key")
	}
}

func TestFilesystemBackend_ValidKeys(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()

	// These should all be valid
	validKeys := []string{
		"/contracts/consumer/provider/1.0.0.json",
		"/a/b/c/d/e/f.txt",
		"/file.json",
		"/contracts/my-service/api/v2.0.0.json",
		"/contracts/foo_bar/baz-qux/1.0.0.json",
	}

	for _, key := range validKeys {
		t.Run(key, func(t *testing.T) {
			data := []byte(`{"key":"` + key + `"}`)
			if err := backend.Save(ctx, key, data); err != nil {
				t.Errorf("Save failed for valid key %q: %v", key, err)
			}

			loaded, err := backend.Load(ctx, key)
			if err != nil {
				t.Errorf("Load failed for valid key %q: %v", key, err)
			}

			if !bytes.Equal(loaded, data) {
				t.Errorf("data mismatch for %q", key)
			}
		})
	}
}

func TestFilesystemBackend_OverwriteExisting(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	key := "/test/overwrite.json"

	// Save original
	if err = backend.Save(ctx, key, []byte(`{"version":1}`)); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	// Overwrite
	if err = backend.Save(ctx, key, []byte(`{"version":2}`)); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	// Verify overwritten
	loaded, err := backend.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if string(loaded) != `{"version":2}` {
		t.Errorf("expected overwritten data, got: %s", loaded)
	}
}

func TestFilesystemBackend_CleanEmptyDirs(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	ctx := context.Background()
	key := "/deep/nested/path/file.json"

	// Save creates nested directories
	if err := backend.Save(ctx, key, []byte(`{}`)); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify directory exists
	nestedDir := filepath.Join(tmpDir, "deep", "nested", "path")
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Fatal("nested directory should exist after save")
	}

	// Delete should clean up empty parent directories
	if err := backend.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify empty directories were cleaned up
	deepDir := filepath.Join(tmpDir, "deep")
	if _, err := os.Stat(deepDir); !os.IsNotExist(err) {
		t.Error("empty parent directories should be cleaned up after delete")
	}
}
