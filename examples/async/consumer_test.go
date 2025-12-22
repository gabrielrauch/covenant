// Package async_example demonstrates async messaging contract testing with covenant.
package async_example

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
	asyncvalidator "github.com/gabrielrauch/covenant/pkg/validator/async"
	"github.com/google/uuid"
)

// OrderCreatedEvent represents an order created event.
type OrderCreatedEvent struct {
	OrderID   string    `json:"order_id"`
	UserID    string    `json:"user_id"`
	Total     float64   `json:"total"`
	Items     []Item    `json:"items"`
	CreatedAt time.Time `json:"created_at"`
}

// Item represents an order item.
type Item struct {
	SKU      string  `json:"sku"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
}

// createOrderEventsContract creates a contract for order events.
func createOrderEventsContract() *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "inventory-service"},
			Provider:  contract.Participant{Name: "orders-service"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "order-created-event",
				Description: "order created event",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "orders.created",
						Direction:   contract.AsyncDirectionPublish,
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"content-type": "application/json",
								"event-type":   "OrderCreated",
							},
							Payload: map[string]any{
								"order_id": "ord-123",
								"user_id":  "user-456",
								"total":    99.99,
								"items": []map[string]any{
									{
										"sku":      "WIDGET-001",
										"name":     "Widget",
										"quantity": 2,
										"price":    49.99,
									},
								},
								"created_at": "2025-01-01T00:00:00Z",
							},
						},
					},
				},
				MatchingRules: contract.MatchingRules{
					"$.message.payload.order_id":   {Match: contract.MatchType_},
					"$.message.payload.user_id":    {Match: contract.MatchType_},
					"$.message.payload.total":      {Match: contract.MatchDecimal},
					"$.message.payload.created_at": {Match: contract.MatchRegex, Pattern: `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`},
				},
			},
		},
	}
}

// TestOrderCreatedEvent_Consumer demonstrates a consumer expecting an async event.
func TestOrderCreatedEvent_Consumer(t *testing.T) {
	c := createOrderEventsContract()

	// Create an in-memory broker for testing
	broker := asyncvalidator.NewInMemoryBroker()

	// Create message capture to verify expected messages
	capture := asyncvalidator.NewMessageCapture(c.Interactions)

	// Subscribe to the orders.created topic as a consumer
	var receivedEvent OrderCreatedEvent
	broker.Subscribe("orders.created", func(headers map[string]string, payload []byte) error {
		// Capture the message for contract verification
		capture.Capture("orders.created", headers, payload)

		// Parse the event
		if err := json.Unmarshal(payload, &receivedEvent); err != nil {
			t.Errorf("Failed to parse event: %v", err)
			return err
		}
		return nil
	})

	// Simulate the producer sending an event
	event := OrderCreatedEvent{
		OrderID:   "ord-789",
		UserID:    "user-123",
		Total:     149.99,
		CreatedAt: time.Now().UTC(),
		Items: []Item{
			{SKU: "GADGET-001", Name: "Gadget", Quantity: 1, Price: 149.99},
		},
	}
	eventBytes, _ := json.Marshal(event)

	headers := map[string]string{
		"content-type": "application/json",
		"event-type":   "OrderCreated",
	}

	// Publish the event
	if err := broker.Publish("orders.created", headers, eventBytes); err != nil {
		t.Fatalf("Failed to publish event: %v", err)
	}

	// Verify the message was received and matches contract
	result := capture.Verify()
	if !result.Success {
		for _, err := range result.Errors {
			t.Logf("Validation note: %s - %s", err.Path, err.Message)
		}
	}

	// Verify consumer processed the event correctly
	if receivedEvent.OrderID != "ord-789" {
		t.Errorf("Expected order ID ord-789, got %s", receivedEvent.OrderID)
	}

	// Save contract
	if err := contract.SaveToFile(c, "./contracts/inventory-orders-order-created.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}

// createPaymentSagaContract creates a contract for a payment saga sequence.
func createPaymentSagaContract() *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "payment-service"},
			Provider:  contract.Participant{Name: "order-orchestrator"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "payment-saga",
				Description: "payment processing saga",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "payments.process",
						Direction:   contract.AsyncDirectionConsume,
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"correlation-id": "saga-123",
							},
							Payload: map[string]any{
								"order_id": "ord-001",
								"amount":   100.00,
								"currency": "USD",
							},
						},
						Sequence: &contract.AsyncSequence{
							Name: "payment-flow",
							Messages: []contract.AsyncSeqMessage{
								{
									Order:       1,
									Destination: "payments.process",
									Payload: map[string]any{
										"order_id": "ord-001",
										"amount":   100.00,
										"currency": "USD",
									},
									Capture: map[string]string{
										"order_id": "$.order_id",
									},
								},
								{
									Order:       2,
									Destination: "payments.confirmed",
									Payload: map[string]any{
										"order_id":      "ord-001",
										"payment_id":    "pay-001",
										"status":        "confirmed",
										"processed_at":  "2025-01-01T00:00:00Z",
									},
								},
								{
									Order:       3,
									Destination: "orders.paid",
									Payload: map[string]any{
										"order_id": "ord-001",
										"status":   "paid",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestPaymentSaga_Consumer demonstrates saga sequence validation.
func TestPaymentSaga_Consumer(t *testing.T) {
	c := createPaymentSagaContract()
	interaction := c.Interactions[0]

	// Create sequence validator
	seqValidator := asyncvalidator.NewSequenceValidator(interaction.Payload.Async.Sequence)

	// Simulate saga message flow

	// Step 1: Payment process request
	msg1, _ := json.Marshal(map[string]any{
		"order_id": "ord-001",
		"amount":   100.00,
		"currency": "USD",
	})
	seqValidator.ValidateNext("payments.process", msg1, map[string]string{
		"correlation-id": "saga-123",
	})

	// Check captured values
	captured := seqValidator.Captured()
	if orderId, ok := captured["order_id"].(string); ok {
		t.Logf("Captured order_id: %s", orderId)
	}

	// Step 2: Payment confirmed
	msg2, _ := json.Marshal(map[string]any{
		"order_id":     "ord-001",
		"payment_id":   "pay-xyz",
		"status":       "confirmed",
		"processed_at": "2025-01-01T12:00:00Z",
	})
	seqValidator.ValidateNext("payments.confirmed", msg2, nil)

	// Step 3: Order marked as paid
	msg3, _ := json.Marshal(map[string]any{
		"order_id": "ord-001",
		"status":   "paid",
	})
	seqValidator.ValidateNext("orders.paid", msg3, nil)

	// Get final result
	result := seqValidator.Result()
	if !result.Success {
		for _, err := range result.Errors {
			t.Errorf("Saga validation error: %s - %s", err.Path, err.Message)
		}
	}

	// Save contract
	if err := contract.SaveToFile(c, "./contracts/payment-order-saga.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}

// TestProducerSpy demonstrates using a producer spy to capture messages.
func TestProducerSpy(t *testing.T) {
	// Define what messages we expect the producer to publish
	c := &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "notification-service"},
			Provider:  contract.Participant{Name: "user-service"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "user-registered",
				Description: "user registered event",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "users.registered",
						Direction:   contract.AsyncDirectionPublish,
						Message: contract.AsyncMessage{
							Headers: map[string]string{
								"event-type": "UserRegistered",
							},
							Payload: map[string]any{
								"user_id": "user-001",
								"email":   "user@example.com",
								"name":    "Test User",
							},
						},
					},
				},
				MatchingRules: contract.MatchingRules{
					"$.message.payload.user_id": {Match: contract.MatchType_},
					"$.message.payload.email":   {Match: contract.MatchRegex, Pattern: `^[^@]+@[^@]+\.[^@]+$`},
				},
			},
		},
	}

	// Create capture for contract interactions
	capture := asyncvalidator.NewMessageCapture(c.Interactions)

	// Create a producer spy that captures messages
	spy := asyncvalidator.NewProducerSpy(capture, nil) // nil = don't actually send

	// Simulate the producer code publishing a message
	eventPayload := asyncvalidator.ToJSON(map[string]any{
		"user_id": "user-new-123",
		"email":   "newuser@example.com",
		"name":    "New User",
	})

	headers := map[string]string{
		"event-type": "UserRegistered",
	}

	// Send through the spy (gets captured)
	if err := spy.Send("users.registered", headers, eventPayload); err != nil {
		t.Fatalf("Failed to send: %v", err)
	}

	// Verify all expected messages were captured
	result := capture.Verify()
	if !result.Success {
		for _, err := range result.Errors {
			t.Errorf("Capture verification failed: %s - %s", err.Path, err.Message)
		}
	}

	// Inspect captured messages
	messages := capture.CapturedMessages()
	if len(messages) != 1 {
		t.Errorf("Expected 1 captured message, got %d", len(messages))
	}

	if messages[0].Matched {
		t.Log("Message matched contract successfully")
	}

	// Save contract
	if err := contract.SaveToFile(c, "./contracts/notification-user-registered.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}
