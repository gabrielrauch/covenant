package grpc

import (
	"github.com/gabrielrauch/covenant/pkg/contract"
)

// StreamRecorder records messages in a streaming interaction for validation.
type StreamRecorder struct {
	messages []StreamMessage
}

// StreamMessage represents a recorded stream message.
type StreamMessage struct {
	Direction contract.StreamDirection
	Data      []byte
	Metadata  map[string]string
}

// NewStreamRecorder creates a new stream recorder.
func NewStreamRecorder() *StreamRecorder {
	return &StreamRecorder{}
}

// RecordClientMessage records a client-sent message.
func (r *StreamRecorder) RecordClientMessage(data []byte, metadata map[string]string) {
	r.messages = append(r.messages, StreamMessage{
		Direction: contract.StreamDirectionClient,
		Data:      data,
		Metadata:  metadata,
	})
}

// RecordServerMessage records a server-sent message.
func (r *StreamRecorder) RecordServerMessage(data []byte, metadata map[string]string) {
	r.messages = append(r.messages, StreamMessage{
		Direction: contract.StreamDirectionServer,
		Data:      data,
		Metadata:  metadata,
	})
}

// Messages returns all recorded messages.
func (r *StreamRecorder) Messages() []StreamMessage {
	return r.messages
}

// ClientMessages returns only client-sent messages.
func (r *StreamRecorder) ClientMessages() []StreamMessage {
	var result []StreamMessage
	for _, msg := range r.messages {
		if msg.Direction == contract.StreamDirectionClient {
			result = append(result, msg)
		}
	}
	return result
}

// ServerMessages returns only server-sent messages.
func (r *StreamRecorder) ServerMessages() []StreamMessage {
	var result []StreamMessage
	for _, msg := range r.messages {
		if msg.Direction == contract.StreamDirectionServer {
			result = append(result, msg)
		}
	}
	return result
}

// Count returns the total number of messages.
func (r *StreamRecorder) Count() int {
	return len(r.messages)
}

// StreamMatcher helps match expected stream sequences against actual.
type StreamMatcher struct {
	expected []contract.StreamMessage
	actual   []StreamMessage
}

// NewStreamMatcher creates a matcher for stream sequences.
func NewStreamMatcher(expected []contract.StreamMessage) *StreamMatcher {
	return &StreamMatcher{
		expected: expected,
	}
}

// AddActual adds an actual message to match.
func (m *StreamMatcher) AddActual(msg StreamMessage) {
	m.actual = append(m.actual, msg)
}

// Match checks if the actual sequence matches the expected.
func (m *StreamMatcher) Match() (bool, []string) {
	var errors []string

	if len(m.actual) != len(m.expected) {
		errors = append(errors,
			"sequence length mismatch: expected %d messages, got %d")
	}

	minLen := len(m.expected)
	if len(m.actual) < minLen {
		minLen = len(m.actual)
	}

	for i := 0; i < minLen; i++ {
		exp := m.expected[i]
		act := m.actual[i]

		if exp.Direction != act.Direction {
			errors = append(errors,
				"message %d: expected %s, got %s")
		}
	}

	return len(errors) == 0, errors
}

// StreamExpectation defines expectations for a streaming interaction.
type StreamExpectation struct {
	MinMessages int
	MaxMessages int
	Ordered     bool
}

// ValidateCount checks if the message count is within bounds.
func (e *StreamExpectation) ValidateCount(count int) bool {
	if e.MinMessages > 0 && count < e.MinMessages {
		return false
	}
	if e.MaxMessages > 0 && count > e.MaxMessages {
		return false
	}
	return true
}
