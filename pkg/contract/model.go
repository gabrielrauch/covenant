// Package contract provides the core contract model for consumer-driven contract testing.
package contract

import (
	"encoding/json"
	"time"
)

// Status represents the lifecycle state of a contract.
type Status string

const (
	// StatusDraft indicates a mutable contract not used in CI verification.
	StatusDraft Status = "draft"
	// StatusPublished indicates an immutable, active verification target.
	StatusPublished Status = "published"
	// StatusDeprecated indicates a contract that is still verifiable but warns on new dependencies.
	StatusDeprecated Status = "deprecated"
)

// Protocol represents the communication protocol type.
type Protocol string

// Protocol constants for supported communication protocols.
const (
	ProtocolHTTP  Protocol = "http"
	ProtocolGRPC  Protocol = "grpc"
	ProtocolAsync Protocol = "async"
)

// Participant represents either a consumer or provider service.
type Participant struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ProviderState represents a precondition for an interaction.
type ProviderState struct {
	Name   string         `json:"name"`
	Params map[string]any `json:"params,omitempty"`
}

// Contract is the unified contract envelope containing metadata and interactions.
type Contract struct {
	Metadata      Metadata      `json:"metadata"`
	Interactions  []Interaction `json:"interactions"`
	MatchingRules MatchingRules `json:"matching_rules,omitempty"`
}

// Metadata contains contract identification and lifecycle information.
type Metadata struct {
	ID        string      `json:"id"`
	Version   string      `json:"version"`
	Consumer  Participant `json:"consumer"`
	Provider  Participant `json:"provider"`
	Status    Status      `json:"status"`
	Tags      []string    `json:"tags,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
	Checksum  string      `json:"checksum,omitempty"`
}

// Interaction represents a single consumer-provider interaction.
type Interaction struct {
	ID             string          `json:"id"`
	Description    string          `json:"description"`
	ProviderStates []ProviderState `json:"provider_states,omitempty"`
	Protocol       Protocol        `json:"protocol"`
	Payload        Payload         `json:"payload"`
	MatchingRules  MatchingRules   `json:"matching_rules,omitempty"`
}

// Payload holds protocol-specific interaction data.
// Only one of HTTP, GRPC, or Async should be set.
type Payload struct {
	HTTP  *HTTPPayload  `json:"http,omitempty"`
	GRPC  *GRPCPayload  `json:"grpc,omitempty"`
	Async *AsyncPayload `json:"async,omitempty"`
}

// HTTPPayload represents an HTTP request/response interaction.
type HTTPPayload struct {
	Request  HTTPRequest  `json:"request"`
	Response HTTPResponse `json:"response"`
}

// HTTPRequest represents an HTTP request.
type HTTPRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   map[string]string `json:"query,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

// HTTPResponse represents an HTTP response.
type HTTPResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    any               `json:"body,omitempty"`
}

// GRPCPayload represents a gRPC service interaction.
type GRPCPayload struct {
	Service    string          `json:"service"`
	Method     string          `json:"method"`
	Request    GRPCMessage     `json:"request"`
	Response   GRPCResponse    `json:"response"`
	StreamType StreamType      `json:"stream_type"`
	Sequence   []StreamMessage `json:"sequence,omitempty"`
}

// GRPCMessage represents a gRPC request message.
type GRPCMessage struct {
	Metadata map[string]string `json:"metadata,omitempty"`
	Message  any               `json:"message"`
}

// GRPCResponse represents a gRPC response.
type GRPCResponse struct {
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Message  any               `json:"message"`
}

// StreamType represents the type of gRPC streaming.
type StreamType string

// StreamType constants for gRPC streaming modes.
const (
	StreamTypeUnary         StreamType = "unary"
	StreamTypeServerStream  StreamType = "server_stream"
	StreamTypeClientStream  StreamType = "client_stream"
	StreamTypeBidirectional StreamType = "bidirectional"
)

