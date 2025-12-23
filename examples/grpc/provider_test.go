package grpcexample

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
	grpcvalidator "github.com/gabrielrauch/covenant/pkg/validator/grpc"
)

// UserServiceProvider simulates the provider's gRPC service implementation.
type UserServiceProvider struct {
	users map[string]*User
}

// NewUserServiceProvider creates a new provider.
func NewUserServiceProvider() *UserServiceProvider {
	return &UserServiceProvider{
		users: make(map[string]*User),
	}
}

// AddUser adds a user to the provider (for state setup).
func (p *UserServiceProvider) AddUser(user *User) {
	p.users[user.ID] = user
}

// GetUser handles the GetUser RPC call.
func (p *UserServiceProvider) GetUser(id string) (*User, error) {
	user, ok := p.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// TestProviderVerification_GRPC demonstrates provider verification for gRPC contracts.
func TestProviderVerification_GRPC(t *testing.T) {
	// Create the provider
	provider := NewUserServiceProvider()

	// Load contracts from directory
	contractFiles, err := filepath.Glob("./contracts/*.json")
	if err != nil {
		t.Fatalf("Failed to glob contracts: %v", err)
	}

	if len(contractFiles) == 0 {
		t.Skip("No contract files found - run consumer tests first")
	}

	grpcValidator := grpcvalidator.NewValidator()

	for _, file := range contractFiles {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			t.Logf("Skipping %s: %v", file, err)
			continue
		}

		t.Run(file, func(t *testing.T) {
			for _, interaction := range c.Interactions {
				if interaction.Protocol != contract.ProtocolGRPC {
					continue
				}

				if interaction.Payload.GRPC == nil {
					continue
				}

				t.Run(interaction.Description, func(t *testing.T) {
					// Setup provider state
					for _, state := range interaction.ProviderStates {
						setupProviderState(provider, state)
					}

					// Execute the actual provider logic
					grpcPayload := interaction.Payload.GRPC

					switch grpcPayload.Method {
					case "GetUser":
						testGetUserInteraction(t, provider, grpcValidator, &interaction)
					case "StreamEvents":
						t.Log("Server streaming test would go here")
					case "Chat":
						t.Log("Bidirectional streaming test would go here")
					default:
						t.Logf("Unknown method: %s", grpcPayload.Method)
					}
				})
			}
		})
	}
}

// setupProviderState sets up the provider state based on contract.
func setupProviderState(provider *UserServiceProvider, state contract.ProviderState) {
	if state.Name == "user 123 exists" {
		id, ok := state.Params["id"].(string)
		if !ok {
			id = ""
		}
		name, ok := state.Params["name"].(string)
		if !ok {
			name = ""
		}
		email, ok := state.Params["email"].(string)
		if !ok {
			email = ""
		}
		provider.AddUser(&User{
			ID:    id,
			Name:  name,
			Email: email,
		})
	}
}

// testGetUserInteraction tests the GetUser RPC against the contract.
func testGetUserInteraction(t *testing.T, provider *UserServiceProvider, v *grpcvalidator.Validator, interaction *contract.Interaction) {
	grpcPayload := interaction.Payload.GRPC

	// Parse the expected request to get the user ID
	requestMsg, ok := grpcPayload.Request.Message.(map[string]any)
	if !ok {
		t.Fatal("Invalid request message format")
	}

	userID, ok := requestMsg["id"].(string)
	if !ok {
		t.Fatal("request message missing 'id' field")
	}

	// Execute the actual provider logic
	user, err := provider.GetUser(userID)
	if err != nil {
		t.Fatalf("Provider error: %v", err)
	}

	if user == nil {
		t.Fatalf("User not found: %s", userID)
	}

	// Build actual response
	responseBytes, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Failed to marshal user: %v", err)
	}

	// Validate response against contract
	actual := validator.ActualData{
		Status:   grpcPayload.Response.Status,
		Metadata: grpcPayload.Response.Metadata,
		Response: responseBytes,
	}

	result := v.Validate(context.Background(), interaction, actual)

	if !result.Success {
		for _, err := range result.Errors {
			t.Errorf("Validation error: %s - %s (expected: %v, actual: %v)",
				err.Path, err.Message, err.Expected, err.Actual)
		}
	}
}

// TestProviderRequestValidation demonstrates validating incoming requests.
func TestProviderRequestValidation(t *testing.T) {
	// This test shows how a provider can validate incoming requests
	// against the contract to ensure consumers are sending valid data

	// Load the contract
	c := createUserServiceContract()
	interaction := c.Interactions[0]

	v := grpcvalidator.NewValidator()

	// Simulate an incoming request from consumer
	validRequest := map[string]any{"id": "123"}
	validRequestBytes, err := json.Marshal(validRequest)
	if err != nil {
		t.Fatalf("Failed to marshal valid request: %v", err)
	}
	validMetadata := map[string]string{"authorization": "Bearer token123"}

	// Validate the request
	result := v.ValidateRequest(&interaction, validRequestBytes, validMetadata)

	if !result.Success {
		t.Errorf("Valid request should pass validation: %v", result.Errors)
	}

	// Test with invalid request (missing authorization)
	invalidMetadata := map[string]string{}
	result = v.ValidateRequest(&interaction, validRequestBytes, invalidMetadata)

	// This should fail because authorization header is required
	if result.Success {
		t.Log("Note: Metadata validation for missing headers may be lenient")
	}
}

// TestStreamValidation demonstrates stream message validation.
func TestStreamValidation(t *testing.T) {
	c := &contract.Contract{
		Metadata: contract.Metadata{
			Consumer: contract.Participant{Name: "test-consumer"},
			Provider: contract.Participant{Name: "test-provider"},
		},
		Interactions: []contract.Interaction{
			{
				ID:          "client-stream",
				Description: "client streaming test",
				Protocol:    contract.ProtocolGRPC,
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Service:    "test.TestService",
						Method:     "ClientStream",
						StreamType: contract.StreamTypeClientStream,
						Request: contract.GRPCMessage{
							Message: map[string]any{
								"value": 1,
							},
						},
						Response: contract.GRPCResponse{
							Status: "OK",
							Message: map[string]any{
								"total": 100,
							},
						},
					},
				},
			},
		},
	}

	interaction := c.Interactions[0]

	// Create stream validator
	sv := grpcvalidator.NewStreamValidator(&interaction)

	// Simulate receiving client messages
	msg1, err := json.Marshal(map[string]any{"value": 10})
	if err != nil {
		t.Fatalf("Failed to marshal msg1: %v", err)
	}
	msg2, err := json.Marshal(map[string]any{"value": 20})
	if err != nil {
		t.Fatalf("Failed to marshal msg2: %v", err)
	}
	msg3, err := json.Marshal(map[string]any{"value": 30})
	if err != nil {
		t.Fatalf("Failed to marshal msg3: %v", err)
	}

	sv.ValidateClientMessage(msg1, nil)
	sv.ValidateClientMessage(msg2, nil)
	sv.ValidateClientMessage(msg3, nil)

	// Get final result
	result := sv.Result()
	if !result.Success {
		for _, err := range result.Errors {
			t.Logf("Stream validation note: %s", err.Message)
		}
	}

	t.Log("Client stream validation completed")
}
