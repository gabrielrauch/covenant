package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// MockServer is a gRPC mock server that responds based on contract interactions.
type MockServer struct {
	interactions []contract.Interaction
	listener     net.Listener
	validator    *Validator

	mu               sync.RWMutex
	receivedRequests []RecordedCall
	matchedCalls     map[string]int
}

// RecordedCall represents a captured gRPC call.
type RecordedCall struct {
	Service   string
	Method    string
	Metadata  map[string]string
	Request   []byte
	Responses [][]byte
	Matched   bool
	Error     string
}

// NewMockServer creates a new gRPC mock server.
func NewMockServer(interactions []contract.Interaction) *MockServer {
	return &MockServer{
		interactions: interactions,
		validator:    NewValidator(),
		matchedCalls: make(map[string]int),
	}
}

// Start starts the mock server on an ephemeral port.
func (s *MockServer) Start() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Note: In a full implementation, we would start a real gRPC server here
	// using google.golang.org/grpc. For now, we provide the mock infrastructure.
	// The actual gRPC server implementation would require proto descriptors.

	return nil
}

// Stop stops the mock server.
func (s *MockServer) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Address returns the address of the mock server.
func (s *MockServer) Address() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// HandleUnaryCall handles a unary gRPC call (for testing without a real gRPC server).
func (s *MockServer) HandleUnaryCall(ctx context.Context, service, method string, metadata map[string]string, request []byte) (response []byte, respMetadata map[string]string, status string, err error) {
	recorded := RecordedCall{
		Service:  service,
		Method:   method,
		Metadata: metadata,
		Request:  request,
	}

	// Find matching interaction
	interaction, err := s.findMatchingInteraction(service, method, contract.StreamTypeUnary)
	if err != nil {
		recorded.Error = err.Error()
		s.recordCall(&recorded)
		return nil, nil, "", err
	}

	// Validate request
	result := s.validator.ValidateRequest(interaction, request, metadata)
	if !result.Success {
		recorded.Error = fmt.Sprintf("request validation failed: %v", formatErrors(result.Errors))
		s.recordCall(&recorded)
		return nil, nil, "", fmt.Errorf("%s", recorded.Error)
	}

	recorded.Matched = true
	s.recordCall(&recorded)
	s.incrementMatchCount(interaction.ID)

	// Build response
	response, respMetadata, status = s.buildResponse(interaction)
	return response, respMetadata, status, nil
}

// ServerStreamHandler handles server-streaming responses.
type ServerStreamHandler struct {
	interaction *contract.Interaction
	mock        *MockServer
	index       int
}

// HandleServerStreamCall initiates a server-streaming call.
func (s *MockServer) HandleServerStreamCall(ctx context.Context, service, method string, metadata map[string]string, request []byte) (*ServerStreamHandler, error) {
	interaction, err := s.findMatchingInteraction(service, method, contract.StreamTypeServerStream)
	if err != nil {
		return nil, err
	}

	// Validate request
	result := s.validator.ValidateRequest(interaction, request, metadata)
	if !result.Success {
		return nil, fmt.Errorf("request validation failed: %v", formatErrors(result.Errors))
	}

	s.incrementMatchCount(interaction.ID)

	return &ServerStreamHandler{
		interaction: interaction,
		mock:        s,
		index:       0,
	}, nil
}

// Next returns the next response in the stream, or nil if done.
func (h *ServerStreamHandler) Next() ([]byte, bool) {
	// For server streaming, we'd typically have multiple response messages
	// defined in the contract. For simplicity, return the single response.
	if h.index > 0 {
		return nil, false
	}

	h.index++
	response, _, _ := h.mock.buildResponse(h.interaction)
	return response, true
}

// ClientStreamHandler handles client-streaming requests.
type ClientStreamHandler struct {
	interaction *contract.Interaction
	mock        *MockServer
	validator   *StreamValidator
	messages    [][]byte
}

// HandleClientStreamCall initiates a client-streaming call.
func (s *MockServer) HandleClientStreamCall(ctx context.Context, service, method string, metadata map[string]string) (*ClientStreamHandler, error) {
	interaction, err := s.findMatchingInteraction(service, method, contract.StreamTypeClientStream)
	if err != nil {
		return nil, err
	}

	return &ClientStreamHandler{
		interaction: interaction,
		mock:        s,
		validator:   NewStreamValidator(interaction),
	}, nil
}

// Send sends a client message to the stream.
func (h *ClientStreamHandler) Send(message []byte, metadata map[string]string) {
	h.messages = append(h.messages, message)
	h.validator.ValidateClientMessage(message, metadata)
}

