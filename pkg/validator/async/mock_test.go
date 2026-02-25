package async

import (
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// newConsumeInteraction creates a consume interaction for testing.
func newConsumeInteraction(id, description, destination string, payload map[string]any, headers map[string]string) contract.Interaction {
	return contract.Interaction{
		ID:          id,
		Description: description,
		Payload: contract.Payload{
			Async: &contract.AsyncPayload{
				Direction:   contract.AsyncDirectionConsume,
				Destination: destination,
				Message: contract.AsyncMessage{
					Payload: payload,
					Headers: headers,
				},
			},
		},
	}
}

// newPublishInteraction creates a publish interaction for testing.
func newPublishInteraction(id, description, destination string, payload map[string]any) contract.Interaction {
	return contract.Interaction{
		ID:          id,
		Description: description,
		Payload: contract.Payload{
			Async: &contract.AsyncPayload{
				Direction:   contract.AsyncDirectionPublish,
				Destination: destination,
				Message: contract.AsyncMessage{
					Payload: payload,
				},
			},
		},
	}
}

// nonAsyncInteraction creates an interaction with no async payload.
func nonAsyncInteraction(id string) contract.Interaction {
	return contract.Interaction{
		ID:      id,
		Payload: contract.Payload{},
	}
}

func TestNewMockSource(t *testing.T) {
	interactions := []contract.Interaction{
		newConsumeInteraction("c1", "consume event", "queue.a", map[string]any{"k": "v"}, nil),
	}

	mock := NewMockSource(interactions)
	if mock == nil {
		t.Fatal("NewMockSource returned nil")
	}
	if mock.validator == nil {
		t.Error("Validator not initialized")
	}
	if mock.matchedDelivery == nil {
		t.Error("matchedDelivery map not initialized")
	}
	if len(mock.interactions) != 1 {
		t.Errorf("Expected 1 interaction, got %d", len(mock.interactions))
	}
}

//nolint:gocyclo // test function
func TestMockSource_GetMessage(t *testing.T) {
	tests := []struct {
		name         string
		interactions []contract.Interaction
		destination  string
		wantErr      bool
		errContains  string
		checkMsg     func(t *testing.T, msg *DeliveredMessage)
	}{
		{
			name: "matching consume interaction",
			interactions: []contract.Interaction{
				newConsumeInteraction("order-1", "order created", "orders.created",
					map[string]any{"order_id": "123", "total": 42.5}, nil),
			},
			destination: "orders.created",
			wantErr:     false,
			checkMsg: func(t *testing.T, msg *DeliveredMessage) {
				t.Helper()
				if msg.Destination != "orders.created" {
					t.Errorf("Destination = %s, want orders.created", msg.Destination)
				}
				if msg.Interaction != "order-1" {
					t.Errorf("Interaction = %s, want order-1", msg.Interaction)
				}
				if !strings.Contains(string(msg.Payload), `"order_id"`) {
					t.Errorf("Payload missing order_id: %s", msg.Payload)
				}
			},
		},
		{
			name: "no matching destination",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "desc", "queue.a", map[string]any{}, nil),
			},
			destination: "queue.b",
			wantErr:     true,
			errContains: "no mock message for destination",
		},
		{
			name: "skips non-async interaction",
			interactions: []contract.Interaction{
				nonAsyncInteraction("no-async"),
				newConsumeInteraction("c1", "desc", "queue.a", map[string]any{"k": "v"}, nil),
			},
			destination: "queue.a",
			wantErr:     false,
			checkMsg: func(t *testing.T, msg *DeliveredMessage) {
				t.Helper()
				if msg.Interaction != "c1" {
					t.Errorf("Interaction = %s, want c1", msg.Interaction)
				}
			},
		},
		{
			name: "skips publish direction",
			interactions: []contract.Interaction{
				newPublishInteraction("pub-1", "publish event", "events.topic", map[string]any{"x": 1}),
			},
			destination: "events.topic",
			wantErr:     true,
			errContains: "no mock message for destination",
		},
		{
			name: "returns first matching interaction",
			interactions: []contract.Interaction{
				newConsumeInteraction("first", "first match", "queue",
					map[string]any{"id": "first"}, nil),
				newConsumeInteraction("second", "second match", "queue",
					map[string]any{"id": "second"}, nil),
			},
			destination: "queue",
			wantErr:     false,
			checkMsg: func(t *testing.T, msg *DeliveredMessage) {
				t.Helper()
				if msg.Interaction != "first" {
					t.Errorf("Interaction = %s, want first", msg.Interaction)
				}
			},
		},
		{
			name: "with headers",
			interactions: []contract.Interaction{
				newConsumeInteraction("h1", "with headers", "queue",
					map[string]any{"v": 1},
					map[string]string{"content-type": "application/json", "x-trace": "abc"}),
			},
			destination: "queue",
			wantErr:     false,
			checkMsg: func(t *testing.T, msg *DeliveredMessage) {
				t.Helper()
				if msg.Headers["content-type"] != "application/json" {
					t.Errorf("content-type header = %s, want application/json", msg.Headers["content-type"])
				}
				if msg.Headers["x-trace"] != "abc" {
					t.Errorf("x-trace header = %s, want abc", msg.Headers["x-trace"])
				}
			},
		},
		{
			name:         "empty interactions",
			interactions: []contract.Interaction{},
			destination:  "any",
			wantErr:      true,
			errContains:  "no mock message for destination",
		},
		{
			name: "marshals payload to JSON bytes",
			interactions: []contract.Interaction{
				newConsumeInteraction("m1", "marshal test", "queue",
					map[string]any{
						"nested": map[string]any{"a": "b"},
						"count":  float64(5),
					}, nil),
			},
			destination: "queue",
			wantErr:     false,
			checkMsg: func(t *testing.T, msg *DeliveredMessage) {
				t.Helper()
				payload := string(msg.Payload)
				if !strings.Contains(payload, `"nested"`) {
					t.Errorf("Payload missing nested: %s", payload)
				}
				if !strings.Contains(payload, `"a":"b"`) && !strings.Contains(payload, `"a": "b"`) {
					t.Errorf("Payload missing nested value: %s", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockSource(tt.interactions)
			msg, err := mock.GetMessage(tt.destination)

			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Error = %q, want containing %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			if msg == nil {
				t.Fatal("Expected message, got nil")
			}
			if tt.checkMsg != nil {
				tt.checkMsg(t, msg)
			}
		})
	}
}

func TestMockSource_GetMessage_TracksDelivery(t *testing.T) {
	interactions := []contract.Interaction{
		newConsumeInteraction("track-1", "tracked", "queue", map[string]any{"k": "v"}, nil),
	}

	mock := NewMockSource(interactions)

	// First call
	_, err := mock.GetMessage("queue")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	mock.mu.RLock()
	count := mock.matchedDelivery["track-1"]
	deliveredCount := len(mock.deliveredMsgs)
	mock.mu.RUnlock()

	if count != 1 {
		t.Errorf("matchedDelivery count = %d, want 1", count)
	}
	if deliveredCount != 1 {
		t.Errorf("deliveredMsgs count = %d, want 1", deliveredCount)
	}

	// Second call increments
	_, err = mock.GetMessage("queue")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	mock.mu.RLock()
	count = mock.matchedDelivery["track-1"]
	deliveredCount = len(mock.deliveredMsgs)
	mock.mu.RUnlock()

	if count != 2 {
		t.Errorf("matchedDelivery count = %d, want 2", count)
	}
	if deliveredCount != 2 {
		t.Errorf("deliveredMsgs count = %d, want 2", deliveredCount)
	}
}

func TestMockSource_GetMessages(t *testing.T) {
	tests := []struct {
		name         string
		interactions []contract.Interaction
		destination  string
		wantCount    int
		checkMsgs    func(t *testing.T, msgs []DeliveredMessage)
	}{
		{
			name: "returns all matching consume interactions",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "first", "events", map[string]any{"id": "1"}, nil),
				newConsumeInteraction("c2", "second", "events", map[string]any{"id": "2"}, nil),
			},
			destination: "events",
			wantCount:   2,
			checkMsgs: func(t *testing.T, msgs []DeliveredMessage) {
				t.Helper()
				if msgs[0].Interaction != "c1" {
					t.Errorf("msgs[0].Interaction = %s, want c1", msgs[0].Interaction)
				}
				if msgs[1].Interaction != "c2" {
					t.Errorf("msgs[1].Interaction = %s, want c2", msgs[1].Interaction)
				}
			},
		},
		{
			name: "skips non-async interactions",
			interactions: []contract.Interaction{
				nonAsyncInteraction("skip-me"),
				newConsumeInteraction("keep", "keep this", "queue", map[string]any{}, nil),
			},
			destination: "queue",
			wantCount:   1,
		},
		{
			name: "skips publish direction",
			interactions: []contract.Interaction{
				newPublishInteraction("pub", "publish", "events", map[string]any{}),
				newConsumeInteraction("con", "consume", "events", map[string]any{}, nil),
			},
			destination: "events",
			wantCount:   1,
			checkMsgs: func(t *testing.T, msgs []DeliveredMessage) {
				t.Helper()
				if msgs[0].Interaction != "con" {
					t.Errorf("Interaction = %s, want con", msgs[0].Interaction)
				}
			},
		},
		{
			name: "skips non-matching destination",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "desc", "queue.a", map[string]any{}, nil),
				newConsumeInteraction("c2", "desc", "queue.b", map[string]any{}, nil),
			},
			destination: "queue.a",
			wantCount:   1,
			checkMsgs: func(t *testing.T, msgs []DeliveredMessage) {
				t.Helper()
				if msgs[0].Interaction != "c1" {
					t.Errorf("Interaction = %s, want c1", msgs[0].Interaction)
				}
			},
		},
		{
			name:         "empty interactions returns empty slice",
			interactions: []contract.Interaction{},
			destination:  "any",
			wantCount:    0,
		},
		{
			name: "no matching destination returns empty slice",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "desc", "queue.x", map[string]any{}, nil),
			},
			destination: "queue.y",
			wantCount:   0,
		},
		{
			name: "sets destination and headers on returned messages",
			interactions: []contract.Interaction{
				newConsumeInteraction("h1", "headers", "queue",
					map[string]any{"data": "test"},
					map[string]string{"x-key": "val"}),
			},
			destination: "queue",
			wantCount:   1,
			checkMsgs: func(t *testing.T, msgs []DeliveredMessage) {
				t.Helper()
				if msgs[0].Destination != "queue" {
					t.Errorf("Destination = %s, want queue", msgs[0].Destination)
				}
				if msgs[0].Headers["x-key"] != "val" {
					t.Errorf("Header x-key = %s, want val", msgs[0].Headers["x-key"])
				}
			},
		},
		{
			name: "marshals payload to JSON",
			interactions: []contract.Interaction{
				newConsumeInteraction("m1", "marshal", "queue",
					map[string]any{"name": "test"}, nil),
			},
			destination: "queue",
			wantCount:   1,
			checkMsgs: func(t *testing.T, msgs []DeliveredMessage) {
				t.Helper()
				payload := string(msgs[0].Payload)
				if !strings.Contains(payload, `"name"`) {
					t.Errorf("Payload missing name field: %s", payload)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockSource(tt.interactions)
			msgs := mock.GetMessages(tt.destination)

			if len(msgs) != tt.wantCount {
				t.Fatalf("GetMessages returned %d messages, want %d", len(msgs), tt.wantCount)
			}
			if tt.checkMsgs != nil {
				tt.checkMsgs(t, msgs)
			}
		})
	}
}

