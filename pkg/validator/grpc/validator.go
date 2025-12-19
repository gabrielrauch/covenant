// Package grpc provides gRPC protocol validation for contract testing.
package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/matching"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// Validator implements gRPC protocol validation.
type Validator struct{}

// NewValidator creates a new gRPC validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates a gRPC interaction against actual data.
func (v *Validator) Validate(ctx context.Context, interaction contract.Interaction, actual validator.ActualData) validator.ValidationResult {
	if interaction.Payload.GRPC == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no gRPC payload",
		})
	}

	result := validator.NewSuccessResult()
	payload := interaction.Payload.GRPC

	// Validate response status
	if actual.Status != nil {
		if statusStr, ok := actual.Status.(string); ok {
			if statusStr != payload.Response.Status {
				result.AddError(validator.ValidationError{
					Path:     "$.response.status",
					Expected: payload.Response.Status,
					Actual:   statusStr,
					Message:  fmt.Sprintf("expected status %s, got %s", payload.Response.Status, statusStr),
				})
			}
		}
	}

	// Validate response metadata
	if len(payload.Response.Metadata) > 0 && actual.Metadata != nil {
		for key, expectedValue := range payload.Response.Metadata {
			actualValue, ok := actual.Metadata[key]
			if !ok {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.response.metadata.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected metadata %s to be present", key),
				})
			} else if actualValue != expectedValue {
				// Check for matching rule
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.response.metadata.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.response.metadata.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected metadata %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate response message
	if payload.Response.Message != nil && len(actual.Response) > 0 {
		msgResult := v.validateMessage(
			payload.Response.Message,
			actual.Response,
			"$.response.message",
			interaction.MatchingRules,
		)
		result.Merge(msgResult)
	}

	return result
}

// ValidateRequest validates a gRPC request against the contract.
func (v *Validator) ValidateRequest(interaction contract.Interaction, message []byte, metadata map[string]string) validator.ValidationResult {
	if interaction.Payload.GRPC == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no gRPC payload",
		})
	}

	result := validator.NewSuccessResult()
	expected := interaction.Payload.GRPC.Request

	// Validate metadata
	if len(expected.Metadata) > 0 {
		for key, expectedValue := range expected.Metadata {
			actualValue, ok := metadata[key]
			if !ok {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.request.metadata.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected metadata %s to be present", key),
				})
			} else if actualValue != expectedValue {
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.request.metadata.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.request.metadata.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected metadata %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate message
	if expected.Message != nil && len(message) > 0 {
		msgResult := v.validateMessage(
			expected.Message,
			message,
			"$.request.message",
			interaction.MatchingRules,
		)
		result.Merge(msgResult)
	}

	return result
}

// validateMessage validates a message against expected structure and matching rules.
func (v *Validator) validateMessage(expected any, actualBytes []byte, basePath string, rules contract.MatchingRules) validator.ValidationResult {
	result := validator.NewSuccessResult()

	// Parse actual message (assuming JSON encoding for simplicity)
	var actual any
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		result.AddError(validator.ValidationError{
			Path:    basePath,
			Message: fmt.Sprintf("failed to parse message as JSON: %v", err),
		})
		return result
	}

	// Build matching engine with rules for this path prefix
	engine := matching.NewEngine()
	for path, rule := range rules {
		if len(path) >= len(basePath) && path[:len(basePath)] == basePath {
			if err := engine.LoadRules(contract.MatchingRules{path: rule}); err != nil {
				result.AddError(validator.ValidationError{
					Path:    path,
					Message: fmt.Sprintf("failed to compile matching rule: %v", err),
				})
			}
		}
	}

	// Apply matching rules
	// Wrap actual in the appropriate structure for path matching
	var wrapped any
	if basePath == "$.request.message" {
		wrapped = map[string]any{"request": map[string]any{"message": actual}}
	} else {
		wrapped = map[string]any{"response": map[string]any{"message": actual}}
	}

	matchResult := engine.Match(wrapped)
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

	// Perform structural matching
	structResult := matching.MatchStructure(expected, actual)
	if !structResult.Success {
		for _, err := range structResult.Errors {
			// Skip if already have explicit rule error
			hasError := false
			for _, existing := range result.Errors {
				if existing.Path == basePath+err.Path[1:] {
					hasError = true
					break
				}
			}
			if !hasError {
				result.AddError(validator.ValidationError{
					Path:     basePath + err.Path[1:],
					Expected: err.Expected,
					Actual:   err.Actual,
					Message:  err.Message,
				})
			}
		}
	}

	return result
}

