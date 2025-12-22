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

	checksum1, _ := ComputeChecksum(interactions1)
	checksum2, _ := ComputeChecksum(interactions2)

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
	if err := UpdateChecksum(c); err != nil {
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
					"$.response.body.id": {Match: MatchType_},
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
	if loaded.Interactions[0].MatchingRules["$.response.body.id"].Match != MatchType_ {
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
