// Package validation provides shared validation utilities across protocol validators.
package validation

import (
	"fmt"
	"strings"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// HeaderValidationConfig configures header/metadata validation behavior.
type HeaderValidationConfig struct {
	// BasePath is the JSON path prefix for error reporting (e.g., "$.response.headers")
	BasePath string
	// FieldName is the human-readable name for error messages (e.g., "header", "metadata")
	FieldName string
	// CaseSensitive controls whether key lookup is case-sensitive (default: false for HTTP headers)
	CaseSensitive bool
}

// ValidateHeaders validates expected headers/metadata against actual values,
// respecting matching rules where defined.
//
// This is the common implementation used by HTTP, gRPC, and async validators
// to eliminate code duplication.
func ValidateHeaders(
	expected map[string]string,
	actual map[string]string,
	rules contract.MatchingRules,
	config HeaderValidationConfig,
) []validator.ValidationError {
	if len(expected) == 0 || actual == nil {
		return nil
	}

	var errors []validator.ValidationError
	fieldName := config.FieldName
	if fieldName == "" {
		fieldName = "header"
	}

	for key, expectedValue := range expected {
		actualValue, ok := actual[key]

		// Try case-insensitive lookup if configured and not found
		if !ok && !config.CaseSensitive {
			actualValue, ok = actual[strings.ToLower(key)]
		}

		path := fmt.Sprintf("%s.%s", config.BasePath, key)

		if !ok {
			errors = append(errors, validator.ValidationError{
				Path:     path,
				Expected: expectedValue,
				Actual:   "(missing)",
				Message:  fmt.Sprintf("expected %s %s to be present", fieldName, key),
			})
			continue
		}

		if actualValue != expectedValue {
			// Check if there's a matching rule for this path
			hasRule := false
			if rules != nil {
				_, hasRule = rules[path]
			}

			if !hasRule {
				errors = append(errors, validator.ValidationError{
					Path:     path,
					Expected: expectedValue,
					Actual:   actualValue,
					Message:  fmt.Sprintf("expected %s %s to be %q, got %q", fieldName, key, expectedValue, actualValue),
				})
			}
		}
	}

	return errors
}

// DefaultHTTPHeaderConfig returns the default configuration for HTTP header validation.
func DefaultHTTPHeaderConfig(basePath string) HeaderValidationConfig {
	return HeaderValidationConfig{
		BasePath:      basePath,
		FieldName:     "header",
		CaseSensitive: false, // HTTP headers are case-insensitive
	}
}

// DefaultGRPCMetadataConfig returns the default configuration for gRPC metadata validation.
func DefaultGRPCMetadataConfig(basePath string) HeaderValidationConfig {
	return HeaderValidationConfig{
		BasePath:      basePath,
		FieldName:     "metadata",
		CaseSensitive: true, // gRPC metadata keys are case-sensitive
	}
}

// DefaultAsyncHeaderConfig returns the default configuration for async message header validation.
func DefaultAsyncHeaderConfig(basePath string) HeaderValidationConfig {
	return HeaderValidationConfig{
		BasePath:      basePath,
		FieldName:     "header",
		CaseSensitive: true,
	}
}
