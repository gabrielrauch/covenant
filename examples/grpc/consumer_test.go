// Package grpcexample demonstrates gRPC consumer-driven contract testing with covenant.
package grpcexample

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gabrielrauch/covenant/pkg/contract"
	grpcvalidator "github.com/gabrielrauch/covenant/pkg/validator/grpc"
)

// User represents a user in the system.
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// createUserServiceContract creates a contract for the User service.
func createUserServiceContract() *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "user-dashboard"},
			Provider:  contract.Participant{Name: "user-service"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "get-user-by-id",
				Description: "get user by id",
				Protocol:    contract.ProtocolGRPC,
				ProviderStates: []contract.ProviderState{
					{
						Name: "user 123 exists",
						Params: map[string]any{
							"id":    "123",
							"name":  "John Doe",
							"email": "john@example.com",
						},
					},
				},
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Service:    "users.UserService",
						Method:     "GetUser",
						StreamType: contract.StreamTypeUnary,
						Request: contract.GRPCMessage{
							Metadata: map[string]string{
								"authorization": "Bearer token123",
							},
							Message: map[string]any{
								"id": "123",
							},
						},
						Response: contract.GRPCResponse{
							Status: "OK",
							Metadata: map[string]string{
								"x-request-id": "req-123",
							},
							Message: map[string]any{
								"id":    "123",
								"name":  "John Doe",
								"email": "john@example.com",
							},
						},
					},
				},
				MatchingRules: contract.MatchingRules{
					"$.response.metadata.x-request-id": {Match: contract.MatchTypeValue},
				},
			},
		},
	}
}

// TestGetUser_Consumer demonstrates a consumer test for gRPC.
func TestGetUser_Consumer(t *testing.T) {
	// Create the contract
	c := createUserServiceContract()

	// Get the interaction
	interaction := c.Interactions[0]

	// Create the mock server with our interaction
	mockServer := grpcvalidator.NewMockServer([]contract.Interaction{interaction})

	// Start the mock server
	if err := mockServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer func() {
		if err := mockServer.Stop(); err != nil {
			t.Errorf("Failed to stop mock server: %v", err)
		}
	}()

	t.Logf("Mock server running at: %s", mockServer.Address())

	// Simulate calling the gRPC service
	// In a real scenario, this would be your actual gRPC client code
	requestPayload, err := json.Marshal(map[string]any{"id": "123"})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	metadata := map[string]string{"authorization": "Bearer token123"}

	// Handle the unary call through the mock
	response, respMetadata, status, err := mockServer.HandleUnaryCall(
		context.Background(),
		"users.UserService",
		"GetUser",
		metadata,
		requestPayload,
	)

	if err != nil {
		t.Fatalf("Unary call failed: %v", err)
	}

	// Verify response
	if status != "OK" {
		t.Errorf("Expected status OK, got %s", status)
	}

	var user User
	if err := json.Unmarshal(response, &user); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if user.ID != "123" {
		t.Errorf("Expected user ID 123, got %s", user.ID)
	}
	if user.Name != "John Doe" {
		t.Errorf("Expected name John Doe, got %s", user.Name)
	}

	// Verify the metadata
	if respMetadata["x-request-id"] != "req-123" {
		t.Errorf("Expected x-request-id in response metadata")
	}

	// Verify all interactions were matched
	result := mockServer.Verify()
	if !result.Success {
		t.Errorf("Not all interactions were matched: %v", result.Errors)
	}

	// Save contract for provider verification
	if err := contract.SaveToFile(c, "./contracts/user-dashboard-user-service.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}

// createStreamingContract creates a contract for server-streaming RPC.
func createStreamingContract() *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "analytics-dashboard"},
			Provider:  contract.Participant{Name: "events-service"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "stream-events",
				Description: "stream events for user",
				Protocol:    contract.ProtocolGRPC,
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Service:    "events.EventService",
						Method:     "StreamEvents",
						StreamType: contract.StreamTypeServerStream,
						Request: contract.GRPCMessage{
							Message: map[string]any{
								"user_id": "user-456",
								"limit":   10,
							},
						},
						Response: contract.GRPCResponse{
							Status: "OK",
							Message: map[string]any{
								"event_id":   "evt-001",
								"type":       "page_view",
								"timestamp":  "2025-01-01T00:00:00Z",
								"properties": map[string]any{},
							},
						},
					},
				},
				MatchingRules: contract.MatchingRules{
					"$.response.message.event_id":  {Match: contract.MatchTypeValue},
					"$.response.message.timestamp": {Match: contract.MatchRegex, Pattern: `^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`},
				},
			},
		},
	}
}