func TestMockSource_Verify(t *testing.T) {
	tests := []struct {
		name         string
		interactions []contract.Interaction
		deliver      []string // destinations to call GetMessage on
		wantSuccess  bool
		wantErrPath  string // expected error path if failure
		wantErrMsg   string // expected substring in error message
	}{
		{
			name: "all consume interactions delivered",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "event A", "queue.a", map[string]any{"a": 1}, nil),
				newConsumeInteraction("c2", "event B", "queue.b", map[string]any{"b": 2}, nil),
			},
			deliver:     []string{"queue.a", "queue.b"},
			wantSuccess: true,
		},
		{
			name: "undelivered consume interaction",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "event A", "queue.a", map[string]any{"a": 1}, nil),
				newConsumeInteraction("c2", "event B", "queue.b", map[string]any{"b": 2}, nil),
			},
			deliver:     []string{"queue.a"},
			wantSuccess: false,
			wantErrPath: "c2",
			wantErrMsg:  "event B",
		},
		{
			name: "no consume interactions",
			interactions: []contract.Interaction{
				newPublishInteraction("pub1", "publish event", "topic", map[string]any{}),
			},
			deliver:     nil,
			wantSuccess: true,
		},
		{
			name: "skips non-async interactions",
			interactions: []contract.Interaction{
				nonAsyncInteraction("no-async"),
			},
			deliver:     nil,
			wantSuccess: true,
		},
		{
			name: "skips publish direction in verify",
			interactions: []contract.Interaction{
				newPublishInteraction("pub1", "publish event", "events", map[string]any{}),
				newConsumeInteraction("c1", "consume event", "events", map[string]any{"x": 1}, nil),
			},
			deliver:     []string{"events"},
			wantSuccess: true,
		},
		{
			name:         "empty interactions passes",
			interactions: []contract.Interaction{},
			deliver:      nil,
			wantSuccess:  true,
		},
		{
			name: "no deliveries with consume interactions fails",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "undelivered msg", "queue", map[string]any{}, nil),
			},
			deliver:     nil,
			wantSuccess: false,
			wantErrPath: "c1",
			wantErrMsg:  "undelivered msg",
		},
		{
			name: "multiple undelivered interactions produce multiple errors",
			interactions: []contract.Interaction{
				newConsumeInteraction("c1", "first undelivered", "q1", map[string]any{}, nil),
				newConsumeInteraction("c2", "second undelivered", "q2", map[string]any{}, nil),
			},
			deliver:     nil,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockSource(tt.interactions)

			for _, dest := range tt.deliver {
				if _, err := mock.GetMessage(dest); err != nil {
					t.Fatalf("GetMessage(%s) failed: %v", dest, err)
				}
			}

			result := mock.Verify()

			if result.Success != tt.wantSuccess {
				t.Errorf("Verify() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}

			if !tt.wantSuccess && tt.wantErrPath != "" {
				found := false
				for _, e := range result.Errors {
					if e.Path == tt.wantErrPath {
						found = true
						if tt.wantErrMsg != "" && !strings.Contains(e.Message, tt.wantErrMsg) {
							t.Errorf("Error message = %q, want containing %q", e.Message, tt.wantErrMsg)
						}
					}
				}
				if !found {
					t.Errorf("Expected error with path %q, got errors: %v", tt.wantErrPath, result.Errors)
				}
			}

			if !tt.wantSuccess && tt.name == "multiple undelivered interactions produce multiple errors" {
				if len(result.Errors) < 2 {
					t.Errorf("Expected at least 2 errors, got %d", len(result.Errors))
				}
			}
		})
	}
}

