// Package http provides HTTP protocol validation for contract testing.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/matching"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// Validator implements HTTP protocol validation.
type Validator struct {
	client *http.Client
}

// NewValidator creates a new HTTP validator.
func NewValidator() *Validator {
	return &Validator{
		client: &http.Client{},
	}
}

// WithClient sets a custom HTTP client.
func (v *Validator) WithClient(client *http.Client) *Validator {
	v.client = client
	return v
}

// Validate validates an HTTP interaction against actual data.
func (v *Validator) Validate(ctx context.Context, interaction contract.Interaction, actual validator.ActualData) validator.ValidationResult {
	if interaction.Payload.HTTP == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no HTTP payload",
		})
	}

	result := validator.NewSuccessResult()
	payload := interaction.Payload.HTTP

	// Validate response status
	if actual.Status != nil {
		if statusCode, ok := actual.Status.(int); ok {
			if statusCode != payload.Response.Status {
				result.AddError(validator.ValidationError{
					Path:     "$.response.status",
					Expected: fmt.Sprintf("%d", payload.Response.Status),
					Actual:   fmt.Sprintf("%d", statusCode),
					Message:  fmt.Sprintf("expected status %d, got %d", payload.Response.Status, statusCode),
				})
			}
		}
	}

	// Validate response headers
	if len(payload.Response.Headers) > 0 && actual.Metadata != nil {
		for key, expectedValue := range payload.Response.Headers {
			actualValue, ok := actual.Metadata[key]
			if !ok {
				actualValue, ok = actual.Metadata[strings.ToLower(key)]
			}
			if !ok {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.response.headers.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected header %s to be present", key),
				})
			} else if actualValue != expectedValue {
				// Check if there's a matching rule for this header
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.response.headers.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.response.headers.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected header %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate response body
	if payload.Response.Body != nil && len(actual.Response) > 0 {
		bodyResult := v.validateBody(
			payload.Response.Body,
			actual.Response,
			"$.response.body",
			interaction.MatchingRules,
		)
		result.Merge(bodyResult)
	}

	return result
}

// ValidateRequest validates an HTTP request against the contract.
func (v *Validator) ValidateRequest(interaction contract.Interaction, req *http.Request) validator.ValidationResult {
	if interaction.Payload.HTTP == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no HTTP payload",
		})
	}

	result := validator.NewSuccessResult()
	expected := interaction.Payload.HTTP.Request

	// Validate method
	if req.Method != expected.Method {
		result.AddError(validator.ValidationError{
			Path:     "$.request.method",
			Expected: expected.Method,
			Actual:   req.Method,
			Message:  fmt.Sprintf("expected method %s, got %s", expected.Method, req.Method),
		})
	}

	// Validate path
	if req.URL.Path != expected.Path {
		result.AddError(validator.ValidationError{
			Path:     "$.request.path",
			Expected: expected.Path,
			Actual:   req.URL.Path,
			Message:  fmt.Sprintf("expected path %s, got %s", expected.Path, req.URL.Path),
		})
	}

	// Validate query parameters
	if len(expected.Query) > 0 {
		actualQuery := req.URL.Query()
		for key, expectedValue := range expected.Query {
			actualValue := actualQuery.Get(key)
			if actualValue != expectedValue {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.request.query.%s", key),
					Expected: expectedValue,
					Actual:   actualValue,
					Message:  fmt.Sprintf("expected query param %s to be %q, got %q", key, expectedValue, actualValue),
				})
			}
		}
	}

	// Validate headers
	if len(expected.Headers) > 0 {
		for key, expectedValue := range expected.Headers {
			actualValue := req.Header.Get(key)
			if actualValue == "" {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.request.headers.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected header %s to be present", key),
				})
			} else if actualValue != expectedValue {
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.request.headers.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.request.headers.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected header %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate body
	if expected.Body != nil && req.Body != nil {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // Reset body
			bodyResult := v.validateBody(
				expected.Body,
				bodyBytes,
				"$.request.body",
				interaction.MatchingRules,
			)
			result.Merge(bodyResult)
		}
	}

	return result
}