// StreamValidator validates streaming gRPC interactions.
type StreamValidator struct {
	interaction contract.Interaction
	validator   *Validator
	seqIndex    int
	errors      []validator.ValidationError
}

// NewStreamValidator creates a new stream validator for the interaction.
func NewStreamValidator(interaction contract.Interaction) *StreamValidator {
	return &StreamValidator{
		interaction: interaction,
		validator:   NewValidator(),
		seqIndex:    0,
	}
}

// ValidateClientMessage validates a client-sent message in a stream.
func (sv *StreamValidator) ValidateClientMessage(message []byte, metadata map[string]string) {
	if sv.interaction.Payload.GRPC == nil {
		sv.errors = append(sv.errors, validator.ValidationError{
			Message: "interaction has no gRPC payload",
		})
		return
	}

	payload := sv.interaction.Payload.GRPC

	// For bidirectional streams, check sequence
	if payload.StreamType == contract.StreamTypeBidirectional && len(payload.Sequence) > 0 {
		if sv.seqIndex >= len(payload.Sequence) {
			sv.errors = append(sv.errors, validator.ValidationError{
				Message: fmt.Sprintf("unexpected message at sequence position %d", sv.seqIndex),
			})
			return
		}

		expected := payload.Sequence[sv.seqIndex]
		if expected.Direction != contract.StreamDirectionClient {
			sv.errors = append(sv.errors, validator.ValidationError{
				Message: fmt.Sprintf("expected server message at position %d, got client message", sv.seqIndex),
			})
		}

		// Validate message content
		if expected.Message != nil {
			msgResult := sv.validator.validateMessage(
				expected.Message,
				message,
				fmt.Sprintf("$.sequence[%d].message", sv.seqIndex),
				sv.interaction.MatchingRules,
			)
			sv.errors = append(sv.errors, msgResult.Errors...)
		}

		sv.seqIndex++
	} else if payload.StreamType == contract.StreamTypeClientStream {
		// For client streaming, validate against the request schema
		result := sv.validator.ValidateRequest(sv.interaction, message, metadata)
		sv.errors = append(sv.errors, result.Errors...)
	}
}

// ValidateServerMessage validates a server-sent message in a stream.
func (sv *StreamValidator) ValidateServerMessage(message []byte, metadata map[string]string) {
	if sv.interaction.Payload.GRPC == nil {
		sv.errors = append(sv.errors, validator.ValidationError{
			Message: "interaction has no gRPC payload",
		})
		return
	}

	payload := sv.interaction.Payload.GRPC

	// For bidirectional streams, check sequence
	if payload.StreamType == contract.StreamTypeBidirectional && len(payload.Sequence) > 0 {
		if sv.seqIndex >= len(payload.Sequence) {
			sv.errors = append(sv.errors, validator.ValidationError{
				Message: fmt.Sprintf("unexpected message at sequence position %d", sv.seqIndex),
			})
			return
		}

		expected := payload.Sequence[sv.seqIndex]
		if expected.Direction != contract.StreamDirectionServer {
			sv.errors = append(sv.errors, validator.ValidationError{
				Message: fmt.Sprintf("expected client message at position %d, got server message", sv.seqIndex),
			})
		}

		// Validate message content
		if expected.Message != nil {
			msgResult := sv.validator.validateMessage(
				expected.Message,
				message,
				fmt.Sprintf("$.sequence[%d].message", sv.seqIndex),
				sv.interaction.MatchingRules,
			)
			sv.errors = append(sv.errors, msgResult.Errors...)
		}

		sv.seqIndex++
	} else if payload.StreamType == contract.StreamTypeServerStream {
		// For server streaming, validate against response schema
		msgResult := sv.validator.validateMessage(
			payload.Response.Message,
			message,
			"$.response.message",
			sv.interaction.MatchingRules,
		)
		sv.errors = append(sv.errors, msgResult.Errors...)
	}
}

// Result returns the validation result after all messages have been processed.
func (sv *StreamValidator) Result() validator.ValidationResult {
	if len(sv.errors) == 0 {
		return validator.NewSuccessResult()
	}
	return validator.ValidationResult{
		Success: false,
		Errors:  sv.errors,
	}
}