func TestMockSource_Reset(t *testing.T) {
	interactions := []contract.Interaction{
		newConsumeInteraction("c1", "event", "queue", map[string]any{"k": "v"}, nil),
	}

	mock := NewMockSource(interactions)

	// Deliver a message
	_, err := mock.GetMessage("queue")
	if err != nil {
		t.Fatalf("GetMessage failed: %v", err)
	}

	// Verify delivery was tracked
	mock.mu.RLock()
	if len(mock.deliveredMsgs) != 1 {
		t.Errorf("Expected 1 delivered message before reset, got %d", len(mock.deliveredMsgs))
	}
	if mock.matchedDelivery["c1"] != 1 {
		t.Errorf("Expected matchedDelivery[c1]=1, got %d", mock.matchedDelivery["c1"])
	}
	mock.mu.RUnlock()

	// Reset
	mock.Reset()

	// Verify state cleared
	mock.mu.RLock()
	if len(mock.deliveredMsgs) != 0 {
		t.Errorf("Expected 0 delivered messages after reset, got %d", len(mock.deliveredMsgs))
	}
	if len(mock.matchedDelivery) != 0 {
		t.Errorf("Expected empty matchedDelivery after reset, got %d entries", len(mock.matchedDelivery))
	}
	mock.mu.RUnlock()

	// Verify that Verify() now reports failures for undelivered interactions
	result := mock.Verify()
	if result.Success {
		t.Error("Expected Verify to fail after Reset with undelivered interactions")
	}
}

