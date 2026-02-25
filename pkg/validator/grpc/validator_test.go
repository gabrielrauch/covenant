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

// ---------------------------------------------------------------------------
// Additional tests below — extends coverage for uncovered branches
// ---------------------------------------------------------------------------

// helper to build a minimal gRPC interaction for tests.
func grpcInteraction(fn func(*contract.GRPCPayload)) contract.Interaction {
	p := &contract.GRPCPayload{}
	if fn != nil {
		fn(p)
	}
	return contract.Interaction{
		Payload: contract.Payload{GRPC: p},
	}
}

func TestValidator_Validate_NilStatus(t *testing.T) {
	t.Helper()
	// When actual.Status is nil the status comparison block is skipped entirely.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status: nil,
	})
	if !result.Success {
		t.Errorf("expected success when status is nil (skipped), got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_NonStringStatus(t *testing.T) {
	t.Helper()
	// Status is typed as `any`; a non-string value should not trigger the
	// string comparison and therefore should not produce a status error.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status: 200, // integer, not string
	})
	if !result.Success {
		t.Errorf("expected success when status is non-string, got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_MetadataMismatch(t *testing.T) {
	t.Helper()
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
		p.Response.Metadata = map[string]string{
			"x-trace-id": "abc",
		}
	})

	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status: "OK",
		Metadata: map[string]string{
			"x-trace-id": "different-value",
		},
	})
	if result.Success {
		t.Error("expected failure when metadata values mismatch")
	}
	found := false
	for _, e := range result.Errors {
		if e.Path != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected at least one error with a non-empty path for metadata mismatch")
	}
}

func TestValidator_Validate_NilResponseMessage(t *testing.T) {
	t.Helper()
	// payload.Response.Message is nil => message validation is skipped even
	// when actual.Response bytes are present.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
		p.Response.Message = nil
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status:   "OK",
		Response: []byte(`{"any":"data"}`),
	})
	if !result.Success {
		t.Errorf("expected success when response message is nil (skipped), got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_EmptyActualResponse(t *testing.T) {
	t.Helper()
	// payload.Response.Message is set but actual.Response is empty => skip.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
		p.Response.Message = map[string]any{"id": "1"}
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status:   "OK",
		Response: nil, // empty
	})
	if !result.Success {
		t.Errorf("expected success when actual response is empty (skipped), got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_ResponseMessageMismatch(t *testing.T) {
	t.Helper()
	// Structural mismatch between expected and actual message bodies.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
		p.Response.Message = map[string]any{"id": "1", "name": "alice"}
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status:   "OK",
		Response: []byte(`{"id":"1"}`), // missing "name"
	})
	if result.Success {
		t.Error("expected failure when actual message is missing expected fields")
	}
}

func TestValidator_ValidateRequest_MetadataMismatch(t *testing.T) {
	t.Helper()
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Request.Metadata = map[string]string{"authorization": "Bearer secret"}
		p.Request.Message = map[string]any{"id": "1"}
	})

	v := NewValidator()
	result := v.ValidateRequest(&interaction, []byte(`{"id":"1"}`), map[string]string{
		"authorization": "wrong-token",
	})
	if result.Success {
		t.Error("expected failure for mismatched request metadata")
	}
}

func TestValidator_ValidateRequest_NilExpectedMessage(t *testing.T) {
	t.Helper()
	// expected.Message is nil => message validation skipped, even with bytes.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Request.Message = nil
	})

	v := NewValidator()
	result := v.ValidateRequest(&interaction, []byte(`{"anything":"here"}`), nil)
	if !result.Success {
		t.Errorf("expected success when expected message is nil, got errors: %v", result.Errors)
	}
}

func TestValidator_ValidateRequest_EmptyActualMessage(t *testing.T) {
	t.Helper()
	// Actual message bytes are empty => message validation skipped.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Request.Message = map[string]any{"id": "1"}
	})

	v := NewValidator()
	result := v.ValidateRequest(&interaction, nil, nil)
	if !result.Success {
		t.Errorf("expected success when actual message is empty, got errors: %v", result.Errors)
	}
}

