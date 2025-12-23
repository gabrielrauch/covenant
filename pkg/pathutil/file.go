// Package pathutil provides secure file path handling utilities.
package pathutil

import (
	"fmt"
	"os"
)

// SecureOpen opens a file after validating the path is absolute and safe.
// This prevents path traversal attacks by validating before opening.
func SecureOpen(path string) (*os.File, error) {
	cleanPath, err := SanitizeAbsolutePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return f, nil
}

// SecureCreate creates a file after validating the path is absolute and safe.
// This prevents path traversal attacks by validating before creating.
func SecureCreate(path string) (*os.File, error) {
	cleanPath, err := SanitizeAbsolutePath(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	f, err := os.Create(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	return f, nil
}

// SecureReadFile reads a file after validating the path is within a root directory.
// This prevents path traversal attacks by ensuring the path stays within bounds.
func SecureReadFile(root, path string) ([]byte, error) {
	validPath, err := ValidateWithinRoot(root, path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	return ReadValidated(validPath)
}

// ReadValidated reads a file from a path that has already been validated.
// This should only be called with paths returned from ValidateWithinRoot
// or SanitizeAbsolutePath to ensure they are safe from path traversal attacks.
// The error is returned unwrapped to preserve os.IsNotExist compatibility.
func ReadValidated(validatedPath string) ([]byte, error) {
	return os.ReadFile(validatedPath)
}
