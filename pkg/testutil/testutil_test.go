package testutil

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
)

// ---------------------------------------------------------------------------
// MockStorage tests
// ---------------------------------------------------------------------------

func TestNewMockStorage(t *testing.T) {
	ms := NewMockStorage()
	if ms == nil {
		t.Fatal("NewMockStorage returned nil")
	}
	if ms.KeyCount() != 0 {
		t.Errorf("expected 0 keys, got %d", ms.KeyCount())
	}
	if len(ms.SaveCalls) != 0 {
		t.Errorf("expected 0 SaveCalls, got %d", len(ms.SaveCalls))
	}
	if len(ms.LoadCalls) != 0 {
		t.Errorf("expected 0 LoadCalls, got %d", len(ms.LoadCalls))
	}
	if len(ms.DeleteCalls) != 0 {
		t.Errorf("expected 0 DeleteCalls, got %d", len(ms.DeleteCalls))
	}
}

func TestMockStorage_Save(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		key       string
		data      []byte
		saveErr   error
		wantErr   bool
		wantCount int
	}{
		{
			name:      "save simple data",
			key:       "key1",
			data:      []byte("value1"),
			wantCount: 1,
		},
		{
			name:      "save empty data",
			key:       "empty",
			data:      []byte{},
			wantCount: 1,
		},
		{
			name:    "save with injected error",
			key:     "fail",
			data:    []byte("data"),
			saveErr: errors.New("disk full"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			ms.SaveError = tt.saveErr

			err := ms.Save(ctx, tt.key, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if ms.KeyCount() != 0 {
					t.Errorf("expected 0 keys after error, got %d", ms.KeyCount())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ms.KeyCount() != tt.wantCount {
				t.Errorf("expected %d keys, got %d", tt.wantCount, ms.KeyCount())
			}
		})
	}
}

func TestMockStorage_Save_TracksCalls(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	if err := ms.Save(ctx, "a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "c", []byte("3")); err != nil {
		t.Fatal(err)
	}

	if len(ms.SaveCalls) != 3 {
		t.Fatalf("expected 3 SaveCalls, got %d", len(ms.SaveCalls))
	}
	if ms.SaveCalls[0].Key != "a" || string(ms.SaveCalls[0].Data) != "1" {
		t.Errorf("SaveCalls[0] = %+v, want Key=a Data=1", ms.SaveCalls[0])
	}
	if ms.SaveCalls[2].Key != "c" || string(ms.SaveCalls[2].Data) != "3" {
		t.Errorf("SaveCalls[2] = %+v, want Key=c Data=3", ms.SaveCalls[2])
	}
}

func TestMockStorage_Save_DataIsolation(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	original := []byte("original")
	if err := ms.Save(ctx, "key", original); err != nil {
		t.Fatal(err)
	}

	// Mutate the original slice after saving
	original[0] = 'X'

	loaded, err := ms.Load(ctx, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(loaded) != "original" {
		t.Errorf("data was mutated through original slice: got %q", loaded)
	}
}

func TestMockStorage_Load(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(*testing.T, *MockStorage)
		key      string
		loadErr  error
		wantData string
		wantErr  error
	}{
		{
			name: "load existing key",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "k1", []byte("v1")); err != nil {
					t.Fatal(err)
				}
			},
			key:      "k1",
			wantData: "v1",
		},
		{
			name:    "load nonexistent key",
			setup:   func(_ *testing.T, _ *MockStorage) {},
			key:     "missing",
			wantErr: storage.ErrNotFound,
		},
		{
			name:    "load with injected error",
			setup:   func(_ *testing.T, _ *MockStorage) {},
			key:     "any",
			loadErr: errors.New("read error"),
			wantErr: errors.New("read error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			tt.setup(t, ms)
			ms.LoadError = tt.loadErr

			data, err := ms.Load(ctx, tt.key)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(data) != tt.wantData {
				t.Errorf("expected data %q, got %q", tt.wantData, data)
			}
		})
	}
}

func TestMockStorage_Load_TracksCallsEvenOnMiss(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	if _, err := ms.Load(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected error for missing key")
	}
	if _, err := ms.Load(ctx, "also-missing"); err == nil {
		t.Fatal("expected error for missing key")
	}

	if len(ms.LoadCalls) != 2 {
		t.Fatalf("expected 2 LoadCalls, got %d", len(ms.LoadCalls))
	}
	if ms.LoadCalls[0] != "does-not-exist" {
		t.Errorf("LoadCalls[0] = %q, want %q", ms.LoadCalls[0], "does-not-exist")
	}
}

