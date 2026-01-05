package storage

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func newTestS3Backend(t *testing.T) *S3Backend {
	t.Helper()
	client := NewMockS3Client()
	backend, err := NewS3Backend(S3Config{
		Client: client,
		Bucket: "test-bucket",
		Prefix: "contracts",
	})
	if err != nil {
		t.Fatalf("failed to create S3 backend: %v", err)
	}
	return backend
}

func TestNewS3Backend(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		client := NewMockS3Client()
		backend, err := NewS3Backend(S3Config{
			Client: client,
			Bucket: "my-bucket",
			Prefix: "prefix",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if backend == nil {
			t.Fatal("expected backend, got nil")
		}
	})

	t.Run("missing client", func(t *testing.T) {
		_, err := NewS3Backend(S3Config{
			Bucket: "my-bucket",
		})
		if err == nil {
			t.Fatal("expected error for missing client")
		}
	})

	t.Run("missing bucket", func(t *testing.T) {
		_, err := NewS3Backend(S3Config{
			Client: NewMockS3Client(),
		})
		if err == nil {
			t.Fatal("expected error for missing bucket")
		}
	})

	t.Run("empty prefix is valid", func(t *testing.T) {
		backend, err := NewS3Backend(S3Config{
			Client: NewMockS3Client(),
			Bucket: "my-bucket",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if backend == nil {
			t.Fatal("expected backend, got nil")
		}
	})
}

func TestS3Backend_SaveAndLoad(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key := "/contracts/test.json"
	data := []byte(`{"test": "data"}`)

	// Save
	if err := backend.Save(ctx, key, data); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := backend.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !bytes.Equal(loaded, data) {
		t.Errorf("loaded data = %q, want %q", loaded, data)
	}
}

func TestS3Backend_LoadNotFound(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	_, err := backend.Load(ctx, "/nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestS3Backend_Delete(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key := "/contracts/delete-me.json"
	data := []byte("delete me")

	// Save first
	if err := backend.Save(ctx, key, data); err != nil {
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
	err = backend.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify gone
	exists, err = backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key to not exist after delete")
	}
}

func TestS3Backend_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	err := backend.Delete(ctx, "/nonexistent")
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestS3Backend_Exists(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key := "/contracts/exists-test.json"

	// Should not exist initially
	exists, err := backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key to not exist initially")
	}

	// Save
	err = backend.Save(ctx, key, []byte("test"))
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Should exist now
	exists, err = backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key to exist after save")
	}
}

func TestS3Backend_List(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	// Save multiple keys
	keys := []string{
		"/contracts/a/1.json",
		"/contracts/a/2.json",
		"/contracts/b/1.json",
	}

	for _, key := range keys {
		if err := backend.Save(ctx, key, []byte(key)); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	t.Run("list all", func(t *testing.T) {
		listed, err := backend.List(ctx, "/contracts/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(listed) != 3 {
			t.Errorf("got %d keys, want 3", len(listed))
		}
	})

	t.Run("list with prefix", func(t *testing.T) {
		listed, err := backend.List(ctx, "/contracts/a/")
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(listed) != 2 {
			t.Errorf("got %d keys, want 2", len(listed))
		}
	})
}

func TestS3Backend_Transaction(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key1 := "/contracts/tx1.json"
	key2 := "/contracts/tx2.json"
	data1 := []byte("data1")
	data2 := []byte("data2")

	// Execute transaction
	ops := []Operation{
		{Type: OpSave, Key: key1, Data: data1},
		{Type: OpSave, Key: key2, Data: data2},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify both saved
	for _, key := range []string{key1, key2} {
		exists, err := backend.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if !exists {
			t.Errorf("expected %s to exist after transaction", key)
		}
	}
}

func TestS3Backend_TransactionWithDelete(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key := "/contracts/delete-in-tx.json"

	// Save first
	if err := backend.Save(ctx, key, []byte("data")); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete in transaction
	ops := []Operation{
		{Type: OpDelete, Key: key},
	}

	if err := backend.Transaction(ctx, ops); err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify deleted
	exists, err := backend.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key to be deleted after transaction")
	}
}

func TestS3Backend_KeyConversion(t *testing.T) {
	ctx := context.Background()

	// Test with prefix
	backend, err := NewS3Backend(S3Config{
		Client: NewMockS3Client(),
		Bucket: "bucket",
		Prefix: "prefix",
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	key := "/contracts/test.json"
	data := []byte("test")

	err = backend.Save(ctx, key, data)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// List should return keys with leading slash
	listed, err := backend.List(ctx, "/contracts/")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(listed) != 1 {
		t.Fatalf("expected 1 key, got %d", len(listed))
	}

	// Key should have leading slash
	if listed[0] != key {
		t.Errorf("listed key = %q, want %q", listed[0], key)
	}
}

func TestS3Backend_Overwrite(t *testing.T) {
	ctx := context.Background()
	backend := newTestS3Backend(t)

	key := "/contracts/overwrite.json"
	data1 := []byte("version1")
	data2 := []byte("version2")

	// Save first version
	if err := backend.Save(ctx, key, data1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Overwrite
	if err := backend.Save(ctx, key, data2); err != nil {
		t.Fatalf("Save (overwrite) failed: %v", err)
	}

	// Load should return second version
	loaded, err := backend.Load(ctx, key)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !bytes.Equal(loaded, data2) {
		t.Errorf("loaded = %q, want %q", loaded, data2)
	}
}

func TestMockS3Client(t *testing.T) {
	ctx := context.Background()
	client := NewMockS3Client()

	// Test basic operations
	bucket := "test-bucket"
	key := "test-key"
	data := []byte("test data")

	// PutObject
	if err := client.PutObject(ctx, bucket, key, testReader(data), int64(len(data))); err != nil {
		t.Fatalf("PutObject failed: %v", err)
	}

	// HeadObject
	exists, err := client.HeadObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}
	if !exists {
		t.Error("expected object to exist")
	}

	// GetObject
	reader, err := client.GetObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("GetObject failed: %v", err)
	}
	defer reader.Close()

	// ListObjects
	keys, err := client.ListObjects(ctx, bucket, "test")
	if err != nil {
		t.Fatalf("ListObjects failed: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("got %d keys, want 1", len(keys))
	}

	// DeleteObject
	err = client.DeleteObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("DeleteObject failed: %v", err)
	}

	// Should not exist after delete
	exists, err = client.HeadObject(ctx, bucket, key)
	if err != nil {
		t.Fatalf("HeadObject failed: %v", err)
	}
	if exists {
		t.Error("expected object to not exist after delete")
	}
}

// testReader creates a bytes.Reader for testing
func testReader(data []byte) *testBytesReader {
	return &testBytesReader{data: data, pos: 0}
}

type testBytesReader struct {
	data []byte
	pos  int
}

func (r *testBytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
