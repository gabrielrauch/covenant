package httpvalidator_test

import (
	"bytes"
	"context"
	"fmt"
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

// errReader is an io.Reader that always returns an error.
type errReader struct {
	err error
}

func (r *errReader) Read([]byte) (int, error) {
	return 0, r.err
}

// newInteractionWithHTTP is a helper that builds an Interaction with an HTTP payload.
func newInteractionWithHTTP(t *testing.T, req contract.HTTPRequest, resp contract.HTTPResponse, rules contract.MatchingRules) contract.Interaction {
	t.Helper()
	return contract.Interaction{
		Payload: contract.Payload{
			HTTP: &contract.HTTPPayload{
				Request:  req,
				Response: resp,
			},
		},
		MatchingRules: rules,
	}
}

// makeRequest is a helper that creates an *http.Request with optional headers and body.
func makeRequest(t *testing.T, method, target string, body io.Reader, headers map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestValidateRequest_MissingQueryParam(t *testing.T) {
	tests := []struct {
		name           string
		expectedQuery  map[string]string
		actualURL      string
		wantSuccess    bool
		wantErrCount   int
		wantErrPathSub string
	}{
		{
			name: "query param completely missing from request",
			expectedQuery: map[string]string{
				"page":  "1",
				"limit": "10",
			},
			actualURL:      "/test?page=1",
			wantSuccess:    false,
			wantErrCount:   1,
			wantErrPathSub: "$.request.query.limit",
		},
		{
			name: "all query params missing from request",
			expectedQuery: map[string]string{
				"page":  "1",
				"limit": "10",
			},
			actualURL:    "/test",
			wantSuccess:  false,
			wantErrCount: 2,
		},
		{
			name: "query param present but empty",
			expectedQuery: map[string]string{
				"filter": "active",
			},
			actualURL:      "/test?filter=",
			wantSuccess:    false,
			wantErrCount:   1,
			wantErrPathSub: "$.request.query.filter",
		},
		{
			name: "query param present but wrong value",
			expectedQuery: map[string]string{
				"sort": "asc",
			},
			actualURL:      "/test?sort=desc",
			wantSuccess:    false,
			wantErrCount:   1,
			wantErrPathSub: "$.request.query.sort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()
			interaction := newInteractionWithHTTP(t,
				contract.HTTPRequest{
					Method: "GET",
					Path:   "/test",
					Query:  tt.expectedQuery,
				},
				contract.HTTPResponse{},
				nil,
			)
			req := makeRequest(t, "GET", tt.actualURL, http.NoBody, nil)
			result := v.ValidateRequest(&interaction, req)

			if result.Success != tt.wantSuccess {
				t.Errorf("success = %v, want %v; errors: %v", result.Success, tt.wantSuccess, result.Errors)
			}
			if len(result.Errors) != tt.wantErrCount {
				t.Errorf("error count = %d, want %d; errors: %v", len(result.Errors), tt.wantErrCount, result.Errors)
			}
			if tt.wantErrPathSub != "" && len(result.Errors) > 0 {
				found := false
				for _, e := range result.Errors {
					if e.Path == tt.wantErrPathSub {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error with path %q, got errors: %v", tt.wantErrPathSub, result.Errors)
				}
			}
		})
	}
}

func TestValidateRequest_HeaderCaseMismatch(t *testing.T) {
	tests := []struct {
		name         string
		expectedHdrs map[string]string
		actualHdrs   map[string]string
		wantSuccess  bool
		wantErrCount int
	}{
		{
			name: "lowercase expected matches uppercase actual via normalization",
			expectedHdrs: map[string]string{
				"content-type": "application/json",
			},
			actualHdrs: map[string]string{
				"Content-Type": "application/json",
			},
			wantSuccess:  true,
			wantErrCount: 0,
		},
		{
			name: "mixed case expected matches actual via normalization",
			expectedHdrs: map[string]string{
				"X-Request-ID": "abc-123",
			},
			actualHdrs: map[string]string{
				"x-request-id": "abc-123",
			},
			wantSuccess:  true,
			wantErrCount: 0,
		},
		{
			name: "case insensitive match with value mismatch still fails",
			expectedHdrs: map[string]string{
				"Authorization": "Bearer token-a",
			},
			actualHdrs: map[string]string{
				"authorization": "Bearer token-b",
			},
			wantSuccess:  false,
			wantErrCount: 1,
		},
		{
			name: "header completely absent fails regardless of case",
			expectedHdrs: map[string]string{
				"X-Correlation-ID": "corr-1",
			},
			actualHdrs:   map[string]string{},
			wantSuccess:  false,
			wantErrCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()
			interaction := newInteractionWithHTTP(t,
				contract.HTTPRequest{
					Method:  "GET",
					Path:    "/test",
					Headers: tt.expectedHdrs,
				},
				contract.HTTPResponse{},
				nil,
			)
			req := makeRequest(t, "GET", "/test", http.NoBody, tt.actualHdrs)
			result := v.ValidateRequest(&interaction, req)

			if result.Success != tt.wantSuccess {
				t.Errorf("success = %v, want %v; errors: %v", result.Success, tt.wantSuccess, result.Errors)
			}
			if len(result.Errors) != tt.wantErrCount {
				t.Errorf("error count = %d, want %d; errors: %v", len(result.Errors), tt.wantErrCount, result.Errors)
			}
		})
	}
}

func TestValidateRequest_BodyReadError(t *testing.T) {
	v := httpvalidator.NewValidator()
	interaction := newInteractionWithHTTP(t,
		contract.HTTPRequest{
			Method: "POST",
			Path:   "/test",
			Body:   map[string]any{"name": "test"},
		},
		contract.HTTPResponse{},
		nil,
	)

	req := httptest.NewRequest("POST", "/test", &errReader{err: fmt.Errorf("disk I/O error")})

	result := v.ValidateRequest(&interaction, req)

	if result.Success {
		t.Fatal("expected failure when body read returns an error")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}

	foundBodyErr := false
	for _, e := range result.Errors {
		if e.Path == "$.request.body" && strings.Contains(e.Message, "failed to read request body") {
			foundBodyErr = true
			break
		}
	}
	if !foundBodyErr {
		t.Errorf("expected a body-read error at path $.request.body, got: %v", result.Errors)
	}
}

func TestValidateBody_InvalidJSON(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		wantMsgSub string
	}{
		{
			name:       "completely invalid JSON",
			body:       []byte(`not json at all`),
			wantMsgSub: "failed to parse response body as JSON",
		},
		{
			name:       "truncated JSON",
			body:       []byte(`{"id": "123`),
			wantMsgSub: "failed to parse response body as JSON",
		},
		{
			name:       "body with only whitespace",
			body:       []byte(`   `),
			wantMsgSub: "failed to parse response body as JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()
			interaction := newInteractionWithHTTP(t,
				contract.HTTPRequest{},
				contract.HTTPResponse{
					Status: 200,
					Body:   map[string]any{"id": "123"},
				},
				nil,
			)

			// Exercise validateBody indirectly through Validate
			result := v.Validate(context.Background(), &interaction, validator.ActualData{
				Status:   200,
				Response: tt.body,
			})

			if result.Success {
				t.Fatal("expected failure for invalid JSON body")
			}

			found := false
			for _, e := range result.Errors {
				if strings.Contains(e.Message, tt.wantMsgSub) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error message containing %q, got: %v", tt.wantMsgSub, result.Errors)
			}
		})
	}
}