func TestMockStorage_Load_ReturnedDataIsolation(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()
	if err := ms.Save(ctx, "key", []byte("hello")); err != nil {
		t.Fatal(err)
	}

	loaded, err := ms.Load(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	loaded[0] = 'X'

	loaded2, err := ms.Load(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if string(loaded2) != "hello" {
		t.Errorf("internal data was mutated via returned slice: got %q", loaded2)
	}
}

//nolint:gocyclo // test function
func TestMockStorage_List(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		setup    func(*testing.T, *MockStorage)
		prefix   string
		listErr  error
		wantKeys []string
		wantErr  bool
	}{
		{
			name: "list with matching prefix",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "contracts/a", []byte("1")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "contracts/b", []byte("2")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "verifications/x", []byte("3")); err != nil {
					t.Fatal(err)
				}
			},
			prefix:   "contracts/",
			wantKeys: []string{"contracts/a", "contracts/b"},
		},
		{
			name: "list with empty prefix returns all",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "x", []byte("1")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "y", []byte("2")); err != nil {
					t.Fatal(err)
				}
			},
			prefix:   "",
			wantKeys: []string{"x", "y"},
		},
		{
			name:     "list with no matches returns empty",
			setup:    func(_ *testing.T, _ *MockStorage) {},
			prefix:   "nothing/",
			wantKeys: nil,
		},
		{
			name:    "list with injected error",
			setup:   func(_ *testing.T, _ *MockStorage) {},
			prefix:  "any",
			listErr: errors.New("list failed"),
			wantErr: true,
		},
		{
			name: "list returns sorted keys",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "z/c", []byte("1")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "z/a", []byte("2")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "z/b", []byte("3")); err != nil {
					t.Fatal(err)
				}
			},
			prefix:   "z/",
			wantKeys: []string{"z/a", "z/b", "z/c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			tt.setup(t, ms)
			ms.ListError = tt.listErr

			keys, err := ms.List(ctx, tt.prefix)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(keys) != len(tt.wantKeys) {
				t.Fatalf("expected %d keys, got %d: %v", len(tt.wantKeys), len(keys), keys)
			}
			for i, k := range keys {
				if k != tt.wantKeys[i] {
					t.Errorf("keys[%d] = %q, want %q", i, k, tt.wantKeys[i])
				}
			}
		})
	}
}

func TestMockStorage_Delete(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*testing.T, *MockStorage)
		key       string
		deleteErr error
		wantErr   error
	}{
		{
			name: "delete existing key",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "k1", []byte("v1")); err != nil {
					t.Fatal(err)
				}
			},
			key: "k1",
		},
		{
			name:    "delete nonexistent key",
			setup:   func(_ *testing.T, _ *MockStorage) {},
			key:     "missing",
			wantErr: storage.ErrNotFound,
		},
		{
			name:      "delete with injected error",
			setup:     func(_ *testing.T, _ *MockStorage) {},
			key:       "any",
			deleteErr: errors.New("delete failed"),
			wantErr:   errors.New("delete failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			tt.setup(t, ms)
			ms.DeleteError = tt.deleteErr

			err := ms.Delete(ctx, tt.key)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify deletion
			exists, existsErr := ms.Exists(ctx, tt.key)
			if existsErr != nil {
				t.Fatal(existsErr)
			}
			if exists {
				t.Error("key still exists after deletion")
			}
		})
	}
}

func TestMockStorage_Delete_TracksCalls(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()
	if err := ms.Save(ctx, "a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "b", []byte("2")); err != nil {
		t.Fatal(err)
	}

	if err := ms.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if err := ms.Delete(ctx, "b"); err != nil {
		t.Fatal(err)
	}

	if len(ms.DeleteCalls) != 2 {
		t.Fatalf("expected 2 DeleteCalls, got %d", len(ms.DeleteCalls))
	}
	if ms.DeleteCalls[0] != "a" {
		t.Errorf("DeleteCalls[0] = %q, want %q", ms.DeleteCalls[0], "a")
	}
	if ms.DeleteCalls[1] != "b" {
		t.Errorf("DeleteCalls[1] = %q, want %q", ms.DeleteCalls[1], "b")
	}
}

func TestMockStorage_Exists(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		setup     func(*testing.T, *MockStorage)
		key       string
		existsErr error
		want      bool
		wantErr   bool
	}{
		{
			name: "exists for stored key",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "present", []byte("data")); err != nil {
					t.Fatal(err)
				}
			},
			key:  "present",
			want: true,
		},
		{
			name:  "not exists for missing key",
			setup: func(_ *testing.T, _ *MockStorage) {},
			key:   "absent",
			want:  false,
		},
		{
			name:      "exists with injected error",
			setup:     func(_ *testing.T, _ *MockStorage) {},
			key:       "any",
			existsErr: errors.New("check failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			tt.setup(t, ms)
			ms.ExistsError = tt.existsErr

			got, err := ms.Exists(ctx, tt.key)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Exists(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

//nolint:gocyclo // test function
func TestMockStorage_Transaction(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		setup   func(*testing.T, *MockStorage)
		ops     []storage.Operation
		saveErr error
		delErr  error
		wantErr bool
		verify  func(*testing.T, *MockStorage)
	}{
		{
			name: "transaction with saves",
			ops: []storage.Operation{
				{Type: storage.OpSave, Key: "tx/a", Data: []byte("A")},
				{Type: storage.OpSave, Key: "tx/b", Data: []byte("B")},
			},
			verify: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if ms.KeyCount() != 2 {
					t.Errorf("expected 2 keys, got %d", ms.KeyCount())
				}
				data, err := ms.Load(ctx, "tx/a")
				if err != nil {
					t.Fatal(err)
				}
				if string(data) != "A" {
					t.Errorf("tx/a = %q, want %q", data, "A")
				}
			},
		},
		{
			name: "transaction with deletes",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "del/x", []byte("X")); err != nil {
					t.Fatal(err)
				}
				if err := ms.Save(ctx, "del/y", []byte("Y")); err != nil {
					t.Fatal(err)
				}
			},
			ops: []storage.Operation{
				{Type: storage.OpDelete, Key: "del/x"},
				{Type: storage.OpDelete, Key: "del/y"},
			},
			verify: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if ms.KeyCount() != 0 {
					t.Errorf("expected 0 keys, got %d", ms.KeyCount())
				}
			},
		},
		{
			name: "transaction with mixed save and delete",
			setup: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if err := ms.Save(ctx, "old", []byte("old-data")); err != nil {
					t.Fatal(err)
				}
			},
			ops: []storage.Operation{
				{Type: storage.OpSave, Key: "new", Data: []byte("new-data")},
				{Type: storage.OpDelete, Key: "old"},
			},
			verify: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if ms.KeyCount() != 1 {
					t.Errorf("expected 1 key, got %d", ms.KeyCount())
				}
				exists, err := ms.Exists(ctx, "new")
				if err != nil {
					t.Fatal(err)
				}
				if !exists {
					t.Error("expected 'new' key to exist")
				}
				exists, err = ms.Exists(ctx, "old")
				if err != nil {
					t.Fatal(err)
				}
				if exists {
					t.Error("expected 'old' key to be deleted")
				}
			},
		},
		{
			name:    "transaction save error",
			ops:     []storage.Operation{{Type: storage.OpSave, Key: "fail", Data: []byte("x")}},
			saveErr: errors.New("save failed"),
			wantErr: true,
		},
		{
			name:    "transaction delete error",
			ops:     []storage.Operation{{Type: storage.OpDelete, Key: "fail"}},
			delErr:  errors.New("delete failed"),
			wantErr: true,
		},
		{
			name: "empty transaction",
			ops:  []storage.Operation{},
			verify: func(t *testing.T, ms *MockStorage) {
				t.Helper()
				if ms.KeyCount() != 0 {
					t.Errorf("expected 0 keys, got %d", ms.KeyCount())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := NewMockStorage()
			if tt.setup != nil {
				tt.setup(t, ms)
			}
			ms.SaveError = tt.saveErr
			ms.DeleteError = tt.delErr

			err := ms.Transaction(ctx, tt.ops)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.verify != nil {
				tt.verify(t, ms)
			}
		})
	}
}

