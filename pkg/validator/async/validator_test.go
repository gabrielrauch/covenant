package async

import (
	"context"
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
			name: "no async payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			actual:      validator.ActualData{},
			wantSuccess: false,
		},
		{
			name: "valid message",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "orders.created",
						Message: contract.AsyncMessage{
							Payload: map[string]any{
								"order_id": "123",
								"total":    100,
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Response: []byte(`{"order_id": "123", "total": 100}`),
			},
			wantSuccess: true,
		},
		{
			name: "header validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"content-type": "application/json",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: map[string]string{
					"content-type": "application/json",
				},
			},
			wantSuccess: true,
		},
		{
			name: "header mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"content-type": "application/json",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: map[string]string{
					"content-type": "text/plain",
				},
			},
			wantSuccess: false,
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

func TestValidator_ValidateMessage(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		message     []byte
		headers     map[string]string
		wantSuccess bool
	}{
		{
			name: "no async payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			message:     []byte(`{}`),
			headers:     nil,
			wantSuccess: false,
		},
		{
			name: "valid message payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{
								"event": "order_created",
								"data": map[string]any{
									"id": "123",
								},
							},
						},
					},
				},
			},
			message:     []byte(`{"event": "order_created", "data": {"id": "123"}}`),
			headers:     nil,
			wantSuccess: true,
		},
		{
			name: "with headers",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"x-correlation-id": "abc123",
							},
							Payload: map[string]any{"event": "test"},
						},
					},
				},
			},
			message: []byte(`{"event": "test"}`),
			headers: map[string]string{
				"x-correlation-id": "abc123",
			},
			wantSuccess: true,
		},
		{
			name: "payload mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{
								"expected": "value",
							},
						},
					},
				},
			},
			message:     []byte(`{"different": "data"}`),
			headers:     nil,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			result := v.ValidateMessage(&tt.interaction, tt.message, tt.headers)

			if result.Success != tt.wantSuccess {
				t.Errorf("ValidateMessage() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}
		})
	}
}

func TestValidator_validatePayload(t *testing.T) {
	v := NewValidator()

	t.Run("invalid JSON", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"key": "value"},
			[]byte("not valid json"),
			nil,
		)
		if result.Success {
			t.Error("Expected failure for invalid JSON")
		}
	})

	t.Run("valid payload", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"id": "123"},
			[]byte(`{"id": "123"}`),
			nil,
		)
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})

	t.Run("with matching rules", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"id": "example"},
			[]byte(`{"id": "different"}`),
			contract.MatchingRules{
				"$.message.payload.id": {Match: contract.MatchTypeValue},
			},
		)
		if !result.Success {
			t.Errorf("Expected success with type matcher, got errors: %v", result.Errors)
		}
	})
}

func TestNewSequenceValidator(t *testing.T) {
	sequence := &contract.AsyncSequence{
		Messages: []contract.AsyncSeqMessage{
			{Destination: "orders", Payload: map[string]any{}},
		},
	}

	sv := NewSequenceValidator(sequence)
	if sv == nil {
		t.Fatal("NewSequenceValidator returned nil")
	}
	if sv.sequence != sequence {
		t.Error("Sequence not set correctly")
	}
}

func TestSequenceValidator_ValidateNext(t *testing.T) {
	t.Run("valid sequence", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "orders.created",
					Payload:     map[string]any{"order_id": "123"},
				},
				{
					Destination: "inventory.reserve",
					Payload:     map[string]any{"item_id": "456"},
				},
			},
		}

		sv := NewSequenceValidator(sequence)

		// Validate first message
		sv.ValidateNext("orders.created", []byte(`{"order_id": "123"}`), nil)
		// Validate second message
		sv.ValidateNext("inventory.reserve", []byte(`{"item_id": "456"}`), nil)

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})

	t.Run("destination mismatch", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{Destination: "expected.queue", Payload: map[string]any{}},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("wrong.queue", []byte(`{}`), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for destination mismatch")
		}
	})

	t.Run("extra message", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{Destination: "queue", Payload: map[string]any{}},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte(`{}`), nil)
		sv.ValidateNext("queue", []byte(`{}`), nil) // Extra message

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for extra message")
		}
	})

	t.Run("incomplete sequence", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{Destination: "queue1", Payload: map[string]any{}},
				{Destination: "queue2", Payload: map[string]any{}},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue1", []byte(`{}`), nil)
		// Missing second message

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for incomplete sequence")
		}
	})

	t.Run("with capture", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "orders",
					Payload:     map[string]any{"id": "123"},
					Capture: map[string]string{
						"order_id": "$.id",
					},
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("orders", []byte(`{"id": "order-123"}`), nil)

		captured := sv.Captured()
		if captured["order_id"] != "order-123" {
			t.Errorf("Captured order_id = %v, want order-123", captured["order_id"])
		}
	})

	t.Run("invalid JSON payload", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{Destination: "queue", Payload: map[string]any{}},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte("not json"), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for invalid JSON")
		}
	})
}

