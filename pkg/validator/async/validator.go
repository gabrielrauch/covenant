// Package async provides async messaging validation for contract testing.
package async

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/matching"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// Validator implements async message validation.
type Validator struct{}

// NewValidator creates a new async validator.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate validates an async message against the contract interaction.
func (v *Validator) Validate(ctx context.Context, interaction contract.Interaction, actual validator.ActualData) validator.ValidationResult {
	if interaction.Payload.Async == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no async payload",
		})
	}

	result := validator.NewSuccessResult()
	payload := interaction.Payload.Async

	// Validate message headers
	if len(payload.Message.Headers) > 0 && actual.Metadata != nil {
		for key, expectedValue := range payload.Message.Headers {
			actualValue, ok := actual.Metadata[key]
			if !ok {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.message.headers.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected header %s to be present", key),
				})
			} else if actualValue != expectedValue {
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.message.headers.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.message.headers.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected header %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate message payload
	if payload.Message.Payload != nil && len(actual.Response) > 0 {
		payloadResult := v.validatePayload(
			payload.Message.Payload,
			actual.Response,
			"$.message.payload",
			interaction.MatchingRules,
		)
		result.Merge(payloadResult)
	}

	return result
}

// ValidateMessage validates a captured message against expected schema.
func (v *Validator) ValidateMessage(interaction contract.Interaction, message []byte, headers map[string]string) validator.ValidationResult {
	if interaction.Payload.Async == nil {
		return validator.NewFailureResult(validator.ValidationError{
			Message: "interaction has no async payload",
		})
	}

	result := validator.NewSuccessResult()
	expected := interaction.Payload.Async.Message

	// Validate headers
	if len(expected.Headers) > 0 {
		for key, expectedValue := range expected.Headers {
			actualValue, ok := headers[key]
			if !ok {
				result.AddError(validator.ValidationError{
					Path:     fmt.Sprintf("$.message.headers.%s", key),
					Expected: expectedValue,
					Actual:   "(missing)",
					Message:  fmt.Sprintf("expected header %s to be present", key),
				})
			} else if actualValue != expectedValue {
				hasRule := false
				if interaction.MatchingRules != nil {
					_, hasRule = interaction.MatchingRules[fmt.Sprintf("$.message.headers.%s", key)]
				}
				if !hasRule {
					result.AddError(validator.ValidationError{
						Path:     fmt.Sprintf("$.message.headers.%s", key),
						Expected: expectedValue,
						Actual:   actualValue,
						Message:  fmt.Sprintf("expected header %s to be %q, got %q", key, expectedValue, actualValue),
					})
				}
			}
		}
	}

	// Validate payload
	if expected.Payload != nil && len(message) > 0 {
		payloadResult := v.validatePayload(
			expected.Payload,
			message,
			"$.message.payload",
			interaction.MatchingRules,
		)
		result.Merge(payloadResult)
	}

	return result
}

// validatePayload validates a message payload against expected schema.
func (v *Validator) validatePayload(expected any, actualBytes []byte, basePath string, rules contract.MatchingRules) validator.ValidationResult {
	result := validator.NewSuccessResult()

	var actual any
	if err := json.Unmarshal(actualBytes, &actual); err != nil {
		result.AddError(validator.ValidationError{
			Path:    basePath,
			Message: fmt.Sprintf("failed to parse payload as JSON: %v", err),
		})
		return result
	}

	// Build matching engine
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

	// Wrap for path matching
	wrapped := map[string]any{"message": map[string]any{"payload": actual}}

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

	// Structural matching
	structResult := matching.MatchStructure(expected, actual)
	if !structResult.Success {
		for _, err := range structResult.Errors {
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

// SequenceValidator validates ordered sequences of async messages (e.g., sagas).
type SequenceValidator struct {
	sequence  *contract.AsyncSequence
	captured  map[string]any // For correlation tracking
	validator *Validator
	index     int
	errors    []validator.ValidationError
}

// NewSequenceValidator creates a validator for async message sequences.
func NewSequenceValidator(sequence *contract.AsyncSequence) *SequenceValidator {
	return &SequenceValidator{
		sequence:  sequence,
		captured:  make(map[string]any),
		validator: NewValidator(),
	}
}

// ValidateNext validates the next message in the sequence.
func (sv *SequenceValidator) ValidateNext(destination string, payload []byte, headers map[string]string) {
	if sv.index >= len(sv.sequence.Messages) {
		sv.errors = append(sv.errors, validator.ValidationError{
			Message: fmt.Sprintf("unexpected message at position %d", sv.index),
		})
		return
	}

	expected := sv.sequence.Messages[sv.index]

	// Check destination
	if expected.Destination != destination {
		sv.errors = append(sv.errors, validator.ValidationError{
			Path:     fmt.Sprintf("$.sequence[%d].destination", sv.index),
			Expected: expected.Destination,
			Actual:   destination,
			Message:  fmt.Sprintf("expected destination %s, got %s", expected.Destination, destination),
		})
	}

	// Parse payload
	var actualPayload any
	if err := json.Unmarshal(payload, &actualPayload); err != nil {
		sv.errors = append(sv.errors, validator.ValidationError{
			Path:    fmt.Sprintf("$.sequence[%d].payload", sv.index),
			Message: fmt.Sprintf("failed to parse payload: %v", err),
		})
		sv.index++
		return
	}

	// Capture values for correlation
	if len(expected.Capture) > 0 {
		resolver := matching.NewPathResolver()
		for name, path := range expected.Capture {
			value, err := resolver.Resolve(path, actualPayload)
			if err == nil {
				sv.captured[name] = value
			}
		}
	}

	// Validate payload structure
	if expected.Payload != nil {
		structResult := matching.MatchStructure(expected.Payload, actualPayload)
		if !structResult.Success {
			for _, err := range structResult.Errors {
				sv.errors = append(sv.errors, validator.ValidationError{
					Path:     fmt.Sprintf("$.sequence[%d].payload%s", sv.index, err.Path[1:]),
					Expected: err.Expected,
					Actual:   err.Actual,
					Message:  err.Message,
				})
			}
		}
	}

	sv.index++
}

// Captured returns captured values for correlation checking.
func (sv *SequenceValidator) Captured() map[string]any {
	return sv.captured
}

// Result returns the final validation result.
func (sv *SequenceValidator) Result() validator.ValidationResult {
	// Check all messages were received
	if sv.index < len(sv.sequence.Messages) {
		sv.errors = append(sv.errors, validator.ValidationError{
			Message: fmt.Sprintf("sequence incomplete: received %d/%d messages", sv.index, len(sv.sequence.Messages)),
		})
	}

	if len(sv.errors) == 0 {
		return validator.NewSuccessResult()
	}
	return validator.ValidationResult{
		Success: false,
		Errors:  sv.errors,
	}
}