// validateBody validates a body against expected structure and matching rules.
func (v *Validator) validateBody(expected any, actualBytes []byte, basePath string, rules contract.MatchingRules) validator.ValidationResult {
	result := validator.NewSuccessResult()

	// Parse actual body
	var actual any
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		result.AddError(validator.ValidationError{
			Path:    basePath,
			Message: fmt.Sprintf("failed to parse response body as JSON: %v", err),
		})
		return result
	}

	// Build matching engine with rules for this path prefix
	engine := matching.NewEngine()
	for path, rule := range rules {
		if strings.HasPrefix(path, basePath) {
			if err := engine.LoadRules(contract.MatchingRules{path: rule}); err != nil {
				result.AddError(validator.ValidationError{
					Path:    path,
					Message: fmt.Sprintf("failed to compile matching rule: %v", err),
				})
			}
		}
	}

	// First, apply explicit matching rules
	matchResult := engine.Match(map[string]any{"response": map[string]any{"body": actual}})
	if !matchResult.Success {
		for _, err := range matchResult.Errors {
			result.AddError(validator.ValidationError{
				Path:     err.Path,
				Expected: err.Expected,
				Actual:   err.Actual,
				Rule:     err.Rule,
				Message:  err.Message,
			})
		}
	}

	// Then, perform structural matching for fields without explicit rules
	structResult := matching.MatchStructure(expected, actual)
	if !structResult.Success {
		for _, err := range structResult.Errors {
			// Skip if there's already an explicit rule error for this path
			hasExplicitError := false
			for _, existing := range result.Errors {
				if existing.Path == basePath+err.Path[1:] {
					hasExplicitError = true
					break
				}
			}
			if !hasExplicitError {
				result.AddError(validator.ValidationError{
					Path:     basePath + err.Path[1:], // Replace $ with basePath
					Expected: err.Expected,
					Actual:   err.Actual,
					Message:  err.Message,
				})
			}
		}
	}

	return result
}

// ExecuteInteraction executes an HTTP interaction against a provider and returns the response.
func (v *Validator) ExecuteInteraction(ctx context.Context, baseURL string, interaction contract.Interaction) (*http.Response, error) {
	if interaction.Payload.HTTP == nil {
		return nil, fmt.Errorf("interaction has no HTTP payload")
	}

	req, err := v.buildRequest(ctx, baseURL, interaction)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	return v.client.Do(req)
}

// buildRequest builds an HTTP request from a contract interaction.
func (v *Validator) buildRequest(ctx context.Context, baseURL string, interaction contract.Interaction) (*http.Request, error) {
	expected := interaction.Payload.HTTP.Request

	// Build URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	u.Path = expected.Path

	// Add query parameters
	if len(expected.Query) > 0 {
		q := u.Query()
		for key, value := range expected.Query {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
	}

	// Build body
	var body io.Reader
	if expected.Body != nil {
		bodyBytes, err := json.Marshal(expected.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(bodyBytes)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, expected.Method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	for key, value := range expected.Headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

// ValidateResponse validates an HTTP response against the contract.
func (v *Validator) ValidateResponse(interaction contract.Interaction, resp *http.Response, contractRules contract.MatchingRules) validator.ValidationResult {
	if interaction.Payload.HTTP == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no HTTP payload",
		})
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return validator.NewFailureResult(validator.ValidationError{
			Path:    "$.response.body",
			Message: fmt.Sprintf("failed to read response body: %v", err),
		})
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // Reset body

	// Build metadata from headers
	metadata := make(map[string]string)
	for key, values := range resp.Header {
		if len(values) > 0 {
			metadata[key] = values[0]
			metadata[strings.ToLower(key)] = values[0]
		}
	}

	// Merge contract-level rules with interaction-level rules
	mergedRules := make(contract.MatchingRules)
	for k, v := range contractRules {
		mergedRules[k] = v
	}
	for k, v := range interaction.MatchingRules {
		mergedRules[k] = v
	}

	// Create interaction with merged rules for validation
	mergedInteraction := interaction
	mergedInteraction.MatchingRules = mergedRules

	return v.Validate(context.Background(), mergedInteraction, validator.ActualData{
		Response: bodyBytes,
		Metadata: metadata,
		Status:   resp.StatusCode,
	})
}