func TestSequenceValidator_Captured(t *testing.T) {
	sequence := &contract.AsyncSequence{
		Messages: []contract.AsyncSeqMessage{},
	}

	sv := NewSequenceValidator(sequence)
	captured := sv.Captured()

	if captured == nil {
		t.Error("Captured should not be nil")
	}
	if len(captured) != 0 {
		t.Error("Captured should be empty initially")
	}
}

func TestSequenceValidator_Result(t *testing.T) {
	sequence := &contract.AsyncSequence{
		Messages: []contract.AsyncSeqMessage{},
	}

	sv := NewSequenceValidator(sequence)
	result := sv.Result()

	if !result.Success {
		t.Errorf("Expected success for empty sequence, got errors: %v", result.Errors)
	}
}

// --- Additional tests for uncovered branches ---

func TestValidator_Validate_NilPayloadHandling(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		actual      validator.ActualData
		wantSuccess bool
		wantErrors  int
	}{
		{
			name: "nil actual response with non-nil expected payload skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{"key": "value"},
						},
					},
				},
			},
			actual: validator.ActualData{
				Response: nil, // nil response
			},
			wantSuccess: true,
		},
		{
			name: "empty actual response with non-nil expected payload skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{"key": "value"},
						},
					},
				},
			},
			actual: validator.ActualData{
				Response: []byte{}, // empty response
			},
			wantSuccess: true,
		},
		{
			name: "nil expected payload with non-empty response skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: nil,
						},
					},
				},
			},
			actual: validator.ActualData{
				Response: []byte(`{"data": "value"}`),
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

func TestValidator_Validate_MetadataEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		actual      validator.ActualData
		wantSuccess bool
	}{
		{
			name: "nil actual metadata with expected headers succeeds (ValidateHeaders returns nil for nil actual)",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"content-type": "application/json",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: nil,
			},
			wantSuccess: true,
		},
		{
			name: "empty expected headers with actual metadata succeeds",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: map[string]string{
					"content-type": "application/json",
				},
			},
			wantSuccess: true,
		},
		{
			name: "missing header in actual metadata fails",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"x-custom-header": "expected-value",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: map[string]string{},
			},
			wantSuccess: false,
		},
		{
			name: "multiple header mismatches produce multiple errors",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"x-header-a": "value-a",
								"x-header-b": "value-b",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Metadata: map[string]string{
					"x-header-a": "wrong-a",
					"x-header-b": "wrong-b",
				},
			},
			wantSuccess: false,
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

func TestValidator_Validate_HeaderAndPayloadCombined(t *testing.T) {
	t.Run("header error combined with payload success", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				Async: &contract.AsyncPayload{
					Message: contract.AsyncMessage{
						Headers: map[string]string{
							"content-type": "application/json",
						},
						Payload: map[string]any{"id": "123"},
					},
				},
			},
		}
		actual := validator.ActualData{
			Metadata: map[string]string{
				"content-type": "text/plain",
			},
			Response: []byte(`{"id": "123"}`),
		}

		v := NewValidator()
		result := v.Validate(context.Background(), &interaction, actual)
		if result.Success {
			t.Error("expected failure due to header mismatch")
		}
		if len(result.Errors) == 0 {
			t.Error("expected at least one error for header mismatch")
		}
	})

	t.Run("header success combined with payload error", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				Async: &contract.AsyncPayload{
					Message: contract.AsyncMessage{
						Headers: map[string]string{
							"content-type": "application/json",
						},
						Payload: map[string]any{"expected_field": "value"},
					},
				},
			},
		}
		actual := validator.ActualData{
			Metadata: map[string]string{
				"content-type": "application/json",
			},
			Response: []byte(`{"different_field": "value"}`),
		}

		v := NewValidator()
		result := v.Validate(context.Background(), &interaction, actual)
		if result.Success {
			t.Error("expected failure due to payload mismatch")
		}
	})
}

