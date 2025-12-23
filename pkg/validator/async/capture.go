package async

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// MessageCapture captures messages published by a producer for verification.
type MessageCapture struct {
	interactions []contract.Interaction
	validator    *Validator

	mu       sync.RWMutex
	captured []CapturedMessage
	matched  map[string]int
}

// CapturedMessage represents a captured published message.
type CapturedMessage struct {
	Destination string
	Headers     map[string]string
	Payload     []byte
	Matched     bool
	MatchedID   string
	Errors      []validator.ValidationError
}

// NewMessageCapture creates a new message capture instance.
func NewMessageCapture(interactions []contract.Interaction) *MessageCapture {
	return &MessageCapture{
		interactions: interactions,
		validator:    NewValidator(),
		matched:      make(map[string]int),
	}
}

// Capture records a message published by the producer.
func (c *MessageCapture) Capture(destination string, headers map[string]string, payload []byte) {
	captured := CapturedMessage{
		Destination: destination,
		Headers:     headers,
		Payload:     payload,
	}

	// Try to match against interactions
	for i := range c.interactions {
		interaction := &c.interactions[i]
		if interaction.Payload.Async == nil {
			continue
		}

		async := interaction.Payload.Async

		// Only match publish direction
		if async.Direction != contract.AsyncDirectionPublish {
			continue
		}

		// Check destination matches
		if async.Destination != destination {
			continue
		}

		// Validate the message
		result := c.validator.ValidateMessage(interaction, payload, headers)
		if result.Success {
			captured.Matched = true
			captured.MatchedID = interaction.ID
			c.mu.Lock()
			c.matched[interaction.ID]++
			c.mu.Unlock()
			break
		} else {
			captured.Errors = result.Errors
		}
	}

	c.mu.Lock()
	c.captured = append(c.captured, captured)
	c.mu.Unlock()
}

// CapturedMessages returns all captured messages.
func (c *MessageCapture) CapturedMessages() []CapturedMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]CapturedMessage, len(c.captured))
	copy(result, c.captured)
	return result
}

// Verify checks that all publish interactions were matched.
func (c *MessageCapture) Verify() validator.ValidationResult {
	result := validator.NewSuccessResult()

	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check all publish interactions were captured
	for _, interaction := range c.interactions {
		if interaction.Payload.Async == nil {
			continue
		}
		if interaction.Payload.Async.Direction != contract.AsyncDirectionPublish {
			continue
		}

		count := c.matched[interaction.ID]
		if count == 0 {
			result.AddError(&validator.ValidationError{
				Path:    interaction.ID,
				Message: fmt.Sprintf("expected message %q was never captured", interaction.Description),
			})
		}
	}

	// Check for unmatched captures
	for _, msg := range c.captured {
		if !msg.Matched {
			result.AddError(&validator.ValidationError{
				Path:    msg.Destination,
				Message: fmt.Sprintf("captured message to %s did not match any interaction", msg.Destination),
			})
		}
	}

	return result
}

// Reset clears all captured messages.
func (c *MessageCapture) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.captured = nil
	c.matched = make(map[string]int)
}

// ProducerSpy wraps a message producer to capture messages.
type ProducerSpy struct {
	capture *MessageCapture
	send    func(destination string, headers map[string]string, payload []byte) error
}

// NewProducerSpy creates a spy that captures messages while forwarding them.
func NewProducerSpy(capture *MessageCapture, sendFunc func(string, map[string]string, []byte) error) *ProducerSpy {
	return &ProducerSpy{
		capture: capture,
		send:    sendFunc,
	}
}

// Send captures the message and optionally forwards it.
func (s *ProducerSpy) Send(destination string, headers map[string]string, payload []byte) error {
	// Capture the message
	s.capture.Capture(destination, headers, payload)

	// Forward if send function is provided
	if s.send != nil {
		return s.send(destination, headers, payload)
	}
	return nil
}

// InMemoryBroker is a simple in-memory message broker for testing.
type InMemoryBroker struct {
	mu       sync.RWMutex
	queues   map[string][]BrokerMessage
	handlers map[string][]MessageHandler
}

// BrokerMessage represents a message in the broker.
type BrokerMessage struct {
	Headers map[string]string
	Payload []byte
}

// MessageHandler handles received messages.
type MessageHandler func(headers map[string]string, payload []byte) error

// NewInMemoryBroker creates a new in-memory broker.
func NewInMemoryBroker() *InMemoryBroker {
	return &InMemoryBroker{
		queues:   make(map[string][]BrokerMessage),
		handlers: make(map[string][]MessageHandler),
	}
}

// Publish publishes a message to a destination.
func (b *InMemoryBroker) Publish(destination string, headers map[string]string, payload []byte) error {
	b.mu.Lock()
	b.queues[destination] = append(b.queues[destination], BrokerMessage{
		Headers: headers,
		Payload: payload,
	})
	handlers := b.handlers[destination]
	b.mu.Unlock()

	// Notify handlers
	for _, handler := range handlers {
		if err := handler(headers, payload); err != nil {
			return err
		}
	}

	return nil
}

// Subscribe registers a handler for a destination.
func (b *InMemoryBroker) Subscribe(destination string, handler MessageHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[destination] = append(b.handlers[destination], handler)
}

// Messages returns all messages for a destination.
func (b *InMemoryBroker) Messages(destination string) []BrokerMessage {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queues[destination]
}

// Clear clears all messages.
func (b *InMemoryBroker) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.queues = make(map[string][]BrokerMessage)
}

// Transport defines the interface for async message transports.
type Transport interface {
	// Publish sends a message to a destination.
	Publish(ctx interface{}, destination string, headers map[string]string, payload []byte) error

	// Subscribe registers a handler for messages on a destination.
	Subscribe(destination string, handler func(headers map[string]string, payload []byte) error) error

	// Close closes the transport.
	Close() error
}

// TransportAdapter wraps a Transport with capture capabilities.
type TransportAdapter struct {
	transport Transport
	capture   *MessageCapture
}

// NewTransportAdapter creates an adapter that captures messages.
func NewTransportAdapter(transport Transport, capture *MessageCapture) *TransportAdapter {
	return &TransportAdapter{
		transport: transport,
		capture:   capture,
	}
}

// Publish captures and forwards messages.
func (a *TransportAdapter) Publish(ctx interface{}, destination string, headers map[string]string, payload []byte) error {
	// Capture
	a.capture.Capture(destination, headers, payload)

	// Forward
	return a.transport.Publish(ctx, destination, headers, payload)
}

// ToJSON converts a value to JSON bytes.
// Returns nil if marshaling fails.
func ToJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