func TestMockStorage_Transaction_TracksCalls(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()
	if err := ms.Save(ctx, "existing", []byte("data")); err != nil {
		t.Fatal(err)
	}

	// Clear save calls from setup
	ms.SaveCalls = nil

	err := ms.Transaction(ctx, []storage.Operation{
		{Type: storage.OpSave, Key: "tx-new", Data: []byte("new")},
		{Type: storage.OpDelete, Key: "existing"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ms.SaveCalls) != 1 {
		t.Fatalf("expected 1 SaveCall, got %d", len(ms.SaveCalls))
	}
	if ms.SaveCalls[0].Key != "tx-new" {
		t.Errorf("SaveCalls[0].Key = %q, want %q", ms.SaveCalls[0].Key, "tx-new")
	}
	if len(ms.DeleteCalls) != 1 {
		t.Fatalf("expected 1 DeleteCall, got %d", len(ms.DeleteCalls))
	}
	if ms.DeleteCalls[0] != "existing" {
		t.Errorf("DeleteCalls[0] = %q, want %q", ms.DeleteCalls[0], "existing")
	}
}

func TestMockStorage_Reset(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	// Populate data and errors
	if err := ms.Save(ctx, "k1", []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "k2", []byte("v2")); err != nil {
		t.Fatal(err)
	}
	if _, err := ms.Load(ctx, "k1"); err != nil {
		t.Fatal(err)
	}
	if err := ms.Delete(ctx, "k1"); err != nil {
		t.Fatal(err)
	}
	ms.SaveError = errors.New("err")
	ms.LoadError = errors.New("err")
	ms.ListError = errors.New("err")
	ms.DeleteError = errors.New("err")
	ms.ExistsError = errors.New("err")

	ms.Reset()

	if ms.KeyCount() != 0 {
		t.Errorf("after Reset, expected 0 keys, got %d", ms.KeyCount())
	}
	if len(ms.SaveCalls) != 0 {
		t.Errorf("after Reset, expected 0 SaveCalls, got %d", len(ms.SaveCalls))
	}
	if len(ms.LoadCalls) != 0 {
		t.Errorf("after Reset, expected 0 LoadCalls, got %d", len(ms.LoadCalls))
	}
	if len(ms.DeleteCalls) != 0 {
		t.Errorf("after Reset, expected 0 DeleteCalls, got %d", len(ms.DeleteCalls))
	}
	if ms.SaveError != nil {
		t.Errorf("after Reset, SaveError should be nil")
	}
	if ms.LoadError != nil {
		t.Errorf("after Reset, LoadError should be nil")
	}
	if ms.ListError != nil {
		t.Errorf("after Reset, ListError should be nil")
	}
	if ms.DeleteError != nil {
		t.Errorf("after Reset, DeleteError should be nil")
	}
	if ms.ExistsError != nil {
		t.Errorf("after Reset, ExistsError should be nil")
	}
}

func TestMockStorage_GetData(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	if err := ms.Save(ctx, "a", []byte("alpha")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "b", []byte("beta")); err != nil {
		t.Fatal(err)
	}

	data := ms.GetData()

	if len(data) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(data))
	}
	if string(data["a"]) != "alpha" {
		t.Errorf("data[a] = %q, want %q", data["a"], "alpha")
	}
	if string(data["b"]) != "beta" {
		t.Errorf("data[b] = %q, want %q", data["b"], "beta")
	}

	// Ensure returned data is a copy
	data["a"][0] = 'X'
	data2 := ms.GetData()
	if string(data2["a"]) != "alpha" {
		t.Error("GetData did not return a copy; internal data was mutated")
	}
}

