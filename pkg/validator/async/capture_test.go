package async

import (
	"bytes"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestNewMessageCapture(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "test-1",
			Payload: contract.Payload{
				Async: &contract.AsyncPayload{
					Direction:   contract.AsyncDirectionPublish,
					Destination: "test.queue",
				},
			},
		},
	}

	capture := NewMessageCapture(interactions)
	if capture == nil {
		t.Fatal("NewMessageCapture returned nil")
	}
	if capture.validator == nil {
		t.Error("Validator not initialized")
	}
	if capture.matched == nil {
		t.Error("Matched map not initialized")
	}
}

func TestMessageCapture_Capture(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID:          "order-created",
			Description: "order created event",
			Payload: contract.Payload{
				Async: &contract.AsyncPayload{
					Direction:   contract.AsyncDirectionPublish,
					Destination: "orders.created",
					Message: contract.AsyncMessage{
						Payload: map[string]any{
							"order_id": "123",
						},
					},
				},
			},
		},
	}

	capture := NewMessageCapture(interactions)

	t.Run("matching message", func(t *testing.T) {
		capture.Capture("orders.created", nil, []byte(`{"order_id": "123"}`))

		messages := capture.CapturedMessages()
		if len(messages) != 1 {
			t.Fatalf("Expected 1 captured message, got %d", len(messages))
		}
		if !messages[0].Matched {
			t.Error("Message should be matched")
		}
		if messages[0].MatchedID != "order-created" {
			t.Errorf("MatchedID = %s, want order-created", messages[0].MatchedID)
		}
	})

	t.Run("non-matching destination", func(t *testing.T) {
		capture.Reset()
		capture.Capture("unknown.queue", nil, []byte(`{"data": "test"}`))

		messages := capture.CapturedMessages()
		if len(messages) != 1 {
			t.Fatalf("Expected 1 captured message, got %d", len(messages))
		}
		if messages[0].Matched {
			t.Error("Message should not be matched")
		}
	})
}

func TestMessageCapture_CapturedMessages(t *testing.T) {
	capture := NewMessageCapture(nil)

	// Capture some messages
	capture.Capture("queue1", nil, []byte(`{}`))
	capture.Capture("queue2", nil, []byte(`{}`))

	messages := capture.CapturedMessages()
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}

	// Verify it returns a copy
	messages[0].Destination = "modified"
	original := capture.CapturedMessages()
	if original[0].Destination == "modified" {
		t.Error("CapturedMessages should return a copy")
	}
}

func TestMessageCapture_Verify(t *testing.T) {
	t.Run("all matched", func(t *testing.T) {
		interactions := []contract.Interaction{
			{
				ID: "event-1",
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Direction:   contract.AsyncDirectionPublish,
						Destination: "events",
						Message: contract.AsyncMessage{
							Payload: map[string]any{"type": "test"},
						},
					},
				},
			},
		}

		capture := NewMessageCapture(interactions)
		capture.Capture("events", nil, []byte(`{"type": "test"}`))

		result := capture.Verify()
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})

	t.Run("unmatched interaction", func(t *testing.T) {
		interactions := []contract.Interaction{
			{
				ID:          "expected",
				Description: "expected event",
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Direction:   contract.AsyncDirectionPublish,
						Destination: "events",
					},
				},
			},
		}

		capture := NewMessageCapture(interactions)
		// No messages captured

		result := capture.Verify()
		if result.Success {
			t.Error("Expected failure for unmatched interaction")
		}
	})

	t.Run("unmatched capture", func(t *testing.T) {
		interactions := []contract.Interaction{}

		capture := NewMessageCapture(interactions)
		capture.Capture("unknown", nil, []byte(`{}`))

		result := capture.Verify()
		if result.Success {
			t.Error("Expected failure for unmatched capture")
		}
	})

	t.Run("ignores consume direction", func(t *testing.T) {
		interactions := []contract.Interaction{
			{
				ID: "consume-event",
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Direction:   contract.AsyncDirectionConsume,
						Destination: "events",
					},
				},
			},
		}

		capture := NewMessageCapture(interactions)
		// No messages captured - consume interactions don't need capture

		result := capture.Verify()
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})
}