// TestStreamEvents_Consumer demonstrates a consumer test for server streaming.
func TestStreamEvents_Consumer(t *testing.T) {
	c := createStreamingContract()
	interaction := c.Interactions[0]

	mockServer := grpcvalidator.NewMockServer([]contract.Interaction{interaction})
	if err := mockServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer func() {
		if err := mockServer.Stop(); err != nil {
			t.Errorf("Failed to stop mock server: %v", err)
		}
	}()

	requestPayload, err := json.Marshal(map[string]any{
		"user_id": "user-456",
		"limit":   10,
	})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	// Initiate server streaming call
	handler, err := mockServer.HandleServerStreamCall(
		context.Background(),
		"events.EventService",
		"StreamEvents",
		nil,
		requestPayload,
	)
	if err != nil {
		t.Fatalf("Server stream call failed: %v", err)
	}

	// Read streamed responses
	eventCount := 0
	for {
		response, hasMore := handler.Next()
		if !hasMore {
			break
		}

		var event map[string]any
		if err := json.Unmarshal(response, &event); err != nil {
			t.Fatalf("Failed to unmarshal event: %v", err)
		}

		eventCount++
		t.Logf("Received event: %v", event)
	}

	if eventCount == 0 {
		t.Error("Expected at least one event")
	}

	// Verify interactions
	result := mockServer.Verify()
	if !result.Success {
		t.Errorf("Not all interactions were matched: %v", result.Errors)
	}

	// Save contract
	if err := contract.SaveToFile(c, "./contracts/analytics-dashboard-events-service.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}

// TestBidirectionalStream_Consumer demonstrates bidirectional streaming.
func TestBidirectionalStream_Consumer(t *testing.T) {
	c := &contract.Contract{
		Metadata: contract.Metadata{
			ID:        uuid.New().String(),
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: "chat-frontend"},
			Provider:  contract.Participant{Name: "chat-service"},
			Status:    contract.StatusDraft,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "chat-stream",
				Description: "bidirectional chat stream",
				Protocol:    contract.ProtocolGRPC,
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Service:    "chat.ChatService",
						Method:     "Chat",
						StreamType: contract.StreamTypeBidirectional,
						Sequence: []contract.StreamMessage{
							{
								Direction: contract.StreamDirectionClient,
								Message:   map[string]any{"text": "Hello", "user": "alice"},
							},
							{
								Direction: contract.StreamDirectionServer,
								Message:   map[string]any{"text": "Hi Alice!", "user": "bot"},
							},
							{
								Direction: contract.StreamDirectionClient,
								Message:   map[string]any{"text": "How are you?", "user": "alice"},
							},
							{
								Direction: contract.StreamDirectionServer,
								Message:   map[string]any{"text": "I'm doing well!", "user": "bot"},
							},
						},
						Request:  contract.GRPCMessage{},
						Response: contract.GRPCResponse{Status: "OK"},
					},
				},
			},
		},
	}

	interaction := c.Interactions[0]
	mockServer := grpcvalidator.NewMockServer([]contract.Interaction{interaction})

	if err := mockServer.Start(); err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}
	defer func() {
		if err := mockServer.Stop(); err != nil {
			t.Errorf("Failed to stop mock server: %v", err)
		}
	}()

	// Initiate bidirectional stream
	handler, err := mockServer.HandleBidiStreamCall(
		context.Background(),
		"chat.ChatService",
		"Chat",
		nil,
	)
	if err != nil {
		t.Fatalf("Bidi stream call failed: %v", err)
	}

	// Send first client message
	msg1, err := json.Marshal(map[string]any{"text": "Hello", "user": "alice"})
	if err != nil {
		t.Fatalf("Failed to marshal msg1: %v", err)
	}
	handler.Send(msg1, nil)

	// Receive server response
	resp1, ok := handler.Recv()
	if !ok {
		t.Fatal("Expected server response")
	}
	t.Logf("Server response 1: %s", string(resp1))

	// Send second client message
	msg2, err := json.Marshal(map[string]any{"text": "How are you?", "user": "alice"})
	if err != nil {
		t.Fatalf("Failed to marshal msg2: %v", err)
	}
	handler.Send(msg2, nil)

	// Receive second server response
	resp2, ok := handler.Recv()
	if !ok {
		t.Fatal("Expected second server response")
	}
	t.Logf("Server response 2: %s", string(resp2))

	// Close stream
	if err := handler.Close(); err != nil {
		t.Errorf("Stream close failed: %v", err)
	}

	// Save contract
	if err := contract.SaveToFile(c, "./contracts/chat-frontend-chat-service.json"); err != nil {
		t.Fatalf("Failed to save contract: %v", err)
	}
}