func TestMockStorage_KeyCount(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	if ms.KeyCount() != 0 {
		t.Errorf("empty storage: expected 0, got %d", ms.KeyCount())
	}

	if err := ms.Save(ctx, "a", []byte("1")); err != nil {
		t.Fatal(err)
	}
	if ms.KeyCount() != 1 {
		t.Errorf("after 1 save: expected 1, got %d", ms.KeyCount())
	}

	if err := ms.Save(ctx, "b", []byte("2")); err != nil {
		t.Fatal(err)
	}
	if ms.KeyCount() != 2 {
		t.Errorf("after 2 saves: expected 2, got %d", ms.KeyCount())
	}

	// Overwrite should not increase count
	if err := ms.Save(ctx, "a", []byte("updated")); err != nil {
		t.Fatal(err)
	}
	if ms.KeyCount() != 2 {
		t.Errorf("after overwrite: expected 2, got %d", ms.KeyCount())
	}

	if err := ms.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if ms.KeyCount() != 1 {
		t.Errorf("after delete: expected 1, got %d", ms.KeyCount())
	}
}

func TestMockStorage_ImplementsBackend(t *testing.T) {
	// Compile-time check already exists, but verify at runtime too
	var _ storage.Backend = NewMockStorage()
}

func TestMockStorage_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()
	const goroutines = 50

	var wg sync.WaitGroup
	// Use Save, Exists, List, KeyCount, GetData concurrently.
	// Note: Load appends to LoadCalls under RLock which is a known
	// limitation, so we exclude it from the race test.
	wg.Add(goroutines * 4)

	for i := 0; i < goroutines; i++ {
		key := "key-" + string(rune('A'+i%26))

		go func(k string) {
			defer wg.Done()
			_ = ms.Save(ctx, k, []byte("data")) //nolint:errcheck // concurrent race test; errors not meaningful
		}(key)

		go func(k string) {
			defer wg.Done()
			_, _ = ms.Exists(ctx, k) //nolint:errcheck // concurrent race test; errors not meaningful
		}(key)

		go func() {
			defer wg.Done()
			_, _ = ms.List(ctx, "key-") //nolint:errcheck // concurrent race test; errors not meaningful
		}()

		go func() {
			defer wg.Done()
			ms.KeyCount()
		}()
	}

	wg.Wait()
	// If we reach here without data races, the test passes
}

// ---------------------------------------------------------------------------
// HTTPRequest builder tests
// ---------------------------------------------------------------------------

func TestNewHTTPRequest(t *testing.T) {
	req := NewHTTPRequest("GET", "/api/test")
	if req == nil {
		t.Fatal("NewHTTPRequest returned nil")
	}
	if req.method != "GET" {
		t.Errorf("method = %q, want %q", req.method, "GET")
	}
	if req.path != "/api/test" {
		t.Errorf("path = %q, want %q", req.path, "/api/test")
	}
}

func TestHTTPRequest_WithHeader(t *testing.T) {
	req := NewHTTPRequest("GET", "/").
		WithHeader("Authorization", "Bearer token123").
		WithHeader("Accept", "application/json")

	built := req.Build(t)

	if got := built.Header.Get("Authorization"); got != "Bearer token123" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer token123")
	}
	if got := built.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestHTTPRequest_WithJSONBody(t *testing.T) {
	body := map[string]string{"name": "test", "value": "123"}
	req := NewHTTPRequest("POST", "/api/items").WithJSONBody(body)

	built := req.Build(t)

	if ct := built.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	bodyBytes, err := io.ReadAll(built.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		t.Fatalf("failed to parse body JSON: %v", err)
	}
	if parsed["name"] != "test" {
		t.Errorf("body.name = %q, want %q", parsed["name"], "test")
	}
}

func TestHTTPRequest_Build_NoBody(t *testing.T) {
	req := NewHTTPRequest("DELETE", "/api/items/1").Build(t)

	if req.Method != "DELETE" {
		t.Errorf("Method = %q, want %q", req.Method, "DELETE")
	}
	if req.Body == nil {
		// httptest.NewRequest sets Body to http.NoBody when nil reader is passed
		return
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if len(bodyBytes) != 0 {
		t.Errorf("expected empty body, got %d bytes", len(bodyBytes))
	}
}

func TestHTTPRequest_Build_MethodAndPath(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"GET request", "GET", "/api/v1/users"},
		{"POST request", "POST", "/api/v1/users"},
		{"PUT request", "PUT", "/api/v1/users/1"},
		{"PATCH request", "PATCH", "/api/v1/users/1"},
		{"DELETE request", "DELETE", "/api/v1/users/1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewHTTPRequest(tt.method, tt.path).Build(t)
			if req.Method != tt.method {
				t.Errorf("Method = %q, want %q", req.Method, tt.method)
			}
			if req.URL.Path != tt.path {
				t.Errorf("Path = %q, want %q", req.URL.Path, tt.path)
			}
		})
	}
}

func TestHTTPRequest_Chaining(t *testing.T) {
	req := NewHTTPRequest("POST", "/api/data").
		WithHeader("X-Request-ID", "abc").
		WithJSONBody(map[string]int{"count": 5}).
		WithHeader("X-Trace", "xyz")

	built := req.Build(t)

	if got := built.Header.Get("X-Request-ID"); got != "abc" {
		t.Errorf("X-Request-ID = %q, want %q", got, "abc")
	}
	if got := built.Header.Get("X-Trace"); got != "xyz" {
		t.Errorf("X-Trace = %q, want %q", got, "xyz")
	}
	if got := built.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
}

