package grpc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestNewMockServer(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "test-1",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "test.Service",
					Method:     "Test",
					StreamType: contract.StreamTypeUnary,
				},
			},
		},
	}

	server := NewMockServer(interactions)
	if server == nil {
		t.Fatal("NewMockServer returned nil")
	}
}

func TestMockServer_StartStop(t *testing.T) {
	server := NewMockServer(nil)

	if err := server.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	addr := server.Address()
	if addr == "" {
		t.Error("Expected non-empty address")
	}

	if err := server.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestMockServer_HandleUnaryCall(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID:          "get-user",
			Description: "get user by id",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "users.UserService",
					Method:     "GetUser",
					StreamType: contract.StreamTypeUnary,
					Request: contract.GRPCMessage{
						Message: map[string]any{"id": "123"},
					},
					Response: contract.GRPCResponse{
						Status: "OK",
						Message: map[string]any{
							"id":   "123",
							"name": "John",
						},
					},
				},
			},
		},
	}

	server := NewMockServer(interactions)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		if err := server.Stop(); err != nil {
			t.Logf("Warning: failed to stop server: %v", err)
		}
	}()

	t.Run("successful call", func(t *testing.T) {
		request, err := json.Marshal(map[string]any{"id": "123"})
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		response, metadata, status, err := server.HandleUnaryCall(
			context.Background(),
			"users.UserService",
			"GetUser",
			nil,
			request,
		)

		if err != nil {
			t.Fatalf("HandleUnaryCall failed: %v", err)
		}

		if status != "OK" {
			t.Errorf("Status = %s, want OK", status)
		}

		var resp map[string]any
		if err := json.Unmarshal(response, &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if resp["id"] != "123" {
			t.Errorf("Response id = %v, want 123", resp["id"])
		}

		_ = metadata // metadata may be nil if not set
	})

	t.Run("no matching interaction", func(t *testing.T) {
		request, err := json.Marshal(map[string]any{})
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		_, _, _, err = server.HandleUnaryCall(
			context.Background(),
			"unknown.Service",
			"Unknown",
			nil,
			request,
		)

		if err == nil {
			t.Error("Expected error for unknown service/method")
		}
	})
}

func TestMockServer_HandleServerStreamCall(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "stream-events",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "events.EventService",
					Method:     "StreamEvents",
					StreamType: contract.StreamTypeServerStream,
					Request: contract.GRPCMessage{
						Message: map[string]any{"user_id": "123"},
					},
					Response: contract.GRPCResponse{
						Status:  "OK",
						Message: map[string]any{"event": "data"},
					},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	t.Run("successful stream", func(t *testing.T) {
		request, err := json.Marshal(map[string]any{"user_id": "123"})
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		handler, err := server.HandleServerStreamCall(
			context.Background(),
			"events.EventService",
			"StreamEvents",
			nil,
			request,
		)

		if err != nil {
			t.Fatalf("HandleServerStreamCall failed: %v", err)
		}

		// Get first response
		resp, ok := handler.Next()
		if !ok {
			t.Error("Expected at least one response")
		}
		if resp == nil {
			t.Error("Response should not be nil")
		}

		// No more responses
		_, ok = handler.Next()
		if ok {
			t.Error("Expected no more responses")
		}
	})

	t.Run("no matching interaction", func(t *testing.T) {
		request, err := json.Marshal(map[string]any{})
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		_, err = server.HandleServerStreamCall(
			context.Background(),
			"unknown.Service",
			"Unknown",
			nil,
			request,
		)

		if err == nil {
			t.Error("Expected error for unknown service/method")
		}
	})
}

func TestMockServer_HandleClientStreamCall(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "upload-data",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "data.DataService",
					Method:     "Upload",
					StreamType: contract.StreamTypeClientStream,
					Request: contract.GRPCMessage{
						Message: map[string]any{"chunk": "data"},
					},
					Response: contract.GRPCResponse{
						Status:  "OK",
						Message: map[string]any{"total": 100},
					},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	t.Run("successful stream", func(t *testing.T) {
		handler, err := server.HandleClientStreamCall(
			context.Background(),
			"data.DataService",
			"Upload",
			nil,
		)

		if err != nil {
			t.Fatalf("HandleClientStreamCall failed: %v", err)
		}

		// Send messages
		msg1, err := json.Marshal(map[string]any{"chunk": "data1"})
		if err != nil {
			t.Fatalf("Failed to marshal msg1: %v", err)
		}
		msg2, err := json.Marshal(map[string]any{"chunk": "data2"})
		if err != nil {
			t.Fatalf("Failed to marshal msg2: %v", err)
		}
		handler.Send(msg1, nil)
		handler.Send(msg2, nil)

		// Close and receive
		response, _, status, err := handler.CloseAndRecv()
		if err != nil {
			t.Fatalf("CloseAndRecv failed: %v", err)
		}

		if status != "OK" {
			t.Errorf("Status = %s, want OK", status)
		}

		if response == nil {
			t.Error("Response should not be nil")
		}
	})
}