// StreamMessage represents a message in a bidirectional stream sequence.
type StreamMessage struct {
	Direction StreamDirection `json:"direction"`
	Message   any             `json:"message"`
}

// StreamDirection indicates the direction of a stream message.
type StreamDirection string

// StreamDirection constants for message flow direction.
const (
	StreamDirectionClient StreamDirection = "client"
	StreamDirectionServer StreamDirection = "server"
)

// AsyncPayload represents an async messaging interaction.
type AsyncPayload struct {
	Destination string         `json:"destination"`
	Direction   AsyncDirection `json:"direction"`
	Message     AsyncMessage   `json:"message"`
	Sequence    *AsyncSequence `json:"sequence,omitempty"`
}

// AsyncDirection indicates whether a service publishes or consumes messages.
type AsyncDirection string

// AsyncDirection constants for async messaging roles.
const (
	AsyncDirectionPublish AsyncDirection = "publish"
	AsyncDirectionConsume AsyncDirection = "consume"
)

// AsyncMessage represents an async message.
type AsyncMessage struct {
	Headers  map[string]string `json:"headers,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Payload  any               `json:"payload"`
}

// AsyncSequence represents an ordered sequence of async messages for saga validation.
type AsyncSequence struct {
	Name       string            `json:"name"`
	Messages   []AsyncSeqMessage `json:"messages"`
	Validation []string          `json:"validation,omitempty"`
}

// AsyncSeqMessage represents a message in an async sequence.
type AsyncSeqMessage struct {
	Order       int               `json:"order"`
	Destination string            `json:"destination"`
	Payload     any               `json:"payload"`
	Capture     map[string]string `json:"capture,omitempty"`
}

// MatchingRules is a map of JSON paths to matching rule definitions.
type MatchingRules map[string]MatchingRule

// MatchingRule defines how values should be compared during verification.
type MatchingRule struct {
	Match   MatchType     `json:"match"`
	Pattern string        `json:"pattern,omitempty"`
	Value   any           `json:"value,omitempty"`
	Min     *int          `json:"min,omitempty"`
	Max     *int          `json:"max,omitempty"`
	Rule    *MatchingRule `json:"rule,omitempty"` // For null_or
}

// MatchType represents the type of matching to perform.
type MatchType string

// MatchType constants for contract matching strategies.
const (
	MatchExact     MatchType = "exact"
	MatchTypeValue MatchType = "type"
	MatchRegex     MatchType = "regex"
	MatchInteger   MatchType = "integer"
	MatchDecimal   MatchType = "decimal"
	MatchInclude   MatchType = "include"
	MatchEachLike  MatchType = "each_like"
	MatchOptional  MatchType = "optional"
	MatchNullOr    MatchType = "null_or"
)

// MarshalJSON implements custom JSON marshaling for Payload.
func (p Payload) MarshalJSON() ([]byte, error) {
	if p.HTTP != nil {
		return json.Marshal(map[string]any{"http": p.HTTP})
	}
	if p.GRPC != nil {
		return json.Marshal(map[string]any{"grpc": p.GRPC})
	}
	if p.Async != nil {
		return json.Marshal(map[string]any{"async": p.Async})
	}
	return []byte("{}"), nil
}

// UnmarshalJSON implements custom JSON unmarshaling for Payload.
func (p *Payload) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if httpData, ok := raw["http"]; ok {
		var http HTTPPayload
		if err := json.Unmarshal(httpData, &http); err != nil {
			return err
		}
		p.HTTP = &http
	}

	if grpcData, ok := raw["grpc"]; ok {
		var grpc GRPCPayload
		if err := json.Unmarshal(grpcData, &grpc); err != nil {
			return err
		}
		p.GRPC = &grpc
	}

	if asyncData, ok := raw["async"]; ok {
		var async AsyncPayload
		if err := json.Unmarshal(asyncData, &async); err != nil {
			return err
		}
		p.Async = &async
	}

	return nil
}