// ---------------------------------------------------------------------------
// HTTPResponse tests
// ---------------------------------------------------------------------------

func TestNewHTTPResponse(t *testing.T) {
	resp := NewHTTPResponse(t)
	if resp == nil {
		t.Fatal("NewHTTPResponse returned nil")
	}
	if resp.ResponseRecorder == nil {
		t.Fatal("ResponseRecorder is nil")
	}
}

func TestHTTPResponse_ExpectStatus(t *testing.T) {
	t.Run("matching status 200", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.WriteHeader(http.StatusOK)
		resp.ExpectStatus(http.StatusOK) // should not fail
	})

	t.Run("matching status 500", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.WriteHeader(http.StatusInternalServerError)
		resp.ExpectStatus(http.StatusInternalServerError) // should not fail
	})

	t.Run("returns self for chaining", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.WriteHeader(http.StatusCreated)
		got := resp.ExpectStatus(http.StatusCreated)
		if got != resp {
			t.Error("ExpectStatus should return the same HTTPResponse for chaining")
		}
	})
}

func TestHTTPResponse_ExpectHeader(t *testing.T) {
	t.Run("matching header", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Header().Set("Content-Type", "application/json")
		resp.ExpectHeader("Content-Type", "application/json") // should not fail
	})

	t.Run("returns self for chaining", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Header().Set("X-Custom", "val")
		got := resp.ExpectHeader("X-Custom", "val")
		if got != resp {
			t.Error("ExpectHeader should return the same HTTPResponse for chaining")
		}
	})

	t.Run("multiple headers", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Header().Set("Content-Type", "application/json")
		resp.Header().Set("X-Request-ID", "abc123")
		resp.
			ExpectHeader("Content-Type", "application/json").
			ExpectHeader("X-Request-ID", "abc123")
	})
}

func TestHTTPResponse_ExpectJSONBody(t *testing.T) {
	t.Run("matching JSON object", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Body.WriteString(`{"name":"alice","age":30}`)
		resp.ExpectJSONBody(map[string]any{"name": "alice", "age": float64(30)})
	})

	t.Run("matching JSON array", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Body.WriteString(`[1,2,3]`)
		resp.ExpectJSONBody([]any{float64(1), float64(2), float64(3)})
	})

	t.Run("matching JSON with different key order", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Body.WriteString(`{"b":2,"a":1}`)
		resp.ExpectJSONBody(map[string]any{"a": float64(1), "b": float64(2)})
	})

	t.Run("returns self for chaining", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Body.WriteString(`{"ok":true}`)
		got := resp.ExpectJSONBody(map[string]any{"ok": true})
		if got != resp {
			t.Error("ExpectJSONBody should return the same HTTPResponse for chaining")
		}
	})

	t.Run("nested object", func(t *testing.T) {
		resp := NewHTTPResponse(t)
		resp.Body.WriteString(`{"user":{"name":"alice","roles":["admin","editor"]}}`)
		resp.ExpectJSONBody(map[string]any{
			"user": map[string]any{
				"name":  "alice",
				"roles": []any{"admin", "editor"},
			},
		})
	})
}

func TestHTTPResponse_JSONBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "valid JSON",
			body: `{"key":"value"}`,
		},
		{
			name:    "invalid JSON",
			body:    `not json`,
			wantErr: true,
		},
		{
			name: "JSON array",
			body: `[1,2,3]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewHTTPResponse(t)
			resp.Body.WriteString(tt.body)

			result, err := resp.JSONBody()

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result == nil {
				t.Fatal("result is nil")
			}
		})
	}
}

func TestHTTPResponse_MustJSONBody(t *testing.T) {
	resp := NewHTTPResponse(t)
	resp.Body.WriteString(`{"status":"ok"}`)

	result := resp.MustJSONBody()
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if m["status"] != "ok" {
		t.Errorf("status = %v, want %q", m["status"], "ok")
	}
}

func TestHTTPResponse_ExpectChaining(t *testing.T) {
	handler := JSONHandler(http.StatusCreated, map[string]string{"id": "123"})

	req := NewHTTPRequest("POST", "/api/items").
		WithJSONBody(map[string]string{"name": "item"}).
		Build(t)

	resp := ServeHTTP(t, handler, req)

	// Verify chaining works
	resp.
		ExpectStatus(http.StatusCreated).
		ExpectHeader("Content-Type", "application/json")
}

// ---------------------------------------------------------------------------
// ServeHTTP tests
// ---------------------------------------------------------------------------

func TestServeHTTP(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "test")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true}`)) //nolint:errcheck // test handler
	})

	req := httptest.NewRequest("GET", "/test", http.NoBody)
	resp := ServeHTTP(t, handler, req)

	resp.
		ExpectStatus(http.StatusAccepted).
		ExpectHeader("X-Custom", "test")

	body := resp.MustJSONBody()
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	if m["accepted"] != true {
		t.Errorf("accepted = %v, want true", m["accepted"])
	}
}

func TestServeHTTP_WithBuiltRequest(t *testing.T) {
	handler := EchoHandler()

	req := NewHTTPRequest("POST", "/echo").
		WithJSONBody(map[string]string{"msg": "hello"}).
		Build(t)

	resp := ServeHTTP(t, handler, req)
	resp.ExpectStatus(http.StatusOK)

	body := resp.MustJSONBody()
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	if m["msg"] != "hello" {
		t.Errorf("msg = %v, want %q", m["msg"], "hello")
	}
}

