// Package testutil provides test utilities and fixtures for the covenant framework.
package testutil

import (
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// NewTestContract creates a basic test contract with sensible defaults.
func NewTestContract(consumer, provider string) *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:       "test-contract-id",
			Version:  "1.0.0",
			Consumer: contract.Participant{Name: consumer, Version: "1.0.0"},
			Provider: contract.Participant{Name: provider, Version: "1.0.0"},
			Status:   contract.StatusDraft,
			Tags:     []string{"test"},
			CreatedAt: time.Now(),
		},
		Interactions: []contract.Interaction{},
	}
}

// NewHTTPInteraction creates a test HTTP interaction.
func NewHTTPInteraction(description, method, path string, status int) contract.Interaction {
	return contract.Interaction{
		ID:          "test-interaction-id",
		Description: description,
		Protocol:    contract.ProtocolHTTP,
		Payload: contract.Payload{
			HTTP: &contract.HTTPPayload{
				Request: contract.HTTPRequest{
					Method: method,
					Path:   path,
				},
				Response: contract.HTTPResponse{
					Status: status,
				},
			},
		},
	}
}

// NewHTTPInteractionWithBody creates a test HTTP interaction with request/response bodies.
func NewHTTPInteractionWithBody(description, method, path string, status int, reqBody, respBody any) contract.Interaction {
	return contract.Interaction{
		ID:          "test-interaction-id",
		Description: description,
		Protocol:    contract.ProtocolHTTP,
		Payload: contract.Payload{
			HTTP: &contract.HTTPPayload{
				Request: contract.HTTPRequest{
					Method:  method,
					Path:    path,
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    reqBody,
				},
				Response: contract.HTTPResponse{
					Status:  status,
					Headers: map[string]string{"Content-Type": "application/json"},
					Body:    respBody,
				},
			},
		},
	}
}

// NewGRPCInteraction creates a test gRPC interaction.
func NewGRPCInteraction(description, service, method string) contract.Interaction {
	return contract.Interaction{
		ID:          "test-grpc-interaction-id",
		Description: description,
		Protocol:    contract.ProtocolGRPC,
		Payload: contract.Payload{
			GRPC: &contract.GRPCPayload{
				Service:    service,
				Method:     method,
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
	}
}

// NewAsyncInteraction creates a test async messaging interaction.
func NewAsyncInteraction(description, destination string, direction contract.AsyncDirection) contract.Interaction {
	return contract.Interaction{
		ID:          "test-async-interaction-id",
		Description: description,
		Protocol:    contract.ProtocolAsync,
		Payload: contract.Payload{
			Async: &contract.AsyncPayload{
				Destination: destination,
				Direction:   direction,
				Message: contract.AsyncMessage{
					Payload: map[string]any{},
				},
			},
		},
	}
}

// WithProviderState adds a provider state to an interaction.
func WithProviderState(interaction contract.Interaction, name string, params map[string]any) contract.Interaction {
	interaction.ProviderStates = append(interaction.ProviderStates, contract.ProviderState{
		Name:   name,
		Params: params,
	})
	return interaction
}

// WithMatchingRules adds matching rules to an interaction.
func WithMatchingRules(interaction contract.Interaction, rules contract.MatchingRules) contract.Interaction {
	interaction.MatchingRules = rules
	return interaction
}

// SampleOrderResponse returns a sample order response body for testing.
func SampleOrderResponse() map[string]any {
	return map[string]any{
		"id":     "order-123",
		"status": "pending",
		"items": []map[string]any{
			{"sku": "ITEM-001", "quantity": 2, "price": 29.99},
		},
		"total": 59.98,
	}
}

// SampleUserResponse returns a sample user response body for testing.
func SampleUserResponse() map[string]any {
	return map[string]any{
		"id":    "user-456",
		"name":  "Test User",
		"email": "test@example.com",
	}
}
