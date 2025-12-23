// Package pathutil provides path sanitization utilities.
package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SanitizeAbsolutePath validates and cleans a file path.
// If the path is relative, it is converted to an absolute path.
// It ensures the path is cleaned and contains no traversal patterns.
func SanitizeAbsolutePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)

	// Convert relative paths to absolute
	if !filepath.IsAbs(cleanPath) {
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
		cleanPath = absPath
	}

	// Check for path traversal patterns (should be cleaned by filepath.Clean/Abs, but verify)
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path traversal not allowed: %s", path)
	}
	return cleanPath, nil
}

// ValidateWithinRoot checks that a path is within the specified root directory.
// Returns the cleaned absolute path if valid.
func ValidateWithinRoot(root, path string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("failed to resolve root directory: %w", err)
	}

	cleanPath := filepath.Clean(path)
	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(absRoot, cleanPath)
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure the path is within root (add separator to prevent prefix matching issues)
	if !strings.HasPrefix(absPath, absRoot+string(filepath.Separator)) && absPath != absRoot {
		return "", fmt.Errorf("path %q is outside root directory", path)
	}

	return absPath, nil
}