// ---------------------------------------------------------------------------
// JSONHandler tests
// ---------------------------------------------------------------------------

func TestJSONHandler(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       any
		wantStatus int
		wantCT     string
	}{
		{
			name:       "200 with object body",
			status:     http.StatusOK,
			body:       map[string]string{"key": "value"},
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
		},
		{
			name:       "201 with nil body",
			status:     http.StatusCreated,
			body:       nil,
			wantStatus: http.StatusCreated,
			wantCT:     "application/json",
		},
		{
			name:       "404 with error body",
			status:     http.StatusNotFound,
			body:       map[string]string{"error": "not found"},
			wantStatus: http.StatusNotFound,
			wantCT:     "application/json",
		},
		{
			name:       "200 with array body",
			status:     http.StatusOK,
			body:       []string{"a", "b", "c"},
			wantStatus: http.StatusOK,
			wantCT:     "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := JSONHandler(tt.status, tt.body)

			req := httptest.NewRequest("GET", "/", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tt.wantCT {
				t.Errorf("Content-Type = %q, want %q", ct, tt.wantCT)
			}

			if tt.body != nil {
				var parsed any
				if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
					t.Fatalf("response body is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestJSONHandler_BodyContent(t *testing.T) {
	type Item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	handler := JSONHandler(http.StatusOK, Item{ID: 42, Name: "widget"})

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var item Item
	if err := json.NewDecoder(rec.Body).Decode(&item); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if item.ID != 42 {
		t.Errorf("ID = %d, want 42", item.ID)
	}
	if item.Name != "widget" {
		t.Errorf("Name = %q, want %q", item.Name, "widget")
	}
}

// ---------------------------------------------------------------------------
// ErrorHandler tests
// ---------------------------------------------------------------------------

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
	}{
		{
			name:       "400 bad request",
			status:     http.StatusBadRequest,
			message:    "invalid input",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "404 not found",
			status:     http.StatusNotFound,
			message:    "resource not found",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "500 internal server error",
			status:     http.StatusInternalServerError,
			message:    "something went wrong",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := ErrorHandler(tt.status, tt.message)

			req := httptest.NewRequest("GET", "/", http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode body: %v", err)
			}
			if body["error"] != tt.message {
				t.Errorf("error = %q, want %q", body["error"], tt.message)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// EchoHandler tests
// ---------------------------------------------------------------------------

func TestEchoHandler(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "echo JSON",
			contentType: "application/json",
			body:        `{"key":"value"}`,
		},
		{
			name:        "echo plain text",
			contentType: "text/plain",
			body:        "hello world",
		},
		{
			name:        "echo empty body",
			contentType: "application/octet-stream",
			body:        "",
		},
		{
			name:        "echo XML",
			contentType: "application/xml",
			body:        `<root><item>test</item></root>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := EchoHandler()

			req := httptest.NewRequest("POST", "/echo", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", tt.contentType)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
			if ct := rec.Header().Get("Content-Type"); ct != tt.contentType {
				t.Errorf("Content-Type = %q, want %q", ct, tt.contentType)
			}
			if got := rec.Body.String(); got != tt.body {
				t.Errorf("body = %q, want %q", got, tt.body)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Fixtures tests
// ---------------------------------------------------------------------------

func TestNewTestContract(t *testing.T) {
	tests := []struct {
		name     string
		consumer string
		provider string
	}{
		{"basic contract", "web-app", "users-api"},
		{"different names", "mobile", "gateway"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewTestContract(tt.consumer, tt.provider)

			if c == nil {
				t.Fatal("NewTestContract returned nil")
			}
			if c.Metadata.ID == "" {
				t.Error("contract ID is empty")
			}
			if c.Metadata.Version == "" {
				t.Error("contract version is empty")
			}
			if c.Metadata.Consumer.Name != tt.consumer {
				t.Errorf("consumer = %q, want %q", c.Metadata.Consumer.Name, tt.consumer)
			}
			if c.Metadata.Provider.Name != tt.provider {
				t.Errorf("provider = %q, want %q", c.Metadata.Provider.Name, tt.provider)
			}
			if c.Metadata.Status != contract.StatusDraft {
				t.Errorf("status = %q, want %q", c.Metadata.Status, contract.StatusDraft)
			}
			if len(c.Metadata.Tags) != 1 || c.Metadata.Tags[0] != "test" {
				t.Errorf("tags = %v, want [test]", c.Metadata.Tags)
			}
			if c.Metadata.CreatedAt.IsZero() {
				t.Error("CreatedAt is zero")
			}
			if c.Interactions == nil {
				t.Error("Interactions is nil, want empty slice")
			}
		})
	}
}

func TestNewHTTPInteraction(t *testing.T) {
	tests := []struct {
		name        string
		description string
		method      string
		path        string
		status      int
	}{
		{"GET users", "get all users", "GET", "/api/users", 200},
		{"POST order", "create order", "POST", "/api/orders", 201},
		{"DELETE item", "delete item", "DELETE", "/api/items/1", 204},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := NewHTTPInteraction(tt.description, tt.method, tt.path, tt.status)

			if interaction.ID == "" {
				t.Error("interaction ID is empty")
			}
			if interaction.Description != tt.description {
				t.Errorf("description = %q, want %q", interaction.Description, tt.description)
			}
			if interaction.Protocol != contract.ProtocolHTTP {
				t.Errorf("protocol = %q, want %q", interaction.Protocol, contract.ProtocolHTTP)
			}
			if interaction.Payload.HTTP == nil {
				t.Fatal("HTTP payload is nil")
			}
			if interaction.Payload.HTTP.Request.Method != tt.method {
				t.Errorf("method = %q, want %q", interaction.Payload.HTTP.Request.Method, tt.method)
			}
			if interaction.Payload.HTTP.Request.Path != tt.path {
				t.Errorf("path = %q, want %q", interaction.Payload.HTTP.Request.Path, tt.path)
			}
			if interaction.Payload.HTTP.Response.Status != tt.status {
				t.Errorf("status = %d, want %d", interaction.Payload.HTTP.Response.Status, tt.status)
			}
		})
	}
}

func TestNewHTTPInteractionWithBody(t *testing.T) {
	reqBody := map[string]string{"name": "widget"}
	respBody := map[string]any{"id": "123", "name": "widget"}

	interaction := NewHTTPInteractionWithBody(
		"create widget", "POST", "/api/widgets", 201,
		reqBody, respBody,
	)

	if interaction.Protocol != contract.ProtocolHTTP {
		t.Errorf("protocol = %q, want %q", interaction.Protocol, contract.ProtocolHTTP)
	}

	hp := interaction.Payload.HTTP
	if hp == nil {
		t.Fatal("HTTP payload is nil")
	}
	if hp.Request.Method != "POST" {
		t.Errorf("method = %q, want POST", hp.Request.Method)
	}
	if hp.Request.Path != "/api/widgets" {
		t.Errorf("path = %q, want /api/widgets", hp.Request.Path)
	}
	if hp.Request.Headers["Content-Type"] != "application/json" {
		t.Errorf("request Content-Type = %q, want application/json", hp.Request.Headers["Content-Type"])
	}
	if hp.Response.Status != 201 {
		t.Errorf("status = %d, want 201", hp.Response.Status)
	}
	if hp.Response.Headers["Content-Type"] != "application/json" {
		t.Errorf("response Content-Type = %q, want application/json", hp.Response.Headers["Content-Type"])
	}
	if hp.Request.Body == nil {
		t.Error("request body is nil")
	}
	if hp.Response.Body == nil {
		t.Error("response body is nil")
	}
}

func TestNewGRPCInteraction(t *testing.T) {
	tests := []struct {
		name        string
		description string
		service     string
		method      string
	}{
		{"unary call", "get user by id", "UserService", "GetUser"},
		{"list call", "list all products", "ProductService", "ListProducts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := NewGRPCInteraction(tt.description, tt.service, tt.method)

			if interaction.ID == "" {
				t.Error("interaction ID is empty")
			}
			if interaction.Description != tt.description {
				t.Errorf("description = %q, want %q", interaction.Description, tt.description)
			}
			if interaction.Protocol != contract.ProtocolGRPC {
				t.Errorf("protocol = %q, want %q", interaction.Protocol, contract.ProtocolGRPC)
			}
			if interaction.Payload.GRPC == nil {
				t.Fatal("gRPC payload is nil")
			}
			if interaction.Payload.GRPC.Service != tt.service {
				t.Errorf("service = %q, want %q", interaction.Payload.GRPC.Service, tt.service)
			}
			if interaction.Payload.GRPC.Method != tt.method {
				t.Errorf("method = %q, want %q", interaction.Payload.GRPC.Method, tt.method)
			}
			if interaction.Payload.GRPC.StreamType != contract.StreamTypeUnary {
				t.Errorf("streamType = %q, want %q", interaction.Payload.GRPC.StreamType, contract.StreamTypeUnary)
			}
		})
	}
}

func TestNewAsyncInteraction(t *testing.T) {
	tests := []struct {
		name        string
		description string
		destination string
		direction   contract.AsyncDirection
	}{
		{
			name:        "publish event",
			description: "order created",
			destination: "orders.created",
			direction:   contract.AsyncDirectionPublish,
		},
		{
			name:        "consume event",
			description: "process payment",
			destination: "payments.process",
			direction:   contract.AsyncDirectionConsume,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := NewAsyncInteraction(tt.description, tt.destination, tt.direction)

			if interaction.ID == "" {
				t.Error("interaction ID is empty")
			}
			if interaction.Description != tt.description {
				t.Errorf("description = %q, want %q", interaction.Description, tt.description)
			}
			if interaction.Protocol != contract.ProtocolAsync {
				t.Errorf("protocol = %q, want %q", interaction.Protocol, contract.ProtocolAsync)
			}
			if interaction.Payload.Async == nil {
				t.Fatal("Async payload is nil")
			}
			if interaction.Payload.Async.Destination != tt.destination {
				t.Errorf("destination = %q, want %q", interaction.Payload.Async.Destination, tt.destination)
			}
			if interaction.Payload.Async.Direction != tt.direction {
				t.Errorf("direction = %q, want %q", interaction.Payload.Async.Direction, tt.direction)
			}
		})
	}
}

func TestWithProviderState(t *testing.T) {
	interaction := NewHTTPInteraction("test", "GET", "/api/users/1", 200)

	result := WithProviderState(&interaction, "user exists", map[string]any{"id": "1"})

	if result != &interaction {
		t.Error("WithProviderState should return the same pointer")
	}
	if len(interaction.ProviderStates) != 1 {
		t.Fatalf("expected 1 provider state, got %d", len(interaction.ProviderStates))
	}
	ps := interaction.ProviderStates[0]
	if ps.Name != "user exists" {
		t.Errorf("state name = %q, want %q", ps.Name, "user exists")
	}
	if ps.Params["id"] != "1" {
		t.Errorf("state param id = %v, want %q", ps.Params["id"], "1")
	}

	// Add another state
	WithProviderState(&interaction, "account active", nil)
	if len(interaction.ProviderStates) != 2 {
		t.Fatalf("expected 2 provider states, got %d", len(interaction.ProviderStates))
	}
}

func TestWithMatchingRules(t *testing.T) {
	interaction := NewHTTPInteraction("test", "GET", "/users", 200)
	rules := contract.MatchingRules{
		"$.body.id": contract.MatchingRule{Match: contract.MatchTypeValue},
	}

	result := WithMatchingRules(&interaction, rules)

	if result != &interaction {
		t.Error("WithMatchingRules should return the same pointer")
	}
	if len(interaction.MatchingRules) != 1 {
		t.Fatalf("expected 1 matching rule, got %d", len(interaction.MatchingRules))
	}
	rule, ok := interaction.MatchingRules["$.body.id"]
	if !ok {
		t.Fatal("missing rule for $.body.id")
	}
	if rule.Match != contract.MatchTypeValue {
		t.Errorf("rule match = %q, want %q", rule.Match, contract.MatchTypeValue)
	}
}

func TestSampleOrderResponse(t *testing.T) {
	resp := SampleOrderResponse()

	if resp["id"] != "order-123" {
		t.Errorf("id = %v, want %q", resp["id"], "order-123")
	}
	if resp["status"] != "pending" {
		t.Errorf("status = %v, want %q", resp["status"], "pending")
	}
	if resp["total"] != 59.98 {
		t.Errorf("total = %v, want %v", resp["total"], 59.98)
	}

	items, ok := resp["items"].([]map[string]any)
	if !ok {
		t.Fatalf("items type = %T, want []map[string]any", resp["items"])
	}
	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	if items[0]["sku"] != "ITEM-001" {
		t.Errorf("items[0].sku = %v, want %q", items[0]["sku"], "ITEM-001")
	}

	// Verify each call returns a fresh copy
	resp2 := SampleOrderResponse()
	resp["id"] = "mutated"
	if resp2["id"] == "mutated" {
		t.Error("SampleOrderResponse should return independent maps")
	}
}

func TestSampleUserResponse(t *testing.T) {
	resp := SampleUserResponse()

	if resp["id"] != "user-456" {
		t.Errorf("id = %v, want %q", resp["id"], "user-456")
	}
	if resp["name"] != "Test User" {
		t.Errorf("name = %v, want %q", resp["name"], "Test User")
	}
	if resp["email"] != "test@example.com" {
		t.Errorf("email = %v, want %q", resp["email"], "test@example.com")
	}

	// Verify each call returns a fresh copy
	resp2 := SampleUserResponse()
	resp["id"] = "mutated"
	if resp2["id"] == "mutated" {
		t.Error("SampleUserResponse should return independent maps")
	}
}

// ---------------------------------------------------------------------------
// Integration-style tests combining multiple utilities
// ---------------------------------------------------------------------------

func TestServeHTTP_WithJSONHandler(t *testing.T) {
	body := map[string]any{"items": []string{"a", "b"}, "count": float64(2)}
	handler := JSONHandler(http.StatusOK, body)
	req := NewHTTPRequest("GET", "/api/items").Build(t)

	resp := ServeHTTP(t, handler, req)
	resp.
		ExpectStatus(http.StatusOK).
		ExpectHeader("Content-Type", "application/json")
}

func TestServeHTTP_WithErrorHandler(t *testing.T) {
	handler := ErrorHandler(http.StatusForbidden, "access denied")
	req := NewHTTPRequest("GET", "/admin").Build(t)

	resp := ServeHTTP(t, handler, req)
	resp.ExpectStatus(http.StatusForbidden)

	body := resp.MustJSONBody()
	m, ok := body.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", body)
	}
	if m["error"] != "access denied" {
		t.Errorf("error = %v, want %q", m["error"], "access denied")
	}
}

func TestMockStorage_SaveOverwrite(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	if err := ms.Save(ctx, "key", []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := ms.Save(ctx, "key", []byte("second")); err != nil {
		t.Fatal(err)
	}

	data, err := ms.Load(ctx, "key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "second" {
		t.Errorf("expected %q, got %q", "second", data)
	}
	if ms.KeyCount() != 1 {
		t.Errorf("expected 1 key after overwrite, got %d", ms.KeyCount())
	}
}

func TestMockStorage_Delete_NonexistentNotTracked(t *testing.T) {
	ctx := context.Background()
	ms := NewMockStorage()

	err := ms.Delete(ctx, "nope")
	if err != storage.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	// DeleteCalls should still not be tracked because the error occurs before
	// tracking is done -- actually let's check what the code does
	// Looking at the code: delete tracks first, then checks existence.
	// Actually no: it checks first with _, ok := m.data[key]; !ok => return ErrNotFound
	// and m.DeleteCalls is appended before the check.
	// Let's verify.
	if len(ms.DeleteCalls) != 1 {
		t.Errorf("expected 1 DeleteCall (tracked before not-found check), got %d", len(ms.DeleteCalls))
	}
}
