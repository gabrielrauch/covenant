package testutil

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
)

// MockStorage is an in-memory storage backend for testing.
type MockStorage struct {
	mu   sync.RWMutex
	data map[string][]byte

	// SaveCalls tracks all Save operations for verification.
	SaveCalls []SaveCall
	// LoadCalls tracks all Load operations for verification.
	LoadCalls []string
	// DeleteCalls tracks all Delete operations for verification.
	DeleteCalls []string

	// Error injection for testing error handling.
	SaveError   error
	LoadError   error
	ListError   error
	DeleteError error
	ExistsError error
}

// SaveCall records a Save operation.
type SaveCall struct {
	Key  string
	Data []byte
}

// NewMockStorage creates a new in-memory mock storage.
func NewMockStorage() *MockStorage {
	return &MockStorage{
		data: make(map[string][]byte),
	}
}

// Save stores data at the given key.
func (m *MockStorage) Save(ctx context.Context, key string, data []byte) error {
	if m.SaveError != nil {
		return m.SaveError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Make a copy of the data to avoid mutation issues
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	m.data[key] = dataCopy
	m.SaveCalls = append(m.SaveCalls, SaveCall{Key: key, Data: dataCopy})

	return nil
}

// Load retrieves data from the given key.
func (m *MockStorage) Load(ctx context.Context, key string) ([]byte, error) {
	if m.LoadError != nil {
		return nil, m.LoadError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	m.LoadCalls = append(m.LoadCalls, key)

	data, ok := m.data[key]
	if !ok {
		return nil, storage.ErrNotFound
	}

	// Return a copy to avoid mutation issues
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)

	return dataCopy, nil
}

// List returns all keys matching the prefix.
func (m *MockStorage) List(ctx context.Context, prefix string) ([]string, error) {
	if m.ListError != nil {
		return nil, m.ListError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []string
	for key := range m.data {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	return keys, nil
}

// Delete removes the data at the given key.
func (m *MockStorage) Delete(ctx context.Context, key string) error {
	if m.DeleteError != nil {
		return m.DeleteError
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.DeleteCalls = append(m.DeleteCalls, key)

	if _, ok := m.data[key]; !ok {
		return storage.ErrNotFound
	}

	delete(m.data, key)
	return nil
}

// Exists checks if a key exists.
func (m *MockStorage) Exists(ctx context.Context, key string) (bool, error) {
	if m.ExistsError != nil {
		return false, m.ExistsError
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.data[key]
	return ok, nil
}

// Transaction executes multiple operations atomically.
func (m *MockStorage) Transaction(ctx context.Context, ops []storage.Operation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Execute all operations
	for _, op := range ops {
		switch op.Type {
		case storage.OpSave:
			if m.SaveError != nil {
				return m.SaveError
			}
			dataCopy := make([]byte, len(op.Data))
			copy(dataCopy, op.Data)
			m.data[op.Key] = dataCopy
			m.SaveCalls = append(m.SaveCalls, SaveCall{Key: op.Key, Data: dataCopy})
		case storage.OpDelete:
			if m.DeleteError != nil {
				return m.DeleteError
			}
			m.DeleteCalls = append(m.DeleteCalls, op.Key)
			delete(m.data, op.Key)
		}
	}

	return nil
}

// Reset clears all stored data and recorded calls.
func (m *MockStorage) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[string][]byte)
	m.SaveCalls = nil
	m.LoadCalls = nil
	m.DeleteCalls = nil
	m.SaveError = nil
	m.LoadError = nil
	m.ListError = nil
	m.DeleteError = nil
	m.ExistsError = nil
}

// GetData returns a copy of all stored data for inspection.
func (m *MockStorage) GetData() map[string][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]byte)
	for k, v := range m.data {
		dataCopy := make([]byte, len(v))
		copy(dataCopy, v)
		result[k] = dataCopy
	}
	return result
}

// KeyCount returns the number of stored keys.
func (m *MockStorage) KeyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}

// Ensure MockStorage implements the Backend interface.
var _ storage.Backend = (*MockStorage)(nil)