func TestValidator_validateMessage_RequestPath(t *testing.T) {
	t.Helper()
	// Exercise the $.request.message wrapping branch.
	v := NewValidator()
	result := v.validateMessage(
		map[string]any{"id": "123"},
		[]byte(`{"id": "123"}`),
		"$.request.message",
		nil,
	)
	if !result.Success {
		t.Errorf("expected success for request path message, got errors: %v", result.Errors)
	}
}

func TestValidator_validateMessage_StructuralMismatch(t *testing.T) {
	t.Helper()
	// Actual missing a key that expected has => structural mismatch error.
	v := NewValidator()
	result := v.validateMessage(
		map[string]any{"id": "1", "name": "alice"},
		[]byte(`{"id":"1"}`),
		"$.response.message",
		nil,
	)
	if result.Success {
		t.Error("expected failure when actual is missing expected fields")
	}
	found := false
	for _, e := range result.Errors {
		if e.Path == "$.response.message.name" {
			found = true
		}
	}
	if !found {
		t.Error("expected error path $.response.message.name for missing field")
	}
}

func TestValidator_validateMessage_ExtraFieldInActual(t *testing.T) {
	t.Helper()
	// Extra fields in actual should NOT cause errors (consumer-driven: only
	// expected fields matter).
	v := NewValidator()
	result := v.validateMessage(
		map[string]any{"id": "1"},
		[]byte(`{"id":"1","extra":"field"}`),
		"$.response.message",
		nil,
	)
	if !result.Success {
		t.Errorf("extra fields should not cause failure, got errors: %v", result.Errors)
	}
}

// ---------------------------------------------------------------------------
// StreamValidator — sequence boundary and wrong direction tests
// ---------------------------------------------------------------------------

func bidiInteraction(seq []contract.StreamMessage) contract.Interaction {
	return contract.Interaction{
		Payload: contract.Payload{
			GRPC: &contract.GRPCPayload{
				StreamType: contract.StreamTypeBidirectional,
				Sequence:   seq,
			},
		},
	}
}

func TestStreamValidator_ClientMessage_BeyondSequenceBoundary(t *testing.T) {
	t.Helper()
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionClient, Message: map[string]any{"x": 1}},
	})

	sv := NewStreamValidator(&interaction)
	// First message is within bounds.
	sv.ValidateClientMessage([]byte(`{"x":1}`), nil)
	// Second message exceeds the sequence.
	sv.ValidateClientMessage([]byte(`{"x":2}`), nil)

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when client message exceeds sequence length")
	}
	found := false
	for _, e := range result.Errors {
		if e.Message == "unexpected message at sequence position 1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'unexpected message at sequence position 1' error, got: %v", result.Errors)
	}
}

func TestStreamValidator_ClientMessage_WrongDirection(t *testing.T) {
	t.Helper()
	// Sequence expects a server message but receives a client message.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionServer, Message: map[string]any{"y": 1}},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateClientMessage([]byte(`{"y":1}`), nil)

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when client message sent at server-expected position")
	}
	found := false
	for _, e := range result.Errors {
		if e.Message == "expected server message at position 0, got client message" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected direction mismatch error, got: %v", result.Errors)
	}
}

func TestStreamValidator_ClientMessage_NilSequenceMessage(t *testing.T) {
	t.Helper()
	// Sequence entry has nil Message — content validation is skipped.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionClient, Message: nil},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateClientMessage([]byte(`{"anything":"here"}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success when sequence message is nil (skip content validation), got errors: %v", result.Errors)
	}
}

func TestStreamValidator_ServerMessage_BeyondSequenceBoundary(t *testing.T) {
	t.Helper()
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionServer, Message: map[string]any{"z": 1}},
	})

	sv := NewStreamValidator(&interaction)
	// First message is within bounds.
	sv.ValidateServerMessage([]byte(`{"z":1}`), nil)
	// Second message exceeds sequence.
	sv.ValidateServerMessage([]byte(`{"z":2}`), nil)

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when server message exceeds sequence length")
	}
	found := false
	for _, e := range result.Errors {
		if e.Message == "unexpected message at sequence position 1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'unexpected message at sequence position 1' error, got: %v", result.Errors)
	}
}

