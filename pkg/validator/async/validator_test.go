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
			result := v.Validate(context.Background(), tt.interaction, tt.actual)

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
			result := v.ValidateMessage(tt.interaction, tt.message, tt.headers)

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
			"$.message.payload",
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
			"$.message.payload",
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
			"$.message.payload",
			contract.MatchingRules{
				"$.message.payload.id": {Match: contract.MatchType_},
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