func TestValidateBody_RuleCompilationError(t *testing.T) {
	v := httpvalidator.NewValidator()

	// Use an unknown match type that Compile will reject
	interaction := newInteractionWithHTTP(t,
		contract.HTTPRequest{},
		contract.HTTPResponse{
			Status: 200,
			Body:   map[string]any{"id": "123"},
		},
		contract.MatchingRules{
			"$.response.body.id": {Match: "unknown_matcher_type"},
		},
	)

	result := v.Validate(context.Background(), &interaction, validator.ActualData{
		Status:   200,
		Response: []byte(`{"id": "123"}`),
	})

	if result.Success {
		t.Fatal("expected failure when matching rule cannot be compiled")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "failed to compile matching rule") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about rule compilation failure, got: %v", result.Errors)
	}
}

func TestValidateResponse_ContractAndInteractionRuleMerging(t *testing.T) {
	tests := []struct {
		name             string
		interactionRules contract.MatchingRules
		contractRules    contract.MatchingRules
		responseBody     string
		wantSuccess      bool
		wantErrCount     int
		description      string
	}{
		{
			name:             "contract-level rules only",
			interactionRules: nil,
			contractRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			responseBody: `{"id": "any-value", "name": "Test"}`,
			wantSuccess:  true,
			wantErrCount: 0,
			description:  "contract-level type rule allows any string id",
		},
		{
			name: "interaction-level rules override contract-level rules",
			interactionRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchRegex, Pattern: "^user-\\d+$"},
			},
			contractRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			responseBody: `{"id": "user-42", "name": "Test"}`,
			wantSuccess:  true,
			wantErrCount: 0,
			description:  "interaction regex rule takes precedence over contract type rule",
		},
		{
			name: "interaction override causes failure when value does not match regex",
			interactionRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchRegex, Pattern: "^user-\\d+$"},
			},
			contractRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			responseBody: `{"id": "not-a-user-id", "name": "Test"}`,
			wantSuccess:  false,
			wantErrCount: 1,
			description:  "interaction regex rule rejects value that contract type rule would accept",
		},
		{
			name: "both levels contribute non-overlapping rules",
			interactionRules: contract.MatchingRules{
				"$.response.body.name": {Match: contract.MatchRegex, Pattern: "^[A-Z]"},
			},
			contractRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			responseBody: `{"id": "anything", "name": "Alice"}`,
			wantSuccess:  true,
			wantErrCount: 0,
			description:  "contract rule for id and interaction rule for name both apply",
		},
		{
			name: "non-overlapping rules with one failing",
			interactionRules: contract.MatchingRules{
				"$.response.body.name": {Match: contract.MatchRegex, Pattern: "^[A-Z]"},
			},
			contractRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			responseBody: `{"id": "anything", "name": "lowercase"}`,
			wantSuccess:  false,
			wantErrCount: 1,
			description:  "interaction regex for name rejects lowercase start",
		},
		{
			name: "nil contract rules do not interfere",
			interactionRules: contract.MatchingRules{
				"$.response.body.id": {Match: contract.MatchTypeValue},
			},
			contractRules: nil,
			responseBody:  `{"id": "any-value", "name": "Test"}`,
			wantSuccess:   true,
			wantErrCount:  0,
			description:   "only interaction rules applied when contract rules are nil",
		},
		{
			name:             "empty rules on both levels use structural matching",
			interactionRules: nil,
			contractRules:    nil,
			responseBody:     `{"id": "example-id", "name": "Test"}`,
			wantSuccess:      true,
			wantErrCount:     0,
			description:      "structural matching when no explicit rules",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := httpvalidator.NewValidator()

			interaction := contract.Interaction{
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Response: contract.HTTPResponse{
							Status: 200,
							Body: map[string]any{
								"id":   "example-id",
								"name": "Test",
							},
						},
					},
				},
				MatchingRules: tt.interactionRules,
			}

			resp := &http.Response{
				StatusCode: 200,
				Header:     http.Header{},
				Body:       io.NopCloser(bytes.NewReader([]byte(tt.responseBody))),
			}

			result := v.ValidateResponse(&interaction, resp, tt.contractRules)

			if result.Success != tt.wantSuccess {
				t.Errorf("[%s] success = %v, want %v; errors: %v",
					tt.description, result.Success, tt.wantSuccess, result.Errors)
			}
			if len(result.Errors) != tt.wantErrCount {
				t.Errorf("[%s] error count = %d, want %d; errors: %v",
					tt.description, len(result.Errors), tt.wantErrCount, result.Errors)
			}
		})
	}
}

