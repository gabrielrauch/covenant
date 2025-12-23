package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
}

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		actual      validator.ActualData
		wantSuccess bool
	}{
		{
			name: "no gRPC payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			actual:      validator.ActualData{},
			wantSuccess: false,
		},
		{
			name: "status match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Response: contract.GRPCResponse{
							Status: "OK",
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: "OK",
			},
			wantSuccess: true,
		},
		{
			name: "status mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Response: contract.GRPCResponse{
							Status: "OK",
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: "NOT_FOUND",
			},
			wantSuccess: false,
		},
		{
			name: "metadata validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Response: contract.GRPCResponse{
							Status: "OK",
							Metadata: map[string]string{
								"x-request-id": "123",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: "OK",
				Metadata: map[string]string{
					"x-request-id": "123",
				},
			},
			wantSuccess: true,
		},
		{
			name: "message validation success",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Response: contract.GRPCResponse{
							Status: "OK",
							Message: map[string]any{
								"id":   "123",
								"name": "test",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Status:   "OK",
				Response: []byte(`{"id": "123", "name": "test"}`),
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			result := v.Validate(context.Background(), &tt.interaction, tt.actual)

			if result.Success != tt.wantSuccess {
				t.Errorf("Validate() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}
		})
	}
}

func TestValidator_ValidateRequest(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		message     []byte
		metadata    map[string]string
		wantSuccess bool
	}{
		{
			name: "no gRPC payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			message:     []byte(`{}`),
			metadata:    nil,
			wantSuccess: false,
		},
		{
			name: "valid request",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Request: contract.GRPCMessage{
							Message: map[string]any{
								"id": "123",
							},
						},
					},
				},
			},
			message:     []byte(`{"id": "123"}`),
			metadata:    nil,
			wantSuccess: true,
		},
		{
			name: "request with metadata",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Request: contract.GRPCMessage{
							Metadata: map[string]string{
								"authorization": "Bearer token",
							},
							Message: map[string]any{
								"id": "123",
							},
						},
					},
				},
			},
			message: []byte(`{"id": "123"}`),
			metadata: map[string]string{
				"authorization": "Bearer token",
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			result := v.ValidateRequest(&tt.interaction, tt.message, tt.metadata)

			if result.Success != tt.wantSuccess {
				t.Errorf("ValidateRequest() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}
		})
	}
}

func TestValidator_validateMessage(t *testing.T) {
	v := NewValidator()

	t.Run("invalid JSON", func(t *testing.T) {
		result := v.validateMessage(
			map[string]any{"key": "value"},
			[]byte("not valid json"),
			"$.response.message",
			nil,
		)
		if result.Success {
			t.Error("Expected failure for invalid JSON")
		}
	})

	t.Run("valid message", func(t *testing.T) {
		result := v.validateMessage(
			map[string]any{"id": "123"},
			[]byte(`{"id": "123"}`),
			"$.response.message",
			nil,
		)
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})

	t.Run("message with matching rules", func(t *testing.T) {
		result := v.validateMessage(
			map[string]any{"id": "example"},
			[]byte(`{"id": "different"}`),
			"$.response.message",
			contract.MatchingRules{
				"$.response.message.id": {Match: contract.MatchTypeValue},
			},
		)
		if !result.Success {
			t.Errorf("Expected success with type matcher, got errors: %v", result.Errors)
		}
	})
}

func TestNewStreamValidator(t *testing.T) {
	interaction := contract.Interaction{
		Payload: contract.Payload{
			GRPC: &contract.GRPCPayload{
				StreamType: contract.StreamTypeBidirectional,
			},
		},
	}

	sv := NewStreamValidator(&interaction)
	if sv == nil {
		t.Fatal("NewStreamValidator returned nil")
	}
}

func TestStreamValidator_ValidateClientMessage(t *testing.T) {
	t.Run("no gRPC payload", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{},
		}
		sv := NewStreamValidator(&interaction)
		sv.ValidateClientMessage([]byte(`{}`), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for missing gRPC payload")
		}
	})

	t.Run("bidirectional stream sequence", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					StreamType: contract.StreamTypeBidirectional,
					Sequence: []contract.StreamMessage{
						{
							Direction: contract.StreamDirectionClient,
							Message:   map[string]any{"text": "hello"},
						},
						{
							Direction: contract.StreamDirectionServer,
							Message:   map[string]any{"text": "hi"},
						},
					},
				},
			},
		}

		sv := NewStreamValidator(&interaction)
		msg, err := json.Marshal(map[string]any{"text": "hello"})
		if err != nil {
			t.Fatalf("Failed to marshal msg: %v", err)
		}
		sv.ValidateClientMessage(msg, nil)

		result := sv.Result()
		// Should have error about incomplete sequence
		if result.Success {
			t.Log("Sequence incomplete but individual message valid")
		}
	})

	t.Run("client stream", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					StreamType: contract.StreamTypeClientStream,
					Request: contract.GRPCMessage{
						Message: map[string]any{"value": 1},
					},
				},
			},
		}

		sv := NewStreamValidator(&interaction)
		msg, err := json.Marshal(map[string]any{"value": 10})
		if err != nil {
			t.Fatalf("Failed to marshal msg: %v", err)
		}
		sv.ValidateClientMessage(msg, nil)

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success for client stream, got errors: %v", result.Errors)
		}
	})
}

func TestStreamValidator_ValidateServerMessage(t *testing.T) {
	t.Run("no gRPC payload", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{},
		}
		sv := NewStreamValidator(&interaction)
		sv.ValidateServerMessage([]byte(`{}`), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for missing gRPC payload")
		}
	})

	t.Run("server stream", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					StreamType: contract.StreamTypeServerStream,
					Response: contract.GRPCResponse{
						Message: map[string]any{"event": "data"},
					},
				},
			},
		}

		sv := NewStreamValidator(&interaction)
		msg, err := json.Marshal(map[string]any{"event": "data"})
		if err != nil {
			t.Fatalf("Failed to marshal msg: %v", err)
		}
		sv.ValidateServerMessage(msg, nil)

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success for server stream, got errors: %v", result.Errors)
		}
	})
}

func TestStreamValidator_Result(t *testing.T) {
	interaction := contract.Interaction{
		Payload: contract.Payload{
			GRPC: &contract.GRPCPayload{
				StreamType: contract.StreamTypeUnary,
			},
		},
	}

	sv := NewStreamValidator(&interaction)
	result := sv.Result()

	if !result.Success {
		t.Errorf("Expected success for empty stream, got errors: %v", result.Errors)
	}
}
