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

// ---------------------------------------------------------------------------
// Additional tests for uncovered paths
// ---------------------------------------------------------------------------

// setupBackend is a test helper that creates a FilesystemBackend in a temp dir.
func setupBackend(t *testing.T) (backend *FilesystemBackend, dir string) {
	t.Helper()
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	return backend, tmpDir
}

// saveKey is a test helper that saves data under the given key.
func saveKey(t *testing.T, backend *FilesystemBackend, key string, data []byte) {
	t.Helper()
	if err := backend.Save(context.Background(), key, data); err != nil {
		t.Fatalf("Save(%q) failed: %v", key, err)
	}
}

// TestTransaction_SaveFailureRollback verifies that when a save operation in a
// transaction fails (e.g., because the parent directory cannot be created),
// previously written temp files are cleaned up.
func TestTransaction_SaveFailureRollback(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	// Pre-create the directory for the first key so it succeeds
	firstKeyDir := filepath.Join(tmpDir, "tx-rollback")
	if err := os.MkdirAll(firstKeyDir, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Place a regular file where the second key's parent directory would need
	// to be created, causing MkdirAll to fail.
	blocker := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blocker, []byte("I am a file"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	ops := []Operation{
		{Type: OpSave, Key: "/tx-rollback/good.json", Data: []byte(`{"ok":true}`)},
		// "blocked" is a file, so creating "blocked/nested" as a directory fails
		{Type: OpSave, Key: "/blocked/nested/bad.json", Data: []byte(`{"ok":false}`)},
	}

	err := backend.Transaction(ctx, ops)
	if err == nil {
		t.Fatal("expected transaction to fail, but it succeeded")
	}

	// The first key's temp file should have been cleaned up by cleanupTempFiles
	tmpFile := filepath.Join(firstKeyDir, "good.json.tmp")
	if _, statErr := os.Stat(tmpFile); !os.IsNotExist(statErr) {
		t.Errorf("expected temp file %q to be cleaned up after rollback, stat err: %v", tmpFile, statErr)
	}

	// The first key's final file should not exist either
	finalFile := filepath.Join(firstKeyDir, "good.json")
	if _, statErr := os.Stat(finalFile); !os.IsNotExist(statErr) {
		t.Errorf("expected final file %q to not exist after rollback, stat err: %v", finalFile, statErr)
	}
}

// TestTransaction_DeleteNonExistent verifies that deleting a non-existent key
// inside a transaction does not return an error (the delete is silently ignored).
func TestTransaction_DeleteNonExistent(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	ops := []Operation{
		{Type: OpDelete, Key: "/does-not-exist.json"},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction with delete of non-existent key should succeed, got: %v", err)
	}
}

// TestTransaction_DeleteOnlyOps tests a transaction containing only delete operations.
func TestTransaction_DeleteOnlyOps(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	// Create files to delete
	saveKey(t, backend, "/del-only/a.json", []byte(`{"a":1}`))
	saveKey(t, backend, "/del-only/b.json", []byte(`{"b":2}`))

	ops := []Operation{
		{Type: OpDelete, Key: "/del-only/a.json"},
		{Type: OpDelete, Key: "/del-only/b.json"},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	for _, key := range []string{"/del-only/a.json", "/del-only/b.json"} {
		exists, err := backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists(%q): %v", key, err)
		}
		if exists {
			t.Errorf("expected %q to be deleted", key)
		}
	}
}

// TestTransaction_SaveOnlyOps tests a transaction containing only save operations.
func TestTransaction_SaveOnlyOps(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	ops := []Operation{
		{Type: OpSave, Key: "/save-only/x.json", Data: []byte(`{"x":1}`)},
		{Type: OpSave, Key: "/save-only/y.json", Data: []byte(`{"y":2}`)},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	for _, op := range ops {
		loaded, err := backend.Load(ctx, op.Key)
		if err != nil {
			t.Fatalf("Load(%q): %v", op.Key, err)
		}
		if !bytes.Equal(loaded, op.Data) {
			t.Errorf("data mismatch for %q: got %q, want %q", op.Key, loaded, op.Data)
		}
	}
}

// TestTransaction_EmptyOps tests that an empty transaction succeeds.
func TestTransaction_EmptyOps(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	if err := backend.Transaction(ctx, nil); err != nil {
		t.Fatalf("empty transaction should succeed, got: %v", err)
	}

	if err := backend.Transaction(ctx, []Operation{}); err != nil {
		t.Fatalf("empty slice transaction should succeed, got: %v", err)
	}
}

// TestTransaction_InvalidKeyInValidation verifies that a transaction with an
// invalid key is rejected before any operations begin.
func TestTransaction_InvalidKeyInValidation(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	tests := []struct {
		name string
		ops  []Operation
	}{
		{
			name: "invalid save key",
			ops: []Operation{
				{Type: OpSave, Key: "/valid/ok.json", Data: []byte(`{}`)},
				{Type: OpSave, Key: "/../../etc/shadow", Data: []byte(`bad`)},
			},
		},
		{
			name: "invalid delete key",
			ops: []Operation{
				{Type: OpDelete, Key: "/valid/ok.json"},
				{Type: OpDelete, Key: "/../../../etc/passwd"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := backend.Transaction(ctx, tt.ops)
			if err != ErrInvalidKey {
				t.Errorf("expected ErrInvalidKey, got: %v", err)
			}

			// Nothing should have been created
			exists, existsErr := backend.Exists(ctx, "/valid/ok.json")
			if existsErr != nil {
				t.Fatalf("Exists: %v", existsErr)
			}
			if exists {
				t.Error("no files should be created when validation fails")
			}
		})
	}
}

// TestList_NestedDirsNoFiles verifies List against a directory tree that has
// nested sub-directories but no regular files underneath the prefix.
func TestList_NestedDirsNoFiles(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	// Create nested directories with no files
	nested := filepath.Join(tmpDir, "empty-tree", "a", "b", "c")
	if err := os.MkdirAll(nested, 0750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	listed, err := backend.List(ctx, "/empty-tree")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("expected 0 keys for dirs-only tree, got %d: %v", len(listed), listed)
	}
}

// TestList_NestedDirsWithMixedContent verifies List returns only files under
// the requested prefix even when sibling directories contain other files.
func TestList_NestedDirsWithMixedContent(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	// Create a tree with files at various depths
	saveKey(t, backend, "/ns/a/deep/file1.json", []byte(`{}`))
	saveKey(t, backend, "/ns/a/deep/deeper/file2.json", []byte(`{}`))
	saveKey(t, backend, "/ns/b/file3.json", []byte(`{}`))
	saveKey(t, backend, "/other/file4.json", []byte(`{}`))

	tests := []struct {
		name    string
		prefix  string
		wantLen int
	}{
		{"deep subtree", "/ns/a", 2},
		{"single file subtree", "/ns/b", 1},
		{"entire ns", "/ns", 3},
		{"other prefix", "/other", 1},
		{"root prefix", "/", 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			listed, err := backend.List(ctx, tt.prefix)
			if err != nil {
				t.Fatalf("List(%q): %v", tt.prefix, err)
			}
			if len(listed) != tt.wantLen {
				t.Errorf("List(%q) returned %d keys, want %d: %v", tt.prefix, len(listed), tt.wantLen, listed)
			}
		})
	}
}

// TestCleanEmptyDirs_PartialCleanup verifies that cleanEmptyDirs stops when it
// encounters a non-empty parent directory (sibling files prevent removal).
func TestCleanEmptyDirs_PartialCleanup(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	// Create two files sharing a common ancestor
	saveKey(t, backend, "/shared/branch-a/leaf.json", []byte(`{}`))
	saveKey(t, backend, "/shared/branch-b/leaf.json", []byte(`{}`))

	// Delete one branch
	if err := backend.Delete(ctx, "/shared/branch-a/leaf.json"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// branch-a directory should be removed
	branchA := filepath.Join(tmpDir, "shared", "branch-a")
	if _, err := os.Stat(branchA); !os.IsNotExist(err) {
		t.Errorf("expected branch-a dir to be cleaned up")
	}

	// shared directory should still exist (branch-b is still there)
	sharedDir := filepath.Join(tmpDir, "shared")
	if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
		t.Error("shared dir should still exist because branch-b is not empty")
	}

	// branch-b should be intact
	exists, err := backend.Exists(ctx, "/shared/branch-b/leaf.json")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("sibling file should still exist")
	}
}

// TestCleanEmptyDirs_StopsAtRoot verifies that cleanEmptyDirs never removes
// the root directory itself.
func TestCleanEmptyDirs_StopsAtRoot(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	// Save and delete so cleanup walks up to root
	saveKey(t, backend, "/only-child/file.json", []byte(`{}`))
	if err := backend.Delete(ctx, "/only-child/file.json"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// The root directory must still exist
	info, err := os.Stat(tmpDir)
	if err != nil {
		t.Fatalf("root dir should still exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("root should be a directory")
	}
}

// TestCleanEmptyDirs_MultipleNestedLevels checks cleanup through many levels.
func TestCleanEmptyDirs_MultipleNestedLevels(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	key := "/l1/l2/l3/l4/l5/file.json"
	saveKey(t, backend, key, []byte(`{}`))

	// Verify all levels exist
	for _, sub := range []string{"l1", "l1/l2", "l1/l2/l3", "l1/l2/l3/l4", "l1/l2/l3/l4/l5"} {
		dir := filepath.Join(tmpDir, sub)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Fatalf("expected %q to exist before delete", dir)
		}
	}

	if err := backend.Delete(ctx, key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// All intermediate dirs should be cleaned
	l1 := filepath.Join(tmpDir, "l1")
	if _, err := os.Stat(l1); !os.IsNotExist(err) {
		t.Error("expected all empty ancestor dirs to be removed")
	}

	// Root should remain
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("root dir should still exist")
	}
}

// TestNewFilesystemBackend_InvalidRootDir checks that NewFilesystemBackend
// returns an error when it cannot create the root directory.
func TestNewFilesystemBackend_InvalidRootDir(t *testing.T) {
	// Use a path under a file (not a directory) so MkdirAll fails
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := NewFilesystemBackend(filepath.Join(blocker, "subdir"))
	if err == nil {
		t.Fatal("expected error when root dir cannot be created")
	}
}

// TestFilesystemBackend_SaveToReadOnlyDir tests Save when the parent directory
// creation fails because the target is inside a read-only path.
func TestFilesystemBackend_SaveToReadOnlyDir(t *testing.T) {
	tmpDir := t.TempDir()
	backend, err := NewFilesystemBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewFilesystemBackend: %v", err)
	}

	// Make root read-only so MkdirAll for sub-paths fails
	if chmodErr := os.Chmod(tmpDir, 0500); chmodErr != nil {
		t.Fatalf("Chmod: %v", chmodErr)
	}
	// Restore permissions for cleanup
	t.Cleanup(func() {
		if chmodErr := os.Chmod(tmpDir, 0750); chmodErr != nil {
			t.Logf("warning: failed to restore permissions: %v", chmodErr)
		}
	})

	ctx := context.Background()
	err = backend.Save(ctx, "/readonly/sub/file.json", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error when saving to read-only directory")
	}
}

// TestFilesystemBackend_ExistsOnExistingKey confirms Exists returns true for
// a key that was previously saved.
func TestFilesystemBackend_ExistsOnExistingKey(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	key := "/exists/test.json"
	saveKey(t, backend, key, []byte(`{}`))

	exists, err := backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Error("expected Exists to return true for saved key")
	}
}

// TestFilesystemBackend_ExistsNotFound confirms Exists returns false for a key
// that does not exist without returning an error.
func TestFilesystemBackend_ExistsNotFound(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	exists, err := backend.Exists(ctx, "/no-such-key.json")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Error("expected Exists to return false for non-existent key")
	}
}

