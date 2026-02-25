package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name: "version command",
			args: []string{"version"},
		},
		{
			name: "help command",
			args: []string{"help"},
		},
		{
			name: "dash-h flag",
			args: []string{"-h"},
		},
		{
			name: "double-dash-help flag",
			args: []string{"--help"},
		},
		{
			name:    "empty args shows usage",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "unknown command returns error",
			args:    []string{"nonexistent"},
			wantErr: true,
			errMsg:  "unknown command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureStdout(t, func() {
				err := run(context.Background(), tt.args)
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("error %q does not contain %q", err.Error(), tt.errMsg)
					}
				} else if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
	}
}

func TestRun_VersionOutput(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run(context.Background(), []string{"version"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "covenant") {
		t.Errorf("version output should contain 'covenant', got: %q", output)
	}
	if !strings.Contains(output, version) {
		t.Errorf("version output should contain version %q, got: %q", version, output)
	}
}

func TestRun_HelpOutput(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run(context.Background(), []string{"help"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	expectedSubstrings := []string{
		"covenant",
		"publish",
		"fetch",
		"verify",
		"can-deploy",
		"tag",
		"broker",
		"version",
	}

	for _, s := range expectedSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("help output should contain %q, got:\n%s", s, output)
		}
	}
}

func TestRun_CommandDispatching(t *testing.T) {
	// Verify commands that need flags return an error when flags are missing
	tests := []struct {
		name string
		args []string
	}{
		{name: "verify without flags", args: []string{"verify"}},
		{name: "can-deploy without flags", args: []string{"can-deploy"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureStdout(t, func() {
				err := run(context.Background(), tt.args)
				if err == nil {
					t.Error("expected error for missing required flags")
				}
			})
		})
	}
}

func TestMainWithExitCode(t *testing.T) {
	// Save and restore os.Args
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("version returns 0", func(t *testing.T) {
		os.Args = []string{"covenant", "version"}
		captureStdout(t, func() {
			code := mainWithExitCode()
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
		})
	})

	t.Run("unknown command returns 1", func(t *testing.T) {
		os.Args = []string{"covenant", "bogus-command"}
		captureStdout(t, func() {
			// Capture stderr too since error goes there
			captureStderr(t, func() {
				code := mainWithExitCode()
				if code != 1 {
					t.Errorf("exit code = %d, want 1", code)
				}
			})
		})
	})
}

func TestPrintUsage(t *testing.T) {
	output := captureStdout(t, func() {
		err := printUsage()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(output, "covenant") {
		t.Errorf("usage should contain 'covenant', got: %q", output)
	}
	if !strings.Contains(output, "Commands:") {
		t.Errorf("usage should contain 'Commands:', got: %q", output)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	r.Close()
	return buf.String()
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stderr: %v", err)
	}
	r.Close()
	return buf.String()
}