// CloseAndRecv closes the client stream and receives the response.
func (h *ClientStreamHandler) CloseAndRecv() (response []byte, metadata map[string]string, status string, err error) {
	result := h.validator.Result()
	if !result.Success {
		return nil, nil, "", fmt.Errorf("stream validation failed: %v", formatErrors(result.Errors))
	}

	h.mock.incrementMatchCount(h.interaction.ID)
	response, respMetadata, status := h.mock.buildResponse(h.interaction)
	return response, respMetadata, status, nil
}

// BidiStreamHandler handles bidirectional streaming.
type BidiStreamHandler struct {
	interaction *contract.Interaction
	mock        *MockServer
	validator   *StreamValidator
	seqIndex    int
}

// HandleBidiStreamCall initiates a bidirectional streaming call.
func (s *MockServer) HandleBidiStreamCall(ctx context.Context, service, method string, metadata map[string]string) (*BidiStreamHandler, error) {
	interaction, err := s.findMatchingInteraction(service, method, contract.StreamTypeBidirectional)
	if err != nil {
		return nil, err
	}

	return &BidiStreamHandler{
		interaction: interaction,
		mock:        s,
		validator:   NewStreamValidator(interaction),
	}, nil
}

// Send sends a client message.
func (h *BidiStreamHandler) Send(message []byte, metadata map[string]string) {
	h.validator.ValidateClientMessage(message, metadata)
}

// Recv receives a server message based on the sequence.
func (h *BidiStreamHandler) Recv() ([]byte, bool) {
	if h.interaction.Payload.GRPC == nil {
		return nil, false
	}

	seq := h.interaction.Payload.GRPC.Sequence
	if h.seqIndex >= len(seq) {
		return nil, false
	}

	// Find next server message in sequence
	for h.seqIndex < len(seq) {
		msg := seq[h.seqIndex]
		h.seqIndex++
		if msg.Direction == contract.StreamDirectionServer {
			response, err := json.Marshal(msg.Message)
			if err != nil {
				return nil, false
			}
			// Notify validator that server message was sent (keeps indices in sync)
			h.validator.ValidateServerMessage(response, nil)
			return response, true
		}
	}

	return nil, false
}

// Close closes the stream and returns validation result.
func (h *BidiStreamHandler) Close() error {
	result := h.validator.Result()
	if !result.Success {
		return fmt.Errorf("stream validation failed: %v", formatErrors(result.Errors))
	}
	h.mock.incrementMatchCount(h.interaction.ID)
	return nil
}

// findMatchingInteraction finds an interaction matching the service/method.
func (s *MockServer) findMatchingInteraction(service, method string, streamType contract.StreamType) (*contract.Interaction, error) {
	for i := range s.interactions {
		interaction := &s.interactions[i]
		if interaction.Payload.GRPC == nil {
			continue
		}

		grpc := interaction.Payload.GRPC
		if grpc.Service == service && grpc.Method == method && grpc.StreamType == streamType {
			return interaction, nil
		}
	}

	return nil, fmt.Errorf("no interaction matches %s/%s (%s)", service, method, streamType)
}

// buildResponse builds the response from the interaction.
func (s *MockServer) buildResponse(interaction *contract.Interaction) (response []byte, metadata map[string]string, status string) {
	if interaction.Payload.GRPC == nil {
		return nil, nil, ""
	}

	resp := interaction.Payload.GRPC.Response

	// Serialize response message
	var responseBytes []byte
	if resp.Message != nil {
		var err error
		responseBytes, err = json.Marshal(resp.Message)
		if err != nil {
			return nil, nil, ""
		}
	}

	return responseBytes, resp.Metadata, resp.Status
}

// recordCall records a call for later analysis.
func (s *MockServer) recordCall(call *RecordedCall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedRequests = append(s.receivedRequests, *call)
}

// incrementMatchCount increments the match counter.
func (s *MockServer) incrementMatchCount(interactionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchedCalls[interactionID]++
}

// ReceivedCalls returns all recorded calls.
func (s *MockServer) ReceivedCalls() []RecordedCall {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]RecordedCall, len(s.receivedRequests))
	copy(result, s.receivedRequests)
	return result
}

// Verify checks that all interactions were matched.
func (s *MockServer) Verify() validator.ValidationResult {
	result := validator.NewSuccessResult()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, interaction := range s.interactions {
		if interaction.Payload.GRPC == nil {
			continue
		}

		count := s.matchedCalls[interaction.ID]
		if count == 0 {
			result.AddError(&validator.ValidationError{
				Path:    interaction.ID,
				Message: fmt.Sprintf("interaction %q was never matched", interaction.Description),
			})
		}
	}

	return result
}

// Reset clears all recorded state.
func (s *MockServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedRequests = nil
	s.matchedCalls = make(map[string]int)
}

// formatErrors formats validation errors.
func formatErrors(errors []validator.ValidationError) string {
	if len(errors) == 0 {
		return ""
	}
	msg := errors[0].Message
	if len(errors) > 1 {
		msg += fmt.Sprintf(" (and %d more)", len(errors)-1)
	}
	return msg
}
