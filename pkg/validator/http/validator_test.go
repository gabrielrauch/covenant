package httpvalidator_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

func TestNewValidator(t *testing.T) {
	v := httpvalidator.NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
	// Validator is properly initialized if NewValidator returns non-nil
}

func TestValidator_WithClient(t *testing.T) {
	v := httpvalidator.NewValidator()
	customClient := &http.Client{}
	result := v.WithClient(customClient)
	if result != v {
		t.Error("WithClient should return the validator for chaining")
	}
	// Custom client is used internally; verified through behavior tests
}

func TestValidator_Validate(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		actual      validator.ActualData
		wantSuccess bool
		wantErrors  int
	}{
		{
			name: "no HTTP payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			actual:      validator.ActualData{},
			wantSuccess: false,
			wantErrors:  1,
		},
		{
			name: "status code match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: 200,
			},
			wantSuccess: true,
			wantErrors:  0,
		},
		{
			name: "status code mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: 404,
			},
			wantSuccess: false,
			wantErrors:  1,
		},
		{
			name: "header validation",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
							Headers: map[string]string{
								"Content-Type": "application/json",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Status: 200,
				Metadata: map[string]string{
					"content-type": "application/json",
				},
			},
			wantSuccess: true,
			wantErrors:  0,
		},
		{
			name: "body validation success",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
							Body: map[string]any{
								"id":   "123",
								"name": "test",
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Status:   200,
				Response: []byte(`{"id": "123", "name": "test"}`),
			},
			wantSuccess: true,
			wantErrors:  0,
		},
		{
			name: "body validation with type mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
							Body: map[string]any{
								"count": 10,
							},
						},
					},
				},
			},
			actual: validator.ActualData{
				Status:   200,
				Response: []byte(`{"count": "not a number"}`),
			},
			wantSuccess: false,
			wantErrors:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()
			result := v.Validate(context.Background(), &tt.interaction, tt.actual)

			if result.Success != tt.wantSuccess {
				t.Errorf("Validate() success = %v, want %v", result.Success, tt.wantSuccess)
			}

			if len(result.Errors) != tt.wantErrors {
				t.Errorf("Validate() errors = %d, want %d: %v", len(result.Errors), tt.wantErrors, result.Errors)
			}
		})
	}
}

func TestValidator_ValidateRequest(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		request     *http.Request
		wantSuccess bool
	}{
		{
			name: "no HTTP payload",
			interaction: contract.Interaction{
				Payload: contract.Payload{},
			},
			request:     httptest.NewRequest("GET", "/test", http.NoBody),
			wantSuccess: false,
		},
		{
			name: "method match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/test",
						},
					},
				},
			},
			request:     httptest.NewRequest("GET", "/test", http.NoBody),
			wantSuccess: true,
		},
		{
			name: "method mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "POST",
							Path:   "/test",
						},
					},
				},
			},
			request:     httptest.NewRequest("GET", "/test", http.NoBody),
			wantSuccess: false,
		},
		{
			name: "path mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/expected",
						},
					},
				},
			},
			request:     httptest.NewRequest("GET", "/actual", http.NoBody),
			wantSuccess: false,
		},
		{
			name: "query params match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/test",
							Query: map[string]string{
								"page": "1",
							},
						},
					},
				},
			},
			request:     httptest.NewRequest("GET", "/test?page=1", http.NoBody),
			wantSuccess: true,
		},
		{
			name: "query params mismatch",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/test",
							Query: map[string]string{
								"page": "1",
							},
						},
					},
				},
			},
			request:     httptest.NewRequest("GET", "/test?page=2", http.NoBody),
			wantSuccess: false,
		},
		{
			name: "headers match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/test",
							Headers: map[string]string{
								"Authorization": "Bearer token",
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest("GET", "/test", http.NoBody)
				req.Header.Set("Authorization", "Bearer token")
				return req
			}(),
			wantSuccess: true,
		},
		{
			name: "body match",
			interaction: contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "POST",
							Path:   "/test",
							Body: map[string]any{
								"name": "test",
							},
						},
					},
				},
			},
			request:     httptest.NewRequest("POST", "/test", strings.NewReader(`{"name": "test"}`)),
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()
			result := v.ValidateRequest(&tt.interaction, tt.request)

			if result.Success != tt.wantSuccess {
				t.Errorf("ValidateRequest() success = %v, want %v, errors: %v",
					result.Success, tt.wantSuccess, result.Errors)
			}
		})
	}
}

func TestValidator_ExecuteInteraction(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/123" && r.Method == "GET" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte(`{"id": "123", "name": "John"}`)); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	v := httpvalidator.NewValidator()

	t.Run("successful request", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				HTTP: &contract.HTTPPayload{
					Request: contract.HTTPRequest{
						Method: "GET",
						Path:   "/users/123",
					},
				},
			},
		}

		resp, err := v.ExecuteInteraction(context.Background(), server.URL, &interaction)
		if err != nil {
			t.Fatalf("ExecuteInteraction failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	t.Run("no HTTP payload", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{},
		}

		resp, err := v.ExecuteInteraction(context.Background(), server.URL, &interaction)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			t.Error("Expected error for missing HTTP payload")
		}
	})

	t.Run("with query params", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				HTTP: &contract.HTTPPayload{
					Request: contract.HTTPRequest{
						Method: "GET",
						Path:   "/search",
						Query: map[string]string{
							"q": "test",
						},
					},
				},
			},
		}

		resp, err := v.ExecuteInteraction(context.Background(), server.URL, &interaction)
		if err != nil {
			t.Fatalf("ExecuteInteraction failed: %v", err)
		}
		defer resp.Body.Close()
	})

	t.Run("with body", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				HTTP: &contract.HTTPPayload{
					Request: contract.HTTPRequest{
						Method: "POST",
						Path:   "/users",
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: map[string]any{
							"name": "New User",
						},
					},
				},
			},
		}

		resp, err := v.ExecuteInteraction(context.Background(), server.URL, &interaction)
		if err != nil {
			t.Fatalf("ExecuteInteraction failed: %v", err)
		}
		defer resp.Body.Close()
	})
}

func TestValidator_ValidateResponse(t *testing.T) {
	v := httpvalidator.NewValidator()

	t.Run("successful validation", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				HTTP: &contract.HTTPPayload{
					Response: contract.HTTPResponse{
						Status: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: map[string]any{
							"id": "123",
						},
					},
				},
			},
		}

		resp := &http.Response{
			StatusCode: 200,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewReader([]byte(`{"id": "123"}`))),
		}

		result := v.ValidateResponse(&interaction, resp, nil)
		if !result.Success {
			t.Errorf("Expected success, got errors: %v", result.Errors)
		}
	})

	t.Run("no HTTP payload", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{},
		}

		resp := &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
		}

		result := v.ValidateResponse(&interaction, resp, nil)
		if result.Success {
			t.Error("Expected failure for missing HTTP payload")
		}
	})

	t.Run("with matching rules", func(t *testing.T) {
		interaction := contract.Interaction{
			Payload: contract.Payload{
				HTTP: &contract.HTTPPayload{
					Response: contract.HTTPResponse{
						Status: 200,
						Body: map[string]any{
							"id": "example-id",
						},
					},
				},
			},
			MatchingRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
		}

		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewReader([]byte(`{"id": "different-id"}`))),
		}

		result := v.ValidateResponse(&interaction, resp, nil)
		if !result.Success {
			t.Errorf("Expected success with type matcher, got errors: %v", result.Errors)
		}
	})
}

// Note: validateBody is a private method and is tested indirectly through
// the public Validate, ValidateRequest, and ValidateResponse methods above.