func TestMessageCapture_Reset(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "test",
			Payload: contract.Payload{
				Async: &contract.AsyncPayload{
					Direction:   contract.AsyncDirectionPublish,
					Destination: "queue",
					Message: contract.AsyncMessage{
						Payload: map[string]any{},
					},
				},
			},
		},
	}

	capture := NewMessageCapture(interactions)
	capture.Capture("queue", nil, []byte(`{}`))

	if len(capture.CapturedMessages()) != 1 {
		t.Error("Expected 1 message before reset")
	}

	capture.Reset()

	if len(capture.CapturedMessages()) != 0 {
		t.Error("Expected 0 messages after reset")
	}
}

func TestNewProducerSpy(t *testing.T) {
	capture := NewMessageCapture(nil)
	var called bool
	sendFunc := func(dest string, headers map[string]string, payload []byte) error {
		called = true
		return nil
	}

	spy := NewProducerSpy(capture, sendFunc)
	if spy == nil {
		t.Fatal("NewProducerSpy returned nil")
	}

	err := spy.Send("test", nil, []byte(`{}`))
	if err != nil {
		t.Errorf("Send failed: %v", err)
	}
	if !called {
		t.Error("Send function was not called")
	}
	if len(capture.CapturedMessages()) != 1 {
		t.Error("Message was not captured")
	}
}

func TestProducerSpy_Send(t *testing.T) {
	t.Run("with send function", func(t *testing.T) {
		capture := NewMessageCapture(nil)
		var receivedDest string
		var receivedPayload []byte

		spy := NewProducerSpy(capture, func(dest string, headers map[string]string, payload []byte) error {
			receivedDest = dest
			receivedPayload = payload
			return nil
		})

		err := spy.Send("my-queue", nil, []byte(`{"data": 1}`))
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
		if receivedDest != "my-queue" {
			t.Errorf("Destination = %s, want my-queue", receivedDest)
		}
		if string(receivedPayload) != `{"data": 1}` {
			t.Errorf("Payload = %s, want {\"data\": 1}", receivedPayload)
		}
	})

	t.Run("without send function", func(t *testing.T) {
		capture := NewMessageCapture(nil)
		spy := NewProducerSpy(capture, nil)

		err := spy.Send("queue", nil, []byte(`{}`))
		if err != nil {
			t.Errorf("Send failed: %v", err)
		}
		// Message should still be captured
		if len(capture.CapturedMessages()) != 1 {
			t.Error("Message was not captured")
		}
	})
}

func TestNewInMemoryBroker(t *testing.T) {
	broker := NewInMemoryBroker()
	if broker == nil {
		t.Fatal("NewInMemoryBroker returned nil")
	}
	if broker.queues == nil {
		t.Error("Queues map not initialized")
	}
	if broker.handlers == nil {
		t.Error("Handlers map not initialized")
	}
}

func TestInMemoryBroker_Publish(t *testing.T) {
	broker := NewInMemoryBroker()

	headers := map[string]string{"content-type": "application/json"}
	payload := []byte(`{"event": "test"}`)

	err := broker.Publish("events", headers, payload)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	messages := broker.Messages("events")
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if !bytes.Equal(messages[0].Payload, payload) {
		t.Error("Payload mismatch")
	}
	if messages[0].Headers["content-type"] != "application/json" {
		t.Error("Headers mismatch")
	}
}

func TestInMemoryBroker_Subscribe(t *testing.T) {
	broker := NewInMemoryBroker()

	var received []byte
	broker.Subscribe("events", func(headers map[string]string, payload []byte) error {
		received = payload
		return nil
	})

	err := broker.Publish("events", nil, []byte(`{"data": "test"}`))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if string(received) != `{"data": "test"}` {
		t.Errorf("Received = %s, want {\"data\": \"test\"}", received)
	}
}

