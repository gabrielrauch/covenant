// Package validator provides protocol-specific validators for contract verification.
package validator

import (
	"context"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// Validator validates actual data against contract interactions.
type Validator interface {
	// Validate checks if actual data matches the contract interaction.
	Validate(ctx context.Context, interaction contract.Interaction, actual ActualData) ValidationResult
}

// ActualData represents captured request/response data for validation.
type ActualData struct {
	Request  []byte            // Serialized request/message
	Response []byte            // Serialized response/message
	Metadata map[string]string // Headers, gRPC metadata, message headers
	Status   any               // HTTP status code, gRPC status, etc.
}

// ValidationResult contains the outcome of a validation.
type ValidationResult struct {
	Success bool
	Errors  []ValidationError
}

// ValidationError represents a single validation failure.
type ValidationError struct {
	Path     string // JSONPath to mismatched field
	Expected string // What contract specified
	Actual   string // What was received
	Rule     string // Which matching rule failed
	Message  string // Human-readable explanation
}

// AddError adds a validation error to the result and marks it as failed.
func (r *ValidationResult) AddError(err ValidationError) {
	r.Success = false
	r.Errors = append(r.Errors, err)
}

// Merge combines another result into this one.
func (r *ValidationResult) Merge(other ValidationResult) {
	if !other.Success {
		r.Success = false
	}
	r.Errors = append(r.Errors, other.Errors...)
}

// NewSuccessResult creates a successful validation result.
func NewSuccessResult() ValidationResult {
	return ValidationResult{Success: true}
}

// NewFailureResult creates a failed validation result with the given errors.
func NewFailureResult(errors ...ValidationError) ValidationResult {
	return ValidationResult{
		Success: false,
		Errors:  errors,
	}
}