func TestStreamValidator_ServerMessage_WrongDirection(t *testing.T) {
	t.Helper()
	// Sequence expects a client message but receives a server message.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionClient, Message: map[string]any{"a": 1}},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateServerMessage([]byte(`{"a":1}`), nil)

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when server message sent at client-expected position")
	}
	found := false
	for _, e := range result.Errors {
		if e.Message == "expected client message at position 0, got server message" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected direction mismatch error, got: %v", result.Errors)
	}
}

func TestStreamValidator_ServerMessage_NilSequenceMessage(t *testing.T) {
	t.Helper()
	// Sequence entry has nil Message — content validation is skipped.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionServer, Message: nil},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateServerMessage([]byte(`{"anything":"here"}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success when sequence message is nil (skip content validation), got errors: %v", result.Errors)
	}
}

func TestStreamValidator_ServerMessage_BidirectionalWithContent(t *testing.T) {
	t.Helper()
	// Verify a correct bidirectional server message with content validation.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionServer, Message: map[string]any{"reply": "ok"}},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateServerMessage([]byte(`{"reply":"ok"}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success for matching server message, got errors: %v", result.Errors)
	}
}

func TestStreamValidator_ServerMessage_BidirectionalContentMismatch(t *testing.T) {
	t.Helper()
	// Content mismatch in a bidirectional server message.
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionServer, Message: map[string]any{"reply": "ok", "code": float64(200)}},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateServerMessage([]byte(`{"reply":"ok"}`), nil) // missing "code"

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when bidirectional server message has structural mismatch")
	}
}

func TestStreamValidator_Result_WithErrors(t *testing.T) {
	t.Helper()
	// Ensure Result() returns Success=false when errors are accumulated.
	interaction := contract.Interaction{
		Payload: contract.Payload{}, // no gRPC => will produce error
	}
	sv := NewStreamValidator(&interaction)
	sv.ValidateClientMessage([]byte(`{}`), nil)
	sv.ValidateServerMessage([]byte(`{}`), nil)

	result := sv.Result()
	if result.Success {
		t.Error("expected failure when errors were accumulated")
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors (one per call), got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestStreamValidator_ClientStream_ValidatesAgainstRequestSchema(t *testing.T) {
	t.Helper()
	// Multiple client-stream messages should each be validated against the
	// request schema defined in the contract.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.StreamType = contract.StreamTypeClientStream
		p.Request = contract.GRPCMessage{
			Message: map[string]any{"value": float64(1)},
		}
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateClientMessage([]byte(`{"value":10}`), nil)
	sv.ValidateClientMessage([]byte(`{"value":20}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success for valid client stream messages, got errors: %v", result.Errors)
	}
}

func TestStreamValidator_ServerStream_MultipleMessages(t *testing.T) {
	t.Helper()
	// Multiple server-stream messages should each be validated against the
	// response message schema.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.StreamType = contract.StreamTypeServerStream
		p.Response = contract.GRPCResponse{
			Message: map[string]any{"event": "data"},
		}
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateServerMessage([]byte(`{"event":"data"}`), nil)
	sv.ValidateServerMessage([]byte(`{"event":"data"}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success for valid server stream messages, got errors: %v", result.Errors)
	}
}

func TestStreamValidator_BidirectionalSequence_FullConversation(t *testing.T) {
	t.Helper()
	// Walk through an entire bidirectional sequence:
	// client -> server -> client
	interaction := bidiInteraction([]contract.StreamMessage{
		{Direction: contract.StreamDirectionClient, Message: map[string]any{"msg": "hello"}},
		{Direction: contract.StreamDirectionServer, Message: map[string]any{"msg": "hi"}},
		{Direction: contract.StreamDirectionClient, Message: map[string]any{"msg": "bye"}},
	})

	sv := NewStreamValidator(&interaction)
	sv.ValidateClientMessage([]byte(`{"msg":"hello"}`), nil)
	sv.ValidateServerMessage([]byte(`{"msg":"hi"}`), nil)
	sv.ValidateClientMessage([]byte(`{"msg":"bye"}`), nil)

	result := sv.Result()
	if !result.Success {
		t.Errorf("expected success for full bidirectional conversation, got errors: %v", result.Errors)
	}
}

func TestStreamValidator_ClientMessage_UnaryOrServerStream_NoOp(t *testing.T) {
	t.Helper()
	// For unary or server-stream types, ValidateClientMessage should be a
	// no-op (no sequence, not client_stream).
	tests := []struct {
		name       string
		streamType contract.StreamType
	}{
		{"unary", contract.StreamTypeUnary},
		{"server_stream", contract.StreamTypeServerStream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := grpcInteraction(func(p *contract.GRPCPayload) {
				p.StreamType = tt.streamType
			})
			sv := NewStreamValidator(&interaction)
			sv.ValidateClientMessage([]byte(`{"anything":true}`), nil)

			result := sv.Result()
			if !result.Success {
				t.Errorf("expected success (no-op) for stream type %s, got errors: %v",
					tt.streamType, result.Errors)
			}
		})
	}
}

func TestStreamValidator_ServerMessage_UnaryOrClientStream_NoOp(t *testing.T) {
	t.Helper()
	// For unary or client-stream types, ValidateServerMessage should be a
	// no-op (no sequence, not server_stream).
	tests := []struct {
		name       string
		streamType contract.StreamType
	}{
		{"unary", contract.StreamTypeUnary},
		{"client_stream", contract.StreamTypeClientStream},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interaction := grpcInteraction(func(p *contract.GRPCPayload) {
				p.StreamType = tt.streamType
			})
			sv := NewStreamValidator(&interaction)
			sv.ValidateServerMessage([]byte(`{"anything":true}`), nil)

			result := sv.Result()
			if !result.Success {
				t.Errorf("expected success (no-op) for stream type %s, got errors: %v",
					tt.streamType, result.Errors)
			}
		})
	}
}