// TestFilesystemBackend_ValidateKey tests the standalone validateKey function
// with various inputs.
func TestFilesystemBackend_ValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"valid simple", "/foo.json", false},
		{"valid nested", "/a/b/c.json", false},
		{"valid no leading slash", "foo/bar.json", false},
		{"invalid double dot", "../escape", true},
		{"invalid embedded double dot", "/a/../b", true},
		{"invalid trailing double dot", "/a/b/..", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateKey(tt.key)
			if tt.wantErr && err == nil {
				t.Errorf("validateKey(%q) should have returned error", tt.key)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateKey(%q) unexpected error: %v", tt.key, err)
			}
			if tt.wantErr && err != nil && err != ErrInvalidKey {
				t.Errorf("validateKey(%q) should return ErrInvalidKey, got: %v", tt.key, err)
			}
		})
	}
}

// TestTransaction_MixedSaveDeleteOrder verifies that saves are committed before
// deletes, allowing a save-then-delete pattern within a single transaction.
func TestTransaction_MixedSaveDeleteOrder(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	// Create a file to be replaced
	saveKey(t, backend, "/replace/old.json", []byte(`{"old":true}`))

	ops := []Operation{
		{Type: OpSave, Key: "/replace/new.json", Data: []byte(`{"new":true}`)},
		{Type: OpDelete, Key: "/replace/old.json"},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// New file should exist
	loaded, err := backend.Load(ctx, "/replace/new.json")
	if err != nil {
		t.Fatalf("Load new: %v", err)
	}
	if string(loaded) != `{"new":true}` {
		t.Errorf("unexpected data: %s", loaded)
	}

	// Old file should be gone
	exists, err := backend.Exists(ctx, "/replace/old.json")
	if err != nil {
		t.Fatalf("Exists old: %v", err)
	}
	if exists {
		t.Error("old file should have been deleted")
	}
}

// TestTransaction_DeleteCleansEmptyDirs confirms that delete operations inside
// a transaction also trigger empty directory cleanup.
func TestTransaction_DeleteCleansEmptyDirs(t *testing.T) {
	backend, tmpDir := setupBackend(t)
	ctx := context.Background()

	saveKey(t, backend, "/tx-clean/deep/nested/file.json", []byte(`{}`))

	ops := []Operation{
		{Type: OpDelete, Key: "/tx-clean/deep/nested/file.json"},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// All empty dirs up to root should be cleaned
	txCleanDir := filepath.Join(tmpDir, "tx-clean")
	if _, err := os.Stat(txCleanDir); !os.IsNotExist(err) {
		t.Errorf("expected empty dirs to be cleaned after transaction delete")
	}
}

// TestList_RootPrefix tests listing all keys from the root prefix.
func TestList_RootPrefix(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	keys := []string{
		"/a/file1.json",
		"/b/file2.json",
		"/c/d/file3.json",
	}

	for _, k := range keys {
		saveKey(t, backend, k, []byte(`{}`))
	}

	listed, err := backend.List(ctx, "/")
	if err != nil {
		t.Fatalf("List(/): %v", err)
	}

	if len(listed) != len(keys) {
		t.Errorf("List(/) returned %d keys, want %d: %v", len(listed), len(keys), listed)
	}
}

// TestList_ExactFileAsPrefix tests listing with a prefix that resolves to an
// exact file path (not a directory).
func TestList_ExactFileAsPrefix(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	saveKey(t, backend, "/exact/file.json", []byte(`{}`))

	// Using the exact file path as prefix should return just that key
	listed, err := backend.List(ctx, "/exact/file.json")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(listed) != 1 {
		t.Errorf("expected 1 key, got %d: %v", len(listed), listed)
	}
}
