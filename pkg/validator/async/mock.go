package async

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// MockSource provides mock messages to consumers during testing.
type MockSource struct {
	interactions []contract.Interaction
	validator    *Validator

	mu              sync.RWMutex
	deliveredMsgs   []DeliveredMessage
	matchedDelivery map[string]int
}

// DeliveredMessage represents a message delivered to a consumer.
type DeliveredMessage struct {
	Destination string
	Headers     map[string]string
	Payload     []byte
	Interaction string // ID of the interaction
}

// NewMockSource creates a new mock message source.
func NewMockSource(interactions []contract.Interaction) *MockSource {
	return &MockSource{
		interactions:    interactions,
		validator:       NewValidator(),
		matchedDelivery: make(map[string]int),
	}
}

// GetMessage returns a mock message for the given destination.
func (m *MockSource) GetMessage(destination string) (*DeliveredMessage, error) {
	for _, interaction := range m.interactions {
		if interaction.Payload.Async == nil {
			continue
		}

		async := interaction.Payload.Async
		if async.Destination != destination {
			continue
		}

		// Only provide messages for consume direction
		if async.Direction != contract.AsyncDirectionConsume {
			continue
		}

		// Build message
		payload, _ := json.Marshal(async.Message.Payload)
		msg := &DeliveredMessage{
			Destination: destination,
			Headers:     async.Message.Headers,
			Payload:     payload,
			Interaction: interaction.ID,
		}

		m.mu.Lock()
		m.deliveredMsgs = append(m.deliveredMsgs, *msg)
		m.matchedDelivery[interaction.ID]++
		m.mu.Unlock()

		return msg, nil
	}

	return nil, fmt.Errorf("no mock message for destination: %s", destination)
}

// GetMessages returns all mock messages for a destination.
func (m *MockSource) GetMessages(destination string) []DeliveredMessage {
	var messages []DeliveredMessage

	for _, interaction := range m.interactions {
		if interaction.Payload.Async == nil {
			continue
		}

		async := interaction.Payload.Async
		if async.Destination != destination || async.Direction != contract.AsyncDirectionConsume {
			continue
		}

		payload, _ := json.Marshal(async.Message.Payload)
		messages = append(messages, DeliveredMessage{
			Destination: destination,
			Headers:     async.Message.Headers,
			Payload:     payload,
			Interaction: interaction.ID,
		})
	}

	return messages
}

// Verify checks that all consume interactions were delivered.
func (m *MockSource) Verify() validator.ValidationResult {
	result := validator.NewSuccessResult()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, interaction := range m.interactions {
		if interaction.Payload.Async == nil {
			continue
		}
		if interaction.Payload.Async.Direction != contract.AsyncDirectionConsume {
			continue
		}

		count := m.matchedDelivery[interaction.ID]
		if count == 0 {
			result.AddError(validator.ValidationError{
				Path:    interaction.ID,
				Message: fmt.Sprintf("message %q was never delivered", interaction.Description),
			})
		}
	}

	return result
}

// Reset clears delivery tracking.
func (m *MockSource) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deliveredMsgs = nil
	m.matchedDelivery = make(map[string]int)
}