func TestMockServer_HandleBidiStreamCall(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "chat",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "chat.ChatService",
					Method:     "Chat",
					StreamType: contract.StreamTypeBidirectional,
					Sequence: []contract.StreamMessage{
						{
							Direction: contract.StreamDirectionClient,
							Message:   map[string]any{"text": "hello"},
						},
						{
							Direction: contract.StreamDirectionServer,
							Message:   map[string]any{"text": "hi"},
						},
					},
					Response: contract.GRPCResponse{Status: "OK"},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	t.Run("successful bidirectional stream", func(t *testing.T) {
		handler, err := server.HandleBidiStreamCall(
			context.Background(),
			"chat.ChatService",
			"Chat",
			nil,
		)

		if err != nil {
			t.Fatalf("HandleBidiStreamCall failed: %v", err)
		}

		// Send client message
		msg, err := json.Marshal(map[string]any{"text": "hello"})
		if err != nil {
			t.Fatalf("Failed to marshal msg: %v", err)
		}
		handler.Send(msg, nil)

		// Receive server message
		resp, ok := handler.Recv()
		if !ok {
			t.Error("Expected server response")
		}
		if resp == nil {
			t.Error("Response should not be nil")
		}

		// Close
		if err := handler.Close(); err != nil {
			t.Logf("Close returned: %v", err)
		}
	})
}

func TestMockServer_ReceivedCalls(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "test",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "test.Service",
					Method:     "Test",
					StreamType: contract.StreamTypeUnary,
					Request: contract.GRPCMessage{
						Message: map[string]any{},
					},
					Response: contract.GRPCResponse{
						Status:  "OK",
						Message: map[string]any{},
					},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	// Make a call
	request, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	if _, _, _, err := server.HandleUnaryCall(context.Background(), "test.Service", "Test", nil, request); err != nil {
		t.Fatalf("HandleUnaryCall failed: %v", err)
	}

	calls := server.ReceivedCalls()
	if len(calls) != 1 {
		t.Errorf("ReceivedCalls count = %d, want 1", len(calls))
	}

	if calls[0].Service != "test.Service" {
		t.Errorf("Call service = %s, want test.Service", calls[0].Service)
	}
}

func TestMockServer_Verify(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID:          "test-1",
			Description: "first interaction",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "test.Service",
					Method:     "Test1",
					StreamType: contract.StreamTypeUnary,
					Request:    contract.GRPCMessage{Message: map[string]any{}},
					Response:   contract.GRPCResponse{Status: "OK", Message: map[string]any{}},
				},
			},
		},
		{
			ID:          "test-2",
			Description: "second interaction",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "test.Service",
					Method:     "Test2",
					StreamType: contract.StreamTypeUnary,
					Request:    contract.GRPCMessage{Message: map[string]any{}},
					Response:   contract.GRPCResponse{Status: "OK", Message: map[string]any{}},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	t.Run("unmatched interactions", func(t *testing.T) {
		result := server.Verify()
		if result.Success {
			t.Error("Expected failure when interactions not matched")
		}
	})

	t.Run("all interactions matched", func(t *testing.T) {
		server.Reset()

		// Match both interactions
		request, err := json.Marshal(map[string]any{})
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}
		if _, _, _, err := server.HandleUnaryCall(context.Background(), "test.Service", "Test1", nil, request); err != nil {
			t.Fatalf("HandleUnaryCall Test1 failed: %v", err)
		}
		if _, _, _, err := server.HandleUnaryCall(context.Background(), "test.Service", "Test2", nil, request); err != nil {
			t.Fatalf("HandleUnaryCall Test2 failed: %v", err)
		}

		result := server.Verify()
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})
}

func TestMockServer_Reset(t *testing.T) {
	interactions := []contract.Interaction{
		{
			ID: "test",
			Payload: contract.Payload{
				GRPC: &contract.GRPCPayload{
					Service:    "test.Service",
					Method:     "Test",
					StreamType: contract.StreamTypeUnary,
					Request:    contract.GRPCMessage{Message: map[string]any{}},
					Response:   contract.GRPCResponse{Status: "OK", Message: map[string]any{}},
				},
			},
		},
	}

	server := NewMockServer(interactions)

	// Make a call
	request, err := json.Marshal(map[string]any{})
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}
	if _, _, _, err := server.HandleUnaryCall(context.Background(), "test.Service", "Test", nil, request); err != nil {
		t.Fatalf("HandleUnaryCall failed: %v", err)
	}

	// Reset
	server.Reset()

	calls := server.ReceivedCalls()
	if len(calls) != 0 {
		t.Errorf("After Reset, ReceivedCalls count = %d, want 0", len(calls))
	}
}