func TestMockSource_Reset_AllowsRedelivery(t *testing.T) {
	interactions := []contract.Interaction{
		newConsumeInteraction("c1", "event", "queue", map[string]any{"k": "v"}, nil),
	}

	mock := NewMockSource(interactions)

	// Deliver, verify, reset, redeliver
	_, err := mock.GetMessage("queue")
	if err != nil {
		t.Fatalf("First GetMessage failed: %v", err)
	}

	result := mock.Verify()
	if !result.Success {
		t.Errorf("Expected success before reset, errors: %v", result.Errors)
	}

	mock.Reset()

	_, err = mock.GetMessage("queue")
	if err != nil {
		t.Fatalf("GetMessage after reset failed: %v", err)
	}

	result = mock.Verify()
	if !result.Success {
		t.Errorf("Expected success after redeliver, errors: %v", result.Errors)
	}
}

func TestDeliveredMessage_Fields(t *testing.T) {
	msg := DeliveredMessage{
		Destination: "orders.created",
		Headers:     map[string]string{"content-type": "application/json"},
		Payload:     []byte(`{"order_id": "123"}`),
		Interaction: "interaction-1",
	}

	if msg.Destination != "orders.created" {
		t.Errorf("Destination = %s, want orders.created", msg.Destination)
	}
	if msg.Headers["content-type"] != "application/json" {
		t.Errorf("content-type = %s, want application/json", msg.Headers["content-type"])
	}
	if string(msg.Payload) != `{"order_id": "123"}` {
		t.Errorf("Payload = %s, want {\"order_id\": \"123\"}", msg.Payload)
	}
	if msg.Interaction != "interaction-1" {
		t.Errorf("Interaction = %s, want interaction-1", msg.Interaction)
	}
}
