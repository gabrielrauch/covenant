package pathutil

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSanitizeAbsolutePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "absolute path",
			path:    "/tmp/test/file.txt",
			wantErr: false,
		},
		{
			name:    "relative path converted to absolute",
			path:    "test/file.txt",
			wantErr: false,
		},
		{
			name:    "current directory",
			path:    ".",
			wantErr: false,
		},
		{
			name:    "parent reference cleaned",
			path:    "/tmp/test/../other/file.txt",
			wantErr: false,
		},
		{
			name:    "double slashes cleaned",
			path:    "/tmp//test///file.txt",
			wantErr: false,
		},
		{
			name:    "empty path becomes current dir",
			path:    "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeAbsolutePath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SanitizeAbsolutePath(%q) expected error containing %q, got nil", tt.path, tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SanitizeAbsolutePath(%q) error = %v, want error containing %q", tt.path, err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("SanitizeAbsolutePath(%q) unexpected error: %v", tt.path, err)
				return
			}

			// Verify result is absolute
			if !filepath.IsAbs(result) {
				t.Errorf("SanitizeAbsolutePath(%q) = %q, want absolute path", tt.path, result)
			}

			// Verify result is clean (no .. or double slashes)
			if strings.Contains(result, "..") {
				t.Errorf("SanitizeAbsolutePath(%q) = %q, contains path traversal", tt.path, result)
			}
		})
	}
}

func TestValidateWithinRoot(t *testing.T) {
	// Use temp directory for consistent testing
	root := t.TempDir()

	tests := []struct {
		name    string
		root    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid relative path",
			root:    root,
			path:    "subdir/file.txt",
			wantErr: false,
		},
		{
			name:    "valid absolute path within root",
			root:    root,
			path:    filepath.Join(root, "subdir", "file.txt"),
			wantErr: false,
		},
		{
			name:    "root itself is valid",
			root:    root,
			path:    root,
			wantErr: false,
		},
		{
			name:    "path traversal attempt - parent reference",
			root:    root,
			path:    "../etc/passwd",
			wantErr: true,
			errMsg:  "outside root directory",
		},
		{
			name:    "path traversal attempt - absolute outside root",
			root:    root,
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "outside root directory",
		},
		{
			name:    "path traversal attempt - sneaky parent in middle",
			root:    root,
			path:    "subdir/../../etc/passwd",
			wantErr: true,
			errMsg:  "outside root directory",
		},
		{
			name:    "empty path uses root",
			root:    root,
			path:    "",
			wantErr: false,
		},
		{
			name:    "dot path uses root",
			root:    root,
			path:    ".",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateWithinRoot(tt.root, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateWithinRoot(%q, %q) expected error containing %q, got nil", tt.root, tt.path, tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateWithinRoot(%q, %q) error = %v, want error containing %q", tt.root, tt.path, err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateWithinRoot(%q, %q) unexpected error: %v", tt.root, tt.path, err)
				return
			}

			// Verify result starts with root
			absRoot, err := filepath.Abs(tt.root)
			if err != nil {
				t.Fatalf("failed to get absolute path for root: %v", err)
			}
			if !strings.HasPrefix(result, absRoot) {
				t.Errorf("ValidateWithinRoot(%q, %q) = %q, not within root %q", tt.root, tt.path, result, absRoot)
			}

			// Verify result is absolute
			if !filepath.IsAbs(result) {
				t.Errorf("ValidateWithinRoot(%q, %q) = %q, want absolute path", tt.root, tt.path, result)
			}
		})
	}
}

func TestValidateWithinRoot_PrefixAttack(t *testing.T) {
	// Test that "/root" doesn't match paths starting with "/rootsomething"
	// This is a common security bug in path validation
	root := t.TempDir()

	// Create a sibling directory that starts with the same prefix
	siblingPath := root + "_sibling/file.txt"

	_, err := ValidateWithinRoot(root, siblingPath)
	if err == nil {
		t.Errorf("ValidateWithinRoot should reject paths that only share prefix: root=%q, path=%q", root, siblingPath)
	}
}

func TestValidateWithinRoot_SymlinkTraversal(t *testing.T) {
	// Skip on Windows where symlinks require elevated privileges
	if runtime.GOOS == "windows" {
		t.Skip("symlink test skipped on Windows")
	}

	// This test documents current behavior - filepath operations follow symlinks
	// which means symlink traversal is NOT prevented by this validation alone
	// Real protection requires checking the resolved path after following symlinks
	t.Skip("symlink traversal prevention requires additional OS-level checks")
}