func TestValidator_ValidateMessage_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		message     []byte
		headers     map[string]string
		wantSuccess bool
	}{
		{
			name: "nil message with non-nil expected payload skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{"key": "value"},
						},
					},
				},
			},
			message:     nil,
			headers:     nil,
			wantSuccess: true,
		},
		{
			name: "empty message with non-nil expected payload skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: map[string]any{"key": "value"},
						},
					},
				},
			},
			message:     []byte{},
			headers:     nil,
			wantSuccess: true,
		},
		{
			name: "nil expected payload with non-empty message skips payload validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Payload: nil,
						},
					},
				},
			},
			message:     []byte(`{"data": "something"}`),
			headers:     nil,
			wantSuccess: true,
		},
		{
			name: "empty headers map in interaction with nil actual headers succeeds",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: map[string]string{},
						},
					},
				},
			},
			message:     []byte(`{}`),
			headers:     nil,
			wantSuccess: true,
		},
		{
			name: "nil headers map in interaction with actual headers succeeds",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Message: contract.AsyncMessage{
							Headers: nil,
							Payload: map[string]any{"x": 1},
						},
					},
				},
			},
			message: []byte(`{"x": 1}`),
			headers: map[string]string{
				"x-trace": "abc",
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewValidator()
			result := v.ValidateMessage(&tt.interaction, tt.message, tt.headers)
			if result.Success != tt.wantSuccess {
				t.Errorf("ValidateMessage() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}
		})
	}
}

func TestValidator_validatePayload_MatchingRuleEdgeCases(t *testing.T) {
	v := NewValidator()

	t.Run("matching rules not under basePath are ignored", func(t *testing.T) {
		// Rules with paths that do NOT start with "$.message.payload" should be ignored
		result := v.validatePayload(
			map[string]any{"id": "123"},
			[]byte(`{"id": "123"}`),
			contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
		)
		if !result.Success {
			t.Errorf("Expected success (unrelated rules ignored), got errors: %v", result.Errors)
		}
	})

	t.Run("matching rule path exactly at basePath boundary", func(t *testing.T) {
		// Path shorter than basePath should not cause issues
		result := v.validatePayload(
			map[string]any{"id": "test"},
			[]byte(`{"id": "different"}`),
			contract.MatchingRules{
				"$.message": {Match: contract.MatchTypeValue},
			},
		)
		// The rule path is shorter than basePath, so it should be filtered out.
		// Structural matching will still check types but not values.
		if result.Success {
			t.Logf("Result: success=%v, errors=%v", result.Success, result.Errors)
		}
	})

	t.Run("type matching rule allows different values of same type", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"name": "example", "count": float64(10)},
			[]byte(`{"name": "different", "count": 42}`),
			contract.MatchingRules{
				"$.message.payload.name":  {Match: contract.MatchTypeValue},
				"$.message.payload.count": {Match: contract.MatchTypeValue},
			},
		)
		if !result.Success {
			t.Errorf("Expected success with type matchers, got errors: %v", result.Errors)
		}
	})

	t.Run("structural mismatch with missing field", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"required_field": "value", "other_field": "data"},
			[]byte(`{"required_field": "value"}`),
			nil,
		)
		if result.Success {
			t.Error("Expected failure for missing field in actual payload")
		}
		// Verify the error mentions the missing field
		found := false
		for _, e := range result.Errors {
			if e.Path == "$.message.payload.other_field" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected error for path $.message.payload.other_field, got: %v", result.Errors)
		}
	})

	t.Run("type mismatch in nested payload", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"data": map[string]any{"count": float64(5)}},
			[]byte(`{"data": {"count": "five"}}`),
			nil,
		)
		if result.Success {
			t.Error("Expected failure for type mismatch (number vs string)")
		}
	})

	t.Run("empty expected and empty actual payload", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{},
			[]byte(`{}`),
			nil,
		)
		if !result.Success {
			t.Errorf("Expected success for empty payloads, got errors: %v", result.Errors)
		}
	})

	t.Run("actual payload is a JSON array instead of object", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{"key": "value"},
			[]byte(`[1, 2, 3]`),
			nil,
		)
		if result.Success {
			t.Error("Expected failure for array vs object type mismatch")
		}
	})

	t.Run("nested JSON payload", func(t *testing.T) {
		result := v.validatePayload(
			map[string]any{
				"order": map[string]any{
					"id":    "o-123",
					"items": []any{map[string]any{"sku": "A1"}},
				},
			},
			[]byte(`{"order": {"id": "o-123", "items": [{"sku": "A1"}]}}`),
			nil,
		)
		if !result.Success {
			t.Errorf("Expected success for nested matching payload, got errors: %v", result.Errors)
		}
	})
}