func TestInMemoryBroker_Messages(t *testing.T) {
	broker := NewInMemoryBroker()

	// Empty queue
	messages := broker.Messages("empty")
	if len(messages) != 0 {
		t.Error("Expected empty slice for empty queue")
	}

	// With messages
	if err := broker.Publish("queue", nil, []byte(`{}`)); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := broker.Publish("queue", nil, []byte(`{}`)); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	messages = broker.Messages("queue")
	if len(messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(messages))
	}
}

func TestInMemoryBroker_Clear(t *testing.T) {
	broker := NewInMemoryBroker()

	if err := broker.Publish("queue1", nil, []byte(`{}`)); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
	if err := broker.Publish("queue2", nil, []byte(`{}`)); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	broker.Clear()

	if len(broker.Messages("queue1")) != 0 {
		t.Error("queue1 should be empty after clear")
	}
	if len(broker.Messages("queue2")) != 0 {
		t.Error("queue2 should be empty after clear")
	}
}

func TestNewTransportAdapter(t *testing.T) {
	capture := NewMessageCapture(nil)
	// Create a mock transport using InMemoryBroker as base
	broker := NewInMemoryBroker()
	mockTransport := &mockTransport{broker: broker}

	adapter := NewTransportAdapter(mockTransport, capture)
	if adapter == nil {
		t.Fatal("NewTransportAdapter returned nil")
	}
}

func TestTransportAdapter_Publish(t *testing.T) {
	capture := NewMessageCapture(nil)
	broker := NewInMemoryBroker()
	mockTransport := &mockTransport{broker: broker}

	adapter := NewTransportAdapter(mockTransport, capture)

	err := adapter.Publish(nil, "events", nil, []byte(`{"event": "test"}`))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Check message was captured
	if len(capture.CapturedMessages()) != 1 {
		t.Error("Message was not captured")
	}

	// Check message was forwarded
	if len(broker.Messages("events")) != 1 {
		t.Error("Message was not forwarded")
	}
}

func TestToJSON(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		result := ToJSON(map[string]any{"key": "value"})
		if string(result) != `{"key":"value"}` {
			t.Errorf("ToJSON = %s, want {\"key\":\"value\"}", result)
		}
	})

	t.Run("struct", func(t *testing.T) {
		type testStruct struct {
			Name string `json:"name"`
		}
		result := ToJSON(testStruct{Name: "test"})
		if string(result) != `{"name":"test"}` {
			t.Errorf("ToJSON = %s, want {\"name\":\"test\"}", result)
		}
	})
}

func TestToJSON_Unmarshalable(t *testing.T) {
	// Channels cannot be marshaled to JSON
	result := ToJSON(make(chan int))
	if result != nil {
		t.Errorf("expected nil for unmarshalable value, got %s", result)
	}
}

func TestMessageCapture_NilAsyncPayload(t *testing.T) {
	interactions := []contract.Interaction{
		{ID: "no-async", Payload: contract.Payload{}},
	}
	capture := NewMessageCapture(interactions)
	capture.Capture("any", nil, []byte(`{}`))

	msgs := capture.CapturedMessages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Matched {
		t.Error("message should not match interaction without async payload")
	}
}

// mockTransport implements Transport for testing
type mockTransport struct {
	broker *InMemoryBroker
}

func (m *mockTransport) Publish(ctx interface{}, destination string, headers map[string]string, payload []byte) error {
	return m.broker.Publish(destination, headers, payload)
}

func (m *mockTransport) Subscribe(destination string, handler func(headers map[string]string, payload []byte) error) error {
	m.broker.Subscribe(destination, handler)
	return nil
}

func (m *mockTransport) Close() error {
	return nil
}
