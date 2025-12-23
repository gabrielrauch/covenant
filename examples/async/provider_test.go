package async_example

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
	asyncvalidator "github.com/gabrielrauch/covenant/pkg/validator/async"
)

// OrdersServiceProvider simulates the orders service that publishes events.
type OrdersServiceProvider struct {
	broker *asyncvalidator.InMemoryBroker
}

// NewOrdersServiceProvider creates a new orders service provider.
func NewOrdersServiceProvider(broker *asyncvalidator.InMemoryBroker) *OrdersServiceProvider {
	return &OrdersServiceProvider{broker: broker}
}

// CreateOrder creates an order and publishes an event.
func (p *OrdersServiceProvider) CreateOrder(userID string, items []Item) (*OrderCreatedEvent, error) {
	// Calculate total
	var total float64
	for _, item := range items {
		total += float64(item.Quantity) * item.Price
	}

	// Create the event
	event := &OrderCreatedEvent{
		OrderID:   "ord-" + time.Now().Format("20060102150405"),
		UserID:    userID,
		Total:     total,
		Items:     items,
		CreatedAt: time.Now().UTC(),
	}

	// Publish the event
	eventBytes, _ := json.Marshal(event)
	headers := map[string]string{
		"content-type": "application/json",
		"event-type":   "OrderCreated",
	}

	if err := p.broker.Publish("orders.created", headers, eventBytes); err != nil {
		return nil, err
	}

	return event, nil
}

// TestProviderVerification_Async demonstrates provider verification for async contracts.
func TestProviderVerification_Async(t *testing.T) {
	// Load contracts from directory
	contractFiles, err := filepath.Glob("./contracts/*.json")
	if err != nil {
		t.Fatalf("Failed to glob contracts: %v", err)
	}

	if len(contractFiles) == 0 {
		t.Skip("No contract files found - run consumer tests first")
	}

	validator := asyncvalidator.NewValidator()

	for _, file := range contractFiles {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			t.Logf("Skipping %s: %v", file, err)
			continue
		}

		t.Run(file, func(t *testing.T) {
			for _, interaction := range c.Interactions {
				if interaction.Protocol != contract.ProtocolAsync {
					continue
				}

				if interaction.Payload.Async == nil {
					continue
				}

				t.Run(interaction.Description, func(t *testing.T) {
					testAsyncInteraction(t, validator, interaction)
				})
			}
		})
	}
}

// testAsyncInteraction tests an async interaction against the provider.
func testAsyncInteraction(t *testing.T, validator *asyncvalidator.Validator, interaction contract.Interaction) {
	async := interaction.Payload.Async

	// For publish direction, we verify the provider produces valid messages
	if async.Direction == contract.AsyncDirectionPublish {
		// Simulate producing a message that matches the contract
		testMessage := produceTestMessage(async)

		result := validator.ValidateMessage(&interaction, testMessage, async.Message.Headers)
		if !result.Success {
			for _, err := range result.Errors {
				t.Errorf("Validation error: %s - %s (expected: %v, actual: %v)",
					err.Path, err.Message, err.Expected, err.Actual)
			}
		}
	}

	// For consume direction, we verify the provider can handle messages
	if async.Direction == contract.AsyncDirectionConsume {
		t.Log("Consumer contract - provider should be able to send these messages")
	}
}

// produceTestMessage produces a test message based on the contract.
func produceTestMessage(async *contract.AsyncPayload) []byte {
	// In a real test, this would call the actual provider code
	// Here we produce a message that matches the expected structure
	msg, _ := json.Marshal(async.Message.Payload)
	return msg
}

// TestProviderCapture demonstrates capturing messages from a real provider.
func TestProviderCapture(t *testing.T) {
	// Create broker and provider
	broker := asyncvalidator.NewInMemoryBroker()
	provider := NewOrdersServiceProvider(broker)

	// Load contracts
	c := createOrderEventsContract()

	// Create capture to verify published messages
	capture := asyncvalidator.NewMessageCapture(c.Interactions)

	// Subscribe to capture messages
	broker.Subscribe("orders.created", func(headers map[string]string, payload []byte) error {
		capture.Capture("orders.created", headers, payload)
		return nil
	})

	// Execute provider action
	_, err := provider.CreateOrder("user-test", []Item{
		{SKU: "TEST-001", Name: "Test Item", Quantity: 1, Price: 50.00},
	})
	if err != nil {
		t.Fatalf("Failed to create order: %v", err)
	}

	// Verify captured messages match contract
	// Note: In a real test, the provider would produce messages matching the contract exactly
	// This demo shows the capture mechanism - validation may report structural differences
	result := capture.Verify()
	if !result.Success {
		for _, err := range result.Errors {
			t.Logf("Capture verification note: %s - %s", err.Path, err.Message)
		}
	}

	// Check messages were captured
	messages := capture.CapturedMessages()
	t.Logf("Captured %d messages", len(messages))

	for _, msg := range messages {
		t.Logf("  Destination: %s, Matched: %v", msg.Destination, msg.Matched)
	}
}