func TestSequenceValidator_ValidateNext_EdgeCases(t *testing.T) {
	t.Run("nil expected payload skips structure validation", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "queue",
					Payload:     nil, // nil payload
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte(`{"any": "data"}`), nil)

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success when expected payload is nil, got errors: %v", result.Errors)
		}
	})

	t.Run("capture with invalid path does not fail", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "queue",
					Payload:     map[string]any{"id": "123"},
					Capture: map[string]string{
						"missing_val": "$.nonexistent.deep.path",
					},
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte(`{"id": "123"}`), nil)

		captured := sv.Captured()
		if _, exists := captured["missing_val"]; exists {
			t.Error("Expected missing_val to not be captured for invalid path")
		}

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success despite capture failure, got errors: %v", result.Errors)
		}
	})

	t.Run("payload structure mismatch in sequence message", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "queue",
					Payload:     map[string]any{"expected_key": "value"},
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte(`{"different_key": "value"}`), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for payload structure mismatch in sequence")
		}
	})

	t.Run("multiple messages with mixed success", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "queue-a",
					Payload:     map[string]any{"id": "1"},
				},
				{
					Destination: "queue-b",
					Payload:     map[string]any{"id": "2"},
				},
				{
					Destination: "queue-c",
					Payload:     map[string]any{"id": "3"},
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue-a", []byte(`{"id": "1"}`), nil)        // ok
		sv.ValidateNext("wrong-dest", []byte(`{"id": "2"}`), nil)     // destination mismatch
		sv.ValidateNext("queue-c", []byte(`{"wrong_key": "3"}`), nil) // payload mismatch

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for mixed errors")
		}
		if len(result.Errors) < 2 {
			t.Errorf("Expected at least 2 errors, got %d: %v", len(result.Errors), result.Errors)
		}
	})

	t.Run("extra messages beyond sequence length accumulate errors", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("queue", []byte(`{}`), nil)
		sv.ValidateNext("queue", []byte(`{}`), nil)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure for extra messages on empty sequence")
		}
		if len(result.Errors) != 2 {
			t.Errorf("Expected 2 errors (one per extra message), got %d", len(result.Errors))
		}
	})

	t.Run("capture multiple values across sequence", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{
					Destination: "orders",
					Payload:     map[string]any{"order_id": "abc"},
					Capture: map[string]string{
						"order_id": "$.order_id",
					},
				},
				{
					Destination: "payments",
					Payload:     map[string]any{"payment_id": "xyz"},
					Capture: map[string]string{
						"payment_id": "$.payment_id",
					},
				},
			},
		}

		sv := NewSequenceValidator(sequence)
		sv.ValidateNext("orders", []byte(`{"order_id": "order-001"}`), nil)
		sv.ValidateNext("payments", []byte(`{"payment_id": "pay-999"}`), nil)

		captured := sv.Captured()
		if captured["order_id"] != "order-001" {
			t.Errorf("Expected order_id=order-001, got %v", captured["order_id"])
		}
		if captured["payment_id"] != "pay-999" {
			t.Errorf("Expected payment_id=pay-999, got %v", captured["payment_id"])
		}

		result := sv.Result()
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})
}

func TestSequenceValidator_Result_IncompleteWithErrors(t *testing.T) {
	t.Run("incomplete sequence appends to existing errors", func(t *testing.T) {
		sequence := &contract.AsyncSequence{
			Messages: []contract.AsyncSeqMessage{
				{Destination: "queue-a", Payload: map[string]any{"id": "1"}},
				{Destination: "queue-b", Payload: map[string]any{"id": "2"}},
			},
		}

		sv := NewSequenceValidator(sequence)
		// Send first message with wrong destination (generates error)
		sv.ValidateNext("wrong-queue", []byte(`{"id": "1"}`), nil)
		// Don't send second message (incomplete)

		result := sv.Result()
		if result.Success {
			t.Error("Expected failure")
		}
		// Should have both destination mismatch error and incomplete error
		if len(result.Errors) < 2 {
			t.Errorf("Expected at least 2 errors (destination + incomplete), got %d: %v",
				len(result.Errors), result.Errors)
		}
	})
}