func TestValidateResponse_BodyReadError(t *testing.T) {
	v := httpvalidator.NewValidator()

	interaction := newInteractionWithHTTP(t,
		contract.HTTPRequest{},
		contract.HTTPResponse{
			Status: 200,
			Body:   map[string]any{"id": "123"},
		},
		nil,
	)

	resp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{},
		Body:       io.NopCloser(&errReader{err: fmt.Errorf("network timeout")}),
	}

	result := v.ValidateResponse(&interaction, resp, nil)

	if result.Success {
		t.Fatal("expected failure when response body read fails")
	}

	found := false
	for _, e := range result.Errors {
		if strings.Contains(e.Message, "failed to read response body") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error about reading response body, got: %v", result.Errors)
	}
}

func TestValidateRequest_MultipleValidationErrors(t *testing.T) {
	v := httpvalidator.NewValidator()
	interaction := newInteractionWithHTTP(t,
		contract.HTTPRequest{
			Method: "POST",
			Path:   "/api/users",
			Query: map[string]string{
				"version": "2",
			},
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]any{
				"name": "Alice",
			},
		},
		contract.HTTPResponse{},
		nil,
	)

	// Wrong method, wrong path, wrong query, wrong header value, wrong body
	req := makeRequest(t, "GET", "/api/orders?version=1", strings.NewReader(`{"name": 42}`), map[string]string{
		"Content-Type": "text/plain",
	})
	result := v.ValidateRequest(&interaction, req)

	if result.Success {
		t.Fatal("expected failure with multiple mismatches")
	}

	// We expect errors for: method, path, query param, header, body (type mismatch)
	if len(result.Errors) < 4 {
		t.Errorf("expected at least 4 errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify specific error paths are present
	wantPaths := []string{
		"$.request.method",
		"$.request.path",
		"$.request.query.version",
	}
	for _, wantPath := range wantPaths {
		found := false
		for _, e := range result.Errors {
			if e.Path == wantPath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected error at path %q; got errors: %v", wantPath, result.Errors)
		}
	}
}
