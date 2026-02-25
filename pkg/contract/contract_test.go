package contract

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewContract(t *testing.T) {
	c := NewContract("consumer-svc", "provider-svc")

	if c.Metadata.Consumer.Name != "consumer-svc" {
		t.Errorf("expected consumer name 'consumer-svc', got %q", c.Metadata.Consumer.Name)
	}
	if c.Metadata.Provider.Name != "provider-svc" {
		t.Errorf("expected provider name 'provider-svc', got %q", c.Metadata.Provider.Name)
	}
	if c.Metadata.Status != StatusDraft {
		t.Errorf("expected status 'draft', got %q", c.Metadata.Status)
	}
	if c.Metadata.ID == "" {
		t.Error("expected non-empty ID")
	}
	if c.Metadata.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", c.Metadata.Version)
	}
}

func TestValidate_ValidHTTPContract(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{
				ID:       "get-user",
				Protocol: ProtocolHTTP,
				Payload: Payload{
					HTTP: &HTTPPayload{
						Request:  HTTPRequest{Method: "GET", Path: "/users/1"},
						Response: HTTPResponse{Status: 200},
					},
				},
			},
		},
	}

	if err := Validate(c); err != nil {
		t.Errorf("expected valid contract, got error: %v", err)
	}
}

func TestValidate_ValidGRPCContract(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{
				ID:       "grpc-call",
				Protocol: ProtocolGRPC,
				Payload: Payload{
					GRPC: &GRPCPayload{
						Service: "UserService",
						Method:  "GetUser",
					},
				},
			},
		},
	}

	if err := Validate(c); err != nil {
		t.Errorf("expected valid contract, got error: %v", err)
	}
}

func TestValidate_ValidAsyncContract(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{
				ID:       "async-msg",
				Protocol: ProtocolAsync,
				Payload: Payload{
					Async: &AsyncPayload{
						Destination: "orders.created",
					},
				},
			},
		},
	}

	if err := Validate(c); err != nil {
		t.Errorf("expected valid contract, got error: %v", err)
	}
}

func TestValidate_MissingID(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for missing ID")
	}
	if !strings.Contains(err.Error(), "contract ID is required") {
		t.Errorf("expected 'contract ID is required' error, got: %v", err)
	}
}

func TestValidate_MissingConsumer(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Provider: Participant{Name: "provider"},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for missing consumer")
	}
	if !strings.Contains(err.Error(), "consumer name is required") {
		t.Errorf("expected 'consumer name is required' error, got: %v", err)
	}
}

func TestValidate_MissingProvider(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for missing provider")
	}
	if !strings.Contains(err.Error(), "provider name is required") {
		t.Errorf("expected 'provider name is required' error, got: %v", err)
	}
}

func TestValidate_MissingInteractionID(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{Protocol: ProtocolHTTP},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for missing interaction ID")
	}
	if !strings.Contains(err.Error(), "ID is required") {
		t.Errorf("expected 'ID is required' error, got: %v", err)
	}
}

func TestValidate_MissingHTTPPayload(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{ID: "test", Protocol: ProtocolHTTP},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for missing HTTP payload")
	}
	if !strings.Contains(err.Error(), "HTTP payload is required") {
		t.Errorf("expected 'HTTP payload is required' error, got: %v", err)
	}
}

func TestValidate_UnknownProtocol(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "test-id",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{ID: "test", Protocol: "unknown"},
		},
	}

	err := Validate(c)
	if err == nil {
		t.Error("expected error for unknown protocol")
	}
	if !strings.Contains(err.Error(), "unknown protocol") {
		t.Errorf("expected 'unknown protocol' error, got: %v", err)
	}
}

func TestClone(t *testing.T) {
	original := NewContract("consumer", "provider")
	original.Interactions = []Interaction{
		{
			ID:       "test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/test"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	clone, err := Clone(original)
	if err != nil {
		t.Fatalf("failed to clone: %v", err)
	}

	// Verify clone matches original
	if clone.Metadata.ID != original.Metadata.ID {
		t.Error("clone ID doesn't match")
	}
	if len(clone.Interactions) != len(original.Interactions) {
		t.Error("clone interactions count doesn't match")
	}

	// Modify clone and verify original is unchanged
	clone.Metadata.ID = "modified"
	if original.Metadata.ID == "modified" {
		t.Error("modifying clone affected original")
	}
}

func TestSaveAndLoadBytes(t *testing.T) {
	original := NewContract("consumer", "provider")
	original.Interactions = []Interaction{
		{
			ID:       "test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "POST", Path: "/items"},
					Response: HTTPResponse{Status: 201},
				},
			},
		},
	}

	data, err := SaveToBytes(original)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Metadata.ID != original.Metadata.ID {
		t.Error("loaded ID doesn't match")
	}
	if loaded.Metadata.Consumer.Name != original.Metadata.Consumer.Name {
		t.Error("loaded consumer doesn't match")
	}
	if len(loaded.Interactions) != 1 {
		t.Error("loaded interactions count doesn't match")
	}
	if loaded.Interactions[0].Payload.HTTP.Request.Method != "POST" {
		t.Error("loaded HTTP method doesn't match")
	}
}

func TestSaveAndLoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "contract.json")

	original := NewContract("file-consumer", "file-provider")
	original.Interactions = []Interaction{
		{
			ID:       "file-test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/file"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	if err := SaveToFile(original, path); err != nil {
		t.Fatalf("failed to save to file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file was not created")
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("failed to load from file: %v", err)
	}

	if loaded.Metadata.Consumer.Name != "file-consumer" {
		t.Error("loaded consumer doesn't match")
	}
}

func TestLoadFromReader(t *testing.T) {
	json := `{
		"metadata": {
			"id": "reader-test",
			"consumer": {"name": "reader-consumer"},
			"provider": {"name": "reader-provider"}
		},
		"interactions": [
			{
				"id": "test",
				"protocol": "http",
				"payload": {
					"http": {
						"request": {"method": "GET", "path": "/test"},
						"response": {"status": 200}
					}
				}
			}
		]
	}`

	loaded, err := LoadFromReader(strings.NewReader(json))
	if err != nil {
		t.Fatalf("failed to load from reader: %v", err)
	}

	if loaded.Metadata.ID != "reader-test" {
		t.Errorf("expected ID 'reader-test', got %q", loaded.Metadata.ID)
	}
}

func TestLoadFromBytes_InvalidJSON(t *testing.T) {
	_, err := LoadFromBytes([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadFromBytes_InvalidContract(t *testing.T) {
	// Valid JSON but missing required fields
	_, err := LoadFromBytes([]byte(`{"metadata": {}}`))
	if err == nil {
		t.Error("expected error for invalid contract")
	}
}

func TestComputeChecksum(t *testing.T) {
	interactions := []Interaction{
		{
			ID:       "test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/test"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	checksum1, err := ComputeChecksum(interactions)
	if err != nil {
		t.Fatalf("failed to compute checksum: %v", err)
	}

	if len(checksum1) != 64 { // SHA256 produces 64 hex characters
		t.Errorf("expected 64 character checksum, got %d", len(checksum1))
	}

	// Same interactions should produce same checksum
	checksum2, err := ComputeChecksum(interactions)
	if err != nil {
		t.Fatalf("failed to compute checksum: %v", err)
	}

	if checksum1 != checksum2 {
		t.Error("identical interactions should produce identical checksum")
	}
}

func TestComputeChecksum_DifferentOrder(t *testing.T) {
	// Create two contracts with same data but potentially different field order
	interactions1 := []Interaction{
		{
			ID:          "test",
			Description: "test interaction",
			Protocol:    ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/test"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	// Create with same data - canonical JSON should produce same checksum
	interactions2 := []Interaction{
		{
			ID:          "test",
			Description: "test interaction",
			Protocol:    ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/test"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	checksum1, err := ComputeChecksum(interactions1)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}
	checksum2, err := ComputeChecksum(interactions2)
	if err != nil {
		t.Fatalf("ComputeChecksum failed: %v", err)
	}

	if checksum1 != checksum2 {
		t.Error("same data should produce same checksum regardless of creation order")
	}
}

func TestVerifyChecksum(t *testing.T) {
	c := NewContract("consumer", "provider")
	c.Interactions = []Interaction{
		{
			ID:       "test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/test"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	// No checksum should be valid
	valid, err := VerifyChecksum(c)
	if err != nil {
		t.Fatalf("failed to verify checksum: %v", err)
	}
	if !valid {
		t.Error("contract without checksum should be valid")
	}

	// Update checksum
	if err = UpdateChecksum(c); err != nil {
		t.Fatalf("failed to update checksum: %v", err)
	}

	// Should now be valid
	valid, err = VerifyChecksum(c)
	if err != nil {
		t.Fatalf("failed to verify checksum: %v", err)
	}
	if !valid {
		t.Error("contract with correct checksum should be valid")
	}

	// Modify contract and checksum should fail
	c.Interactions[0].Description = "modified"
	valid, err = VerifyChecksum(c)
	if err != nil {
		t.Fatalf("failed to verify checksum: %v", err)
	}
	if valid {
		t.Error("modified contract should have invalid checksum")
	}
}

func TestCanonicalJSON(t *testing.T) {
	// Test that keys are sorted
	data := map[string]any{
		"zebra": 1,
		"apple": 2,
		"mango": 3,
	}

	canonical, err := CanonicalJSON(data)
	if err != nil {
		t.Fatalf("failed to produce canonical JSON: %v", err)
	}

	// Keys should be in alphabetical order
	expected := `{"apple":2,"mango":3,"zebra":1}`
	if string(canonical) != expected {
		t.Errorf("expected %s, got %s", expected, string(canonical))
	}
}

func TestPayloadMarshalUnmarshal_HTTP(t *testing.T) {
	original := Payload{
		HTTP: &HTTPPayload{
			Request:  HTTPRequest{Method: "GET", Path: "/test"},
			Response: HTTPResponse{Status: 200},
		},
	}

	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var loaded Payload
	if err := loaded.UnmarshalJSON(data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if loaded.HTTP == nil {
		t.Fatal("HTTP payload should not be nil")
	}
	if loaded.HTTP.Request.Method != "GET" {
		t.Error("HTTP method doesn't match")
	}
	if loaded.GRPC != nil || loaded.Async != nil {
		t.Error("other payloads should be nil")
	}
}

func TestPayloadMarshalUnmarshal_GRPC(t *testing.T) {
	original := Payload{
		GRPC: &GRPCPayload{
			Service: "UserService",
			Method:  "GetUser",
		},
	}

	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var loaded Payload
	if err := loaded.UnmarshalJSON(data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if loaded.GRPC == nil {
		t.Fatal("GRPC payload should not be nil")
	}
	if loaded.GRPC.Service != "UserService" {
		t.Error("GRPC service doesn't match")
	}
}

func TestPayloadMarshalUnmarshal_Async(t *testing.T) {
	original := Payload{
		Async: &AsyncPayload{
			Destination: "orders.created",
			Direction:   AsyncDirectionPublish,
		},
	}

	data, err := original.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var loaded Payload
	if err := loaded.UnmarshalJSON(data); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if loaded.Async == nil {
		t.Fatal("Async payload should not be nil")
	}
	if loaded.Async.Destination != "orders.created" {
		t.Error("Async destination doesn't match")
	}
}

func TestPayloadMarshal_Empty(t *testing.T) {
	empty := Payload{}
	data, err := empty.MarshalJSON()
	if err != nil {
		t.Fatalf("failed to marshal empty payload: %v", err)
	}
	if string(data) != "{}" {
		t.Errorf("expected '{}', got %s", string(data))
	}
}

func TestSaveToWriter(t *testing.T) {
	c := NewContract("writer-consumer", "writer-provider")
	c.Interactions = []Interaction{
		{
			ID:       "writer-test",
			Protocol: ProtocolHTTP,
			Payload: Payload{
				HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/writer"},
					Response: HTTPResponse{Status: 200},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := SaveToWriter(c, &buf); err != nil {
		t.Fatalf("failed to save to writer: %v", err)
	}

	// Verify it's valid JSON that can be loaded back
	loaded, err := LoadFromBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("failed to load written JSON: %v", err)
	}

	if loaded.Metadata.Consumer.Name != "writer-consumer" {
		t.Error("loaded consumer doesn't match")
	}
}

func TestContractWithMatchingRules(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "rules-test",
			Consumer: Participant{Name: "consumer"},
			Provider: Participant{Name: "provider"},
		},
		Interactions: []Interaction{
			{
				ID:       "with-rules",
				Protocol: ProtocolHTTP,
				Payload: Payload{
					HTTP: &HTTPPayload{
						Request:  HTTPRequest{Method: "GET", Path: "/test"},
						Response: HTTPResponse{Status: 200, Body: map[string]any{"id": "123"}},
					},
				},
				MatchingRules: MatchingRules{
					"$.response.body.id": {Match: MatchTypeValue},
				},
			},
		},
	}

	if err := Validate(c); err != nil {
		t.Errorf("contract with matching rules should be valid: %v", err)
	}

	// Serialize and deserialize
	data, err := SaveToBytes(c)
	if err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded.Interactions[0].MatchingRules) != 1 {
		t.Error("matching rules should be preserved")
	}
	if loaded.Interactions[0].MatchingRules["$.response.body.id"].Match != MatchTypeValue {
		t.Error("matching rule type should be preserved")
	}
}

func TestMetadataTimestamp(t *testing.T) {
	before := time.Now().UTC().Truncate(time.Second)
	c := NewContract("consumer", "provider")
	after := time.Now().UTC().Add(time.Second)

	if c.Metadata.CreatedAt.Before(before) || c.Metadata.CreatedAt.After(after) {
		t.Error("created_at should be approximately now")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// validHTTPContract returns a minimal valid contract with an HTTP interaction.
func validHTTPContract(t *testing.T) *Contract {
	t.Helper()
	return &Contract{
		Metadata: Metadata{
			ID:        "test-id",
			Version:   "1.0.0",
			Consumer:  Participant{Name: "consumer"},
			Provider:  Participant{Name: "provider"},
			Status:    StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []Interaction{
			{
				ID:          "get-users",
				Description: "get all users",
				Protocol:    ProtocolHTTP,
				Payload: Payload{
					HTTP: &HTTPPayload{
						Request:  HTTPRequest{Method: "GET", Path: "/users"},
						Response: HTTPResponse{Status: 200, Body: map[string]any{"users": []any{}}},
					},
				},
			},
		},
	}
}

// validGRPCContract returns a minimal valid contract with a gRPC interaction.
func validGRPCContract(t *testing.T) *Contract {
	t.Helper()
	return &Contract{
		Metadata: Metadata{
			ID:        "grpc-id",
			Version:   "1.0.0",
			Consumer:  Participant{Name: "grpc-consumer"},
			Provider:  Participant{Name: "grpc-provider"},
			Status:    StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []Interaction{
			{
				ID:       "grpc-get-user",
				Protocol: ProtocolGRPC,
				Payload: Payload{
					GRPC: &GRPCPayload{
						Service:    "UserService",
						Method:     "GetUser",
						StreamType: StreamTypeUnary,
						Request:    GRPCMessage{Message: map[string]any{"id": "1"}},
						Response:   GRPCResponse{Status: "OK", Message: map[string]any{"name": "Alice"}},
					},
				},
			},
		},
	}
}

// validAsyncContract returns a minimal valid contract with an async interaction.
func validAsyncContract(t *testing.T) *Contract {
	t.Helper()
	return &Contract{
		Metadata: Metadata{
			ID:        "async-id",
			Version:   "1.0.0",
			Consumer:  Participant{Name: "async-consumer"},
			Provider:  Participant{Name: "async-provider"},
			Status:    StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []Interaction{
			{
				ID:       "async-order-created",
				Protocol: ProtocolAsync,
				Payload: Payload{
					Async: &AsyncPayload{
						Destination: "orders.created",
						Direction:   AsyncDirectionPublish,
						Message: AsyncMessage{
							Payload: map[string]any{"order_id": "42"},
						},
					},
				},
			},
		},
	}
}

// writeContractJSON is a helper that writes raw JSON to a temporary file and
// returns the path. The caller does not need to clean up; t.TempDir handles it.
func writeContractJSON(t *testing.T, jsonContent string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "contract.json")
	if err := os.WriteFile(path, []byte(jsonContent), 0o644); err != nil {
		t.Fatalf("writeContractJSON: %v", err)
	}
	return path
}

// ---------------------------------------------------------------------------
// LoadFromFile
// ---------------------------------------------------------------------------

func TestLoadFromFile_Errors(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T) string // returns path
		wantSub string                    // substring expected in err
	}{
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "does-not-exist.json")
			},
			wantSub: "failed to open contract file",
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T) string {
				t.Helper()
				return writeContractJSON(t, `{not json}`)
			},
			wantSub: "failed to decode contract",
		},
		{
			name: "validation failure: missing ID",
			setup: func(t *testing.T) string {
				t.Helper()
				return writeContractJSON(t, `{
					"metadata": {"consumer": {"name": "c"}, "provider": {"name": "p"}},
					"interactions": []
				}`)
			},
			wantSub: "contract ID is required",
		},
		{
			name: "validation failure: missing consumer",
			setup: func(t *testing.T) string {
				t.Helper()
				return writeContractJSON(t, `{
					"metadata": {"id": "x", "provider": {"name": "p"}},
					"interactions": []
				}`)
			},
			wantSub: "consumer name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			_, err := LoadFromFile(path)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LoadFromReader
// ---------------------------------------------------------------------------

func TestLoadFromReader_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSub string
	}{
		{
			name:    "empty input",
			input:   "",
			wantSub: "failed to decode contract",
		},
		{
			name:    "invalid JSON",
			input:   "<<<NOT-JSON>>>",
			wantSub: "failed to decode contract",
		},
		{
			name:    "valid JSON missing required fields",
			input:   `{"metadata":{}}`,
			wantSub: "contract ID is required",
		},
		{
			name: "interaction with missing protocol",
			input: `{
				"metadata": {"id":"x","consumer":{"name":"c"},"provider":{"name":"p"}},
				"interactions": [{"id":"i1"}]
			}`,
			wantSub: "protocol is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadFromReader(strings.NewReader(tt.input))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// LoadFromBytes
// ---------------------------------------------------------------------------

func TestLoadFromBytes_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantSub string
	}{
		{
			name:    "nil input",
			input:   nil,
			wantSub: "failed to decode contract",
		},
		{
			name:    "empty bytes",
			input:   []byte{},
			wantSub: "failed to decode contract",
		},
		{
			name:    "garbage bytes",
			input:   []byte{0xff, 0xfe},
			wantSub: "failed to decode contract",
		},
		{
			name:    "valid JSON but invalid contract",
			input:   []byte(`{"metadata":{"id":"","consumer":{"name":"c"},"provider":{"name":"p"}}}`),
			wantSub: "contract ID is required",
		},
		{
			name: "HTTP interaction missing payload",
			input: []byte(`{
				"metadata":{"id":"x","consumer":{"name":"c"},"provider":{"name":"p"}},
				"interactions":[{"id":"i","protocol":"http"}]
			}`),
			wantSub: "HTTP payload is required",
		},
		{
			name: "gRPC interaction missing payload",
			input: []byte(`{
				"metadata":{"id":"x","consumer":{"name":"c"},"provider":{"name":"p"}},
				"interactions":[{"id":"i","protocol":"grpc"}]
			}`),
			wantSub: "gRPC payload is required",
		},
		{
			name: "async interaction missing payload",
			input: []byte(`{
				"metadata":{"id":"x","consumer":{"name":"c"},"provider":{"name":"p"}},
				"interactions":[{"id":"i","protocol":"async"}]
			}`),
			wantSub: "async payload is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadFromBytes(tt.input)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// SaveToFile / SaveToBytes / SaveToWriter round-trip
// ---------------------------------------------------------------------------

func TestSaveToFile_RoundTrip(t *testing.T) {
	protocols := []struct {
		name     string
		contract func(t *testing.T) *Contract
	}{
		{"HTTP", validHTTPContract},
		{"gRPC", validGRPCContract},
		{"Async", validAsyncContract},
	}

	for _, p := range protocols {
		t.Run(p.name, func(t *testing.T) {
			original := p.contract(t)
			dir := t.TempDir()
			path := filepath.Join(dir, "contract.json")

			if err := SaveToFile(original, path); err != nil {
				t.Fatalf("SaveToFile: %v", err)
			}

			loaded, err := LoadFromFile(path)
			if err != nil {
				t.Fatalf("LoadFromFile: %v", err)
			}

			if loaded.Metadata.ID != original.Metadata.ID {
				t.Errorf("ID mismatch: got %q, want %q", loaded.Metadata.ID, original.Metadata.ID)
			}
			if loaded.Metadata.Consumer.Name != original.Metadata.Consumer.Name {
				t.Errorf("consumer mismatch: got %q, want %q", loaded.Metadata.Consumer.Name, original.Metadata.Consumer.Name)
			}
			if loaded.Metadata.Provider.Name != original.Metadata.Provider.Name {
				t.Errorf("provider mismatch: got %q, want %q", loaded.Metadata.Provider.Name, original.Metadata.Provider.Name)
			}
			if len(loaded.Interactions) != len(original.Interactions) {
				t.Fatalf("interaction count: got %d, want %d", len(loaded.Interactions), len(original.Interactions))
			}
			if loaded.Interactions[0].Protocol != original.Interactions[0].Protocol {
				t.Errorf("protocol mismatch: got %q, want %q", loaded.Interactions[0].Protocol, original.Interactions[0].Protocol)
			}
		})
	}
}

func TestSaveToBytes_IndentedOutput(t *testing.T) {
	c := validHTTPContract(t)

	data, err := SaveToBytes(c)
	if err != nil {
		t.Fatalf("SaveToBytes: %v", err)
	}

	output := string(data)

	// Verify the output is indented (contains leading spaces).
	if !strings.Contains(output, "  ") {
		t.Error("expected indented output with two-space indent")
	}

	// Verify it contains expected top-level keys.
	for _, key := range []string{`"metadata"`, `"interactions"`} {
		if !strings.Contains(output, key) {
			t.Errorf("output missing expected key %s", key)
		}
	}
}

func TestSaveToWriter_IndentedOutput(t *testing.T) {
	c := validHTTPContract(t)

	var buf bytes.Buffer
	if err := SaveToWriter(c, &buf); err != nil {
		t.Fatalf("SaveToWriter: %v", err)
	}

	output := buf.String()
	// Encoder adds a trailing newline.
	if !strings.HasSuffix(output, "\n") {
		t.Error("SaveToWriter output should end with newline")
	}
	// Verify indentation.
	if !strings.Contains(output, "  ") {
		t.Error("expected indented output")
	}
}

func TestSaveToBytes_RoundTrip(t *testing.T) {
	original := validHTTPContract(t)
	original.Interactions[0].MatchingRules = MatchingRules{
		"$.response.body.users": {Match: MatchEachLike, Min: intPtr(1)},
	}

	data, err := SaveToBytes(original)
	if err != nil {
		t.Fatalf("SaveToBytes: %v", err)
	}

	loaded, err := LoadFromBytes(data)
	if err != nil {
		t.Fatalf("LoadFromBytes: %v", err)
	}

	if len(loaded.Interactions[0].MatchingRules) != 1 {
		t.Fatalf("matching rules count: got %d, want 1", len(loaded.Interactions[0].MatchingRules))
	}
	rule := loaded.Interactions[0].MatchingRules["$.response.body.users"]
	if rule.Match != MatchEachLike {
		t.Errorf("rule.Match = %q, want %q", rule.Match, MatchEachLike)
	}
	if rule.Min == nil || *rule.Min != 1 {
		t.Error("rule.Min should be 1")
	}
}

func TestSaveToWriter_RoundTrip(t *testing.T) {
	original := validGRPCContract(t)

	var buf bytes.Buffer
	if err := SaveToWriter(original, &buf); err != nil {
		t.Fatalf("SaveToWriter: %v", err)
	}

	loaded, err := LoadFromReader(&buf)
	if err != nil {
		t.Fatalf("LoadFromReader: %v", err)
	}

	if loaded.Interactions[0].Payload.GRPC == nil {
		t.Fatal("expected gRPC payload after round-trip")
	}
	if loaded.Interactions[0].Payload.GRPC.Service != "UserService" {
		t.Errorf("GRPC service = %q, want %q", loaded.Interactions[0].Payload.GRPC.Service, "UserService")
	}
}

// ---------------------------------------------------------------------------
// Validate — table-driven per-protocol missing fields and unknown protocol
// ---------------------------------------------------------------------------

func TestValidate_MissingMetadataFields(t *testing.T) {
	tests := []struct {
		name    string
		meta    Metadata
		wantSub string
	}{
		{
			name:    "empty ID",
			meta:    Metadata{ID: "", Consumer: Participant{Name: "c"}, Provider: Participant{Name: "p"}},
			wantSub: "contract ID is required",
		},
		{
			name:    "empty consumer name",
			meta:    Metadata{ID: "x", Consumer: Participant{}, Provider: Participant{Name: "p"}},
			wantSub: "consumer name is required",
		},
		{
			name:    "empty provider name",
			meta:    Metadata{ID: "x", Consumer: Participant{Name: "c"}, Provider: Participant{}},
			wantSub: "provider name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Contract{Metadata: tt.meta}
			err := Validate(c)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

func TestValidate_HTTPInteractionErrors(t *testing.T) {
	baseMeta := Metadata{ID: "x", Consumer: Participant{Name: "c"}, Provider: Participant{Name: "p"}}

	tests := []struct {
		name        string
		interaction Interaction
		wantSub     string
	}{
		{
			name:        "missing interaction ID",
			interaction: Interaction{Protocol: ProtocolHTTP},
			wantSub:     "ID is required",
		},
		{
			name:        "nil HTTP payload",
			interaction: Interaction{ID: "i1", Protocol: ProtocolHTTP},
			wantSub:     "HTTP payload is required",
		},
		{
			name: "missing HTTP method",
			interaction: Interaction{
				ID: "i1", Protocol: ProtocolHTTP,
				Payload: Payload{HTTP: &HTTPPayload{
					Request:  HTTPRequest{Path: "/foo"},
					Response: HTTPResponse{Status: 200},
				}},
			},
			wantSub: "HTTP request method is required",
		},
		{
			name: "missing HTTP path",
			interaction: Interaction{
				ID: "i1", Protocol: ProtocolHTTP,
				Payload: Payload{HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET"},
					Response: HTTPResponse{Status: 200},
				}},
			},
			wantSub: "HTTP request path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Contract{
				Metadata:     baseMeta,
				Interactions: []Interaction{tt.interaction},
			}
			err := Validate(c)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

func TestValidate_GRPCInteractionErrors(t *testing.T) {
	baseMeta := Metadata{ID: "x", Consumer: Participant{Name: "c"}, Provider: Participant{Name: "p"}}

	tests := []struct {
		name        string
		interaction Interaction
		wantSub     string
	}{
		{
			name:        "nil gRPC payload",
			interaction: Interaction{ID: "i1", Protocol: ProtocolGRPC},
			wantSub:     "gRPC payload is required",
		},
		{
			name: "missing gRPC service",
			interaction: Interaction{
				ID: "i1", Protocol: ProtocolGRPC,
				Payload: Payload{GRPC: &GRPCPayload{Method: "GetUser"}},
			},
			wantSub: "gRPC service is required",
		},
		{
			name: "missing gRPC method",
			interaction: Interaction{
				ID: "i1", Protocol: ProtocolGRPC,
				Payload: Payload{GRPC: &GRPCPayload{Service: "UserService"}},
			},
			wantSub: "gRPC method is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Contract{
				Metadata:     baseMeta,
				Interactions: []Interaction{tt.interaction},
			}
			err := Validate(c)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

func TestValidate_AsyncInteractionErrors(t *testing.T) {
	baseMeta := Metadata{ID: "x", Consumer: Participant{Name: "c"}, Provider: Participant{Name: "p"}}

	tests := []struct {
		name        string
		interaction Interaction
		wantSub     string
	}{
		{
			name:        "nil async payload",
			interaction: Interaction{ID: "i1", Protocol: ProtocolAsync},
			wantSub:     "async payload is required",
		},
		{
			name: "missing async destination",
			interaction: Interaction{
				ID: "i1", Protocol: ProtocolAsync,
				Payload: Payload{Async: &AsyncPayload{Direction: AsyncDirectionPublish}},
			},
			wantSub: "async destination is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Contract{
				Metadata:     baseMeta,
				Interactions: []Interaction{tt.interaction},
			}
			err := Validate(c)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err, tt.wantSub)
			}
		})
	}
}

func TestValidate_UnknownProtocol_Table(t *testing.T) {
	baseMeta := Metadata{ID: "x", Consumer: Participant{Name: "c"}, Provider: Participant{Name: "p"}}

	tests := []struct {
		name     string
		protocol Protocol
	}{
		{"unknown", Protocol("unknown")},
		{"websocket", Protocol("websocket")},
		{"empty string after ID set", Protocol("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Contract{
				Metadata: baseMeta,
				Interactions: []Interaction{
					{ID: "i1", Protocol: tt.protocol},
				},
			}
			err := Validate(c)
			if err == nil {
				t.Fatalf("expected error for protocol %q", tt.protocol)
			}
			// For empty protocol the error is "protocol is required", for others
			// it is "unknown protocol".
			if tt.protocol == "" {
				if !strings.Contains(err.Error(), "protocol is required") {
					t.Errorf("error = %q, want substring %q", err, "protocol is required")
				}
			} else {
				if !strings.Contains(err.Error(), "unknown protocol") {
					t.Errorf("error = %q, want substring %q", err, "unknown protocol")
				}
			}
		})
	}
}

func TestValidate_NoInteractions_IsValid(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "no-interactions",
			Consumer: Participant{Name: "c"},
			Provider: Participant{Name: "p"},
		},
	}
	if err := Validate(c); err != nil {
		t.Errorf("contract with no interactions should be valid, got: %v", err)
	}
}

func TestValidate_MultipleInteractions_FirstInvalid(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "multi",
			Consumer: Participant{Name: "c"},
			Provider: Participant{Name: "p"},
		},
		Interactions: []Interaction{
			{ID: "bad", Protocol: ProtocolHTTP}, // missing HTTP payload
			{
				ID: "good", Protocol: ProtocolHTTP,
				Payload: Payload{HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/ok"},
					Response: HTTPResponse{Status: 200},
				}},
			},
		},
	}
	err := Validate(c)
	if err == nil {
		t.Fatal("expected error for first invalid interaction")
	}
	if !strings.Contains(err.Error(), "interaction 0") {
		t.Errorf("error should reference interaction 0, got: %v", err)
	}
}

func TestValidate_MultipleInteractions_SecondInvalid(t *testing.T) {
	c := &Contract{
		Metadata: Metadata{
			ID:       "multi",
			Consumer: Participant{Name: "c"},
			Provider: Participant{Name: "p"},
		},
		Interactions: []Interaction{
			{
				ID: "good", Protocol: ProtocolHTTP,
				Payload: Payload{HTTP: &HTTPPayload{
					Request:  HTTPRequest{Method: "GET", Path: "/ok"},
					Response: HTTPResponse{Status: 200},
				}},
			},
			{ID: "bad", Protocol: ProtocolGRPC}, // missing gRPC payload
		},
	}
	err := Validate(c)
	if err == nil {
		t.Fatal("expected error for second invalid interaction")
	}
	if !strings.Contains(err.Error(), "interaction 1") {
		t.Errorf("error should reference interaction 1, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Clone — deep copy verification
// ---------------------------------------------------------------------------

func TestClone_DeepCopy_HTTP(t *testing.T) {
	original := validHTTPContract(t)
	original.Metadata.Tags = []string{"tag1", "tag2"}
	original.Interactions[0].Payload.HTTP.Request.Headers = map[string]string{
		"Authorization": "Bearer token",
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Verify equality.
	if cloned.Metadata.ID != original.Metadata.ID {
		t.Errorf("ID mismatch after clone")
	}
	if len(cloned.Metadata.Tags) != len(original.Metadata.Tags) {
		t.Fatalf("tags length mismatch")
	}
	if cloned.Interactions[0].Payload.HTTP.Request.Headers["Authorization"] != "Bearer token" {
		t.Error("HTTP header not cloned")
	}

	// Mutate clone and verify original is unaffected.
	cloned.Metadata.ID = "mutated-id"
	cloned.Metadata.Tags[0] = "mutated-tag"
	cloned.Interactions[0].Payload.HTTP.Request.Headers["Authorization"] = "mutated"
	cloned.Interactions[0].Payload.HTTP.Request.Path = "/mutated"

	if original.Metadata.ID == "mutated-id" {
		t.Error("mutating clone ID affected original")
	}
	if original.Metadata.Tags[0] == "mutated-tag" {
		t.Error("mutating clone tags affected original")
	}
	if original.Interactions[0].Payload.HTTP.Request.Headers["Authorization"] == "mutated" {
		t.Error("mutating clone HTTP headers affected original")
	}
	if original.Interactions[0].Payload.HTTP.Request.Path == "/mutated" {
		t.Error("mutating clone HTTP path affected original")
	}
}

func TestClone_DeepCopy_GRPC(t *testing.T) {
	original := validGRPCContract(t)
	original.Interactions[0].Payload.GRPC.Request.Metadata = map[string]string{
		"x-request-id": "abc",
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Mutate clone metadata map.
	cloned.Interactions[0].Payload.GRPC.Request.Metadata["x-request-id"] = "mutated"

	if original.Interactions[0].Payload.GRPC.Request.Metadata["x-request-id"] == "mutated" {
		t.Error("mutating clone gRPC request metadata affected original")
	}
}

func TestClone_DeepCopy_Async(t *testing.T) {
	original := validAsyncContract(t)
	original.Interactions[0].Payload.Async.Message.Headers = map[string]string{
		"content-type": "application/json",
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	cloned.Interactions[0].Payload.Async.Message.Headers["content-type"] = "text/plain"

	if original.Interactions[0].Payload.Async.Message.Headers["content-type"] == "text/plain" {
		t.Error("mutating clone async headers affected original")
	}
}

func TestClone_DeepCopy_MatchingRules(t *testing.T) {
	original := validHTTPContract(t)
	minVal := 1
	original.Interactions[0].MatchingRules = MatchingRules{
		"$.response.body.users": {Match: MatchEachLike, Min: &minVal},
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Mutate cloned matching rule.
	newMin := 99
	clonedRule := cloned.Interactions[0].MatchingRules["$.response.body.users"]
	clonedRule.Min = &newMin
	cloned.Interactions[0].MatchingRules["$.response.body.users"] = clonedRule

	origRule := original.Interactions[0].MatchingRules["$.response.body.users"]
	if origRule.Min == nil || *origRule.Min != 1 {
		t.Error("mutating clone matching rule Min affected original")
	}
}

func TestClone_DeepCopy_ProviderStates(t *testing.T) {
	original := validHTTPContract(t)
	original.Interactions[0].ProviderStates = []ProviderState{
		{
			Name:   "user exists",
			Params: map[string]any{"id": "42"},
		},
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	// Mutate clone's provider states.
	cloned.Interactions[0].ProviderStates[0].Name = "mutated"
	cloned.Interactions[0].ProviderStates[0].Params["id"] = "999"

	if original.Interactions[0].ProviderStates[0].Name == "mutated" {
		t.Error("mutating clone provider state name affected original")
	}
	if original.Interactions[0].ProviderStates[0].Params["id"] == "999" {
		t.Error("mutating clone provider state params affected original")
	}
}

func TestClone_EmptyContract(t *testing.T) {
	original := &Contract{
		Metadata: Metadata{
			ID:       "empty",
			Consumer: Participant{Name: "c"},
			Provider: Participant{Name: "p"},
		},
	}

	cloned, err := Clone(original)
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}

	if cloned.Metadata.ID != "empty" {
		t.Error("clone ID mismatch for empty contract")
	}
	if len(cloned.Interactions) != 0 {
		t.Error("clone of empty contract should have no interactions")
	}
}

// intPtr is a helper to create a pointer to an int.
func intPtr(n int) *int {
	return &n
}