func TestValidator_Validate_InvalidResponseJSON(t *testing.T) {
	t.Helper()
	// When actual.Response contains invalid JSON, validateMessage should
	// report a parse error.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
		p.Response.Message = map[string]any{"id": "1"}
	})

	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status:   "OK",
		Response: []byte(`{not valid json`),
	})
	if result.Success {
		t.Error("expected failure when actual response contains invalid JSON")
	}
}

func TestValidator_ValidateRequest_InvalidJSON(t *testing.T) {
	t.Helper()
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Request.Message = map[string]any{"id": "1"}
	})

	v := NewValidator()
	result := v.ValidateRequest(&interaction, []byte(`%%%`), nil)
	if result.Success {
		t.Error("expected failure for invalid JSON in request")
	}
}

func TestValidator_validateMessage_MatchingRulesOnRequestPath(t *testing.T) {
	t.Helper()
	// Exercise matching rules with the $.request.message base path to ensure
	// rules are filtered and applied correctly for requests.
	v := NewValidator()
	result := v.validateMessage(
		map[string]any{"count": float64(42)},
		[]byte(`{"count": 99}`),
		"$.request.message",
		contract.MatchingRules{
			"$.request.message.count": {Match: contract.MatchTypeValue},
		},
	)
	if !result.Success {
		t.Errorf("expected success with type matcher on request path, got errors: %v", result.Errors)
	}
}

func TestValidator_Validate_StatusMatchNoMetadataNoMessage(t *testing.T) {
	t.Helper()
	// Minimal interaction: status matches, no metadata, no message.
	interaction := grpcInteraction(func(p *contract.GRPCPayload) {
		p.Response.Status = "OK"
	})
	v := NewValidator()
	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status: "OK",
	})
	if !result.Success {
		t.Errorf("expected success for minimal matching interaction, got errors: %v", result.Errors)
	}
}