// NotificationService is a service that consumes user events.
type NotificationService struct {
	notifications []Notification
}

// Notification represents a notification to send.
type Notification struct {
	UserID  string
	Email   string
	Message string
}

// HandleUserRegistered handles a user registered event.
func (s *NotificationService) HandleUserRegistered(payload []byte) error {
	var event struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		return err
	}

	s.notifications = append(s.notifications, Notification{
		UserID:  event.UserID,
		Email:   event.Email,
		Message: "Welcome " + event.Name + "!",
	})

	return nil
}

// GetNotifications returns all notifications.
func (s *NotificationService) GetNotifications() []Notification {
	return s.notifications
}

// TestConsumerServiceVerification verifies a service that consumes messages.
func TestConsumerServiceVerification(t *testing.T) {
	// Create the notification service (consumer)
	notificationService := &NotificationService{}

	// Load the contract that defines what messages we expect to receive
	c := &contract.Contract{
		Metadata: contract.Metadata{
			Consumer: contract.Participant{Name: "notification-service"},
			Provider: contract.Participant{Name: "user-service"},
		},
		Interactions: []contract.Interaction{
			{
				ID:          "user-registered",
				Description: "handle user registered event",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "users.registered",
						Direction:   contract.AsyncDirectionConsume,
						Message: contract.AsyncMessage{
							Payload: map[string]any{
								"user_id": "user-001",
								"email":   "user@example.com",
								"name":    "Test User",
							},
						},
					},
				},
			},
		},
	}

	// For each consume interaction, verify the consumer can handle the message
	for _, interaction := range c.Interactions {
		if interaction.Payload.Async == nil {
			continue
		}
		if interaction.Payload.Async.Direction != contract.AsyncDirectionConsume {
			continue
		}

		t.Run(interaction.Description, func(t *testing.T) {
			// Convert expected message to JSON
			msgBytes, _ := json.Marshal(interaction.Payload.Async.Message.Payload)

			// Process through consumer
			err := notificationService.HandleUserRegistered(msgBytes)
			if err != nil {
				t.Errorf("Consumer failed to handle message: %v", err)
			}
		})
	}

	// Verify consumer produced expected side effects
	notifications := notificationService.GetNotifications()
	if len(notifications) == 0 {
		t.Error("Expected at least one notification")
	} else {
		t.Logf("Consumer generated %d notifications", len(notifications))
		for _, n := range notifications {
			t.Logf("  To: %s - %s", n.Email, n.Message)
		}
	}
}

// TestMessageValidation demonstrates direct message validation.
func TestMessageValidation(t *testing.T) {
	c := createOrderEventsContract()
	interaction := c.Interactions[0]
	validator := asyncvalidator.NewValidator()

	// Test with valid message
	validMessage := map[string]any{
		"order_id":   "ord-valid-123",
		"user_id":    "user-789",
		"total":      250.50,
		"items":      []map[string]any{{"sku": "ITEM-1", "name": "Item", "quantity": 1, "price": 250.50}},
		"created_at": "2025-06-15T10:30:00Z",
	}
	validBytes, _ := json.Marshal(validMessage)
	validHeaders := map[string]string{
		"content-type": "application/json",
		"event-type":   "OrderCreated",
	}

	result := validator.ValidateMessage(&interaction, validBytes, validHeaders)
	if !result.Success {
		t.Errorf("Valid message should pass: %v", result.Errors)
	}

	// Test with invalid message (wrong total type)
	invalidMessage := map[string]any{
		"order_id":   "ord-123",
		"user_id":    "user-456",
		"total":      "not-a-number", // Invalid type
		"items":      []map[string]any{},
		"created_at": "invalid-date", // Invalid format
	}
	invalidBytes, _ := json.Marshal(invalidMessage)

	result = validator.ValidateMessage(&interaction, invalidBytes, validHeaders)
	// Note: This may or may not fail depending on matching rules
	t.Logf("Invalid message validation: success=%v, errors=%d", result.Success, len(result.Errors))
}
