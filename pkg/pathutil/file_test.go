package pathutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecureOpen(t *testing.T) {
	// Create a temp directory with a test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid absolute path",
			path:    testFile,
			wantErr: false,
		},
		{
			name:    "valid relative path",
			path:    "file_test.go", // This file exists
			wantErr: false,
		},
		{
			name:    "non-existent file",
			path:    filepath.Join(tmpDir, "nonexistent.txt"),
			wantErr: true,
			errMsg:  "failed to open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := SecureOpen(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SecureOpen(%q) expected error, got nil", tt.path)
					if f != nil {
						f.Close()
					}
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SecureOpen(%q) error = %v, want error containing %q", tt.path, err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("SecureOpen(%q) unexpected error: %v", tt.path, err)
				return
			}

			if f == nil {
				t.Errorf("SecureOpen(%q) returned nil file without error", tt.path)
				return
			}

			// Verify we can read from the file
			buf := make([]byte, 100)
			_, err = f.Read(buf)
			if err != nil && err.Error() != "EOF" {
				t.Errorf("SecureOpen(%q) file not readable: %v", tt.path, err)
			}

			f.Close()
		})
	}
}

func TestSecureCreate(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "create new file",
			path:    filepath.Join(tmpDir, "new_file.txt"),
			wantErr: false,
		},
		{
			name:    "create in nested directory that exists",
			path:    filepath.Join(tmpDir, "subdir", "file.txt"),
			wantErr: true, // Directory doesn't exist
			errMsg:  "failed to create file",
		},
	}

	// Create the subdir for one test
	if err := os.MkdirAll(filepath.Join(tmpDir, "existing_subdir"), 0750); err != nil {
		t.Fatalf("failed to create test subdir: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := SecureCreate(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SecureCreate(%q) expected error, got nil", tt.path)
					if f != nil {
						f.Close()
						os.Remove(tt.path)
					}
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SecureCreate(%q) error = %v, want error containing %q", tt.path, err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("SecureCreate(%q) unexpected error: %v", tt.path, err)
				return
			}

			if f == nil {
				t.Errorf("SecureCreate(%q) returned nil file without error", tt.path)
				return
			}

			// Verify we can write to the file
			_, err = f.WriteString("test content")
			if err != nil {
				t.Errorf("SecureCreate(%q) file not writable: %v", tt.path, err)
			}

			f.Close()

			// Verify the file was created
			if _, err := os.Stat(tt.path); os.IsNotExist(err) {
				t.Errorf("SecureCreate(%q) file was not created", tt.path)
			}
		})
	}
}

func TestSecureReadFile(t *testing.T) {
	// Create a temp directory with test files
	root := t.TempDir()
	testFile := filepath.Join(root, "test.txt")
	testContent := []byte("test content for reading")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a subdirectory with a file
	subDir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	subFile := filepath.Join(subDir, "nested.txt")
	subContent := []byte("nested content")
	if err := os.WriteFile(subFile, subContent, 0600); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	tests := []struct {
		name        string
		root        string
		path        string
		wantContent []byte
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "read file with relative path",
			root:        root,
			path:        "test.txt",
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name:        "read nested file",
			root:        root,
			path:        "subdir/nested.txt",
			wantContent: subContent,
			wantErr:     false,
		},
		{
			name:        "read with absolute path within root",
			root:        root,
			path:        testFile,
			wantContent: testContent,
			wantErr:     false,
		},
		{
			name:    "path traversal blocked",
			root:    root,
			path:    "../etc/passwd",
			wantErr: true,
			errMsg:  "outside root directory",
		},
		{
			name:    "absolute path outside root blocked",
			root:    root,
			path:    "/etc/passwd",
			wantErr: true,
			errMsg:  "outside root directory",
		},
		{
			name:    "non-existent file",
			root:    root,
			path:    "nonexistent.txt",
			wantErr: true,
			errMsg:  "no such file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := SecureReadFile(tt.root, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SecureReadFile(%q, %q) expected error, got nil", tt.root, tt.path)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("SecureReadFile(%q, %q) error = %v, want error containing %q", tt.root, tt.path, err, tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("SecureReadFile(%q, %q) unexpected error: %v", tt.root, tt.path, err)
				return
			}

			if !bytes.Equal(content, tt.wantContent) {
				t.Errorf("SecureReadFile(%q, %q) = %q, want %q", tt.root, tt.path, content, tt.wantContent)
			}
		})
	}
}

func TestReadValidated(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("validated content")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test successful read
	content, err := ReadValidated(testFile)
	if err != nil {
		t.Errorf("ReadValidated(%q) unexpected error: %v", testFile, err)
	}
	if !bytes.Equal(content, testContent) {
		t.Errorf("ReadValidated(%q) = %q, want %q", testFile, content, testContent)
	}

	// Test non-existent file - error should be compatible with os.IsNotExist
	_, err = ReadValidated(filepath.Join(tmpDir, "nonexistent.txt"))
	if err == nil {
		t.Error("ReadValidated should return error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("ReadValidated error should be compatible with os.IsNotExist, got: %v", err)
	}
}

func TestSecureOpen_PathTraversal(t *testing.T) {
	// These paths should be sanitized successfully (paths cleaned by filepath.Clean)
	// The actual file doesn't need to exist for the path validation to work
	tests := []struct {
		name string
		path string
	}{
		{
			name: "parent reference in absolute path",
			path: "/tmp/../tmp/test.txt",
		},
		{
			name: "double slashes",
			path: "/tmp//test///file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// SecureOpen should not return a path validation error
			// (file not existing is a different error)
			_, err := SecureOpen(tt.path)
			if err != nil && strings.Contains(err.Error(), "path traversal") {
				t.Errorf("SecureOpen(%q) incorrectly rejected as path traversal: %v", tt.path, err)
			}
		})
	}
}
