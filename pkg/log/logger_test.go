package log

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    Level
		expected string
	}{
		{name: "debug", level: LevelDebug, expected: "DEBUG"},
		{name: "info", level: LevelInfo, expected: "INFO"},
		{name: "warn", level: LevelWarn, expected: "WARN"},
		{name: "error", level: LevelError, expected: "ERROR"},
		{name: "unknown negative", level: Level(-1), expected: "UNKNOWN"},
		{name: "unknown high", level: Level(99), expected: "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.level.String()
			if got != tt.expected {
				t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestFieldConstructors(t *testing.T) {
	t.Run("String", func(t *testing.T) {
		f := String("host", "localhost")
		if f.Key != "host" {
			t.Errorf("String() Key = %q, want %q", f.Key, "host")
		}
		if f.Value != "localhost" {
			t.Errorf("String() Value = %q, want %q", f.Value, "localhost")
		}
	})

	t.Run("Int", func(t *testing.T) {
		f := Int("port", 8080)
		if f.Key != "port" {
			t.Errorf("Int() Key = %q, want %q", f.Key, "port")
		}
		if f.Value != 8080 {
			t.Errorf("Int() Value = %v, want %v", f.Value, 8080)
		}
	})

	t.Run("Int64", func(t *testing.T) {
		f := Int64("size", int64(1234567890))
		if f.Key != "size" {
			t.Errorf("Int64() Key = %q, want %q", f.Key, "size")
		}
		if f.Value != int64(1234567890) {
			t.Errorf("Int64() Value = %v, want %v", f.Value, int64(1234567890))
		}
	})

	t.Run("Bool", func(t *testing.T) {
		f := Bool("verbose", true)
		if f.Key != "verbose" {
			t.Errorf("Bool() Key = %q, want %q", f.Key, "verbose")
		}
		if f.Value != true {
			t.Errorf("Bool() Value = %v, want %v", f.Value, true)
		}
	})

	t.Run("Err", func(t *testing.T) {
		testErr := errors.New("something failed")
		f := Err(testErr)
		if f.Key != "error" {
			t.Errorf("Err() Key = %q, want %q", f.Key, "error")
		}
		if f.Value != testErr {
			t.Errorf("Err() Value = %v, want %v", f.Value, testErr)
		}
	})

	t.Run("Err nil", func(t *testing.T) {
		f := Err(nil)
		if f.Key != "error" {
			t.Errorf("Err(nil) Key = %q, want %q", f.Key, "error")
		}
		if f.Value != nil {
			t.Errorf("Err(nil) Value = %v, want nil", f.Value)
		}
	})

	t.Run("Duration", func(t *testing.T) {
		d := 5 * time.Second
		f := Duration("elapsed", d)
		if f.Key != "elapsed" {
			t.Errorf("Duration() Key = %q, want %q", f.Key, "elapsed")
		}
		if f.Value != d {
			t.Errorf("Duration() Value = %v, want %v", f.Value, d)
		}
	})
}

// newTestLogger creates a logger backed by a bytes.Buffer for test assertions.
func newTestLogger(t *testing.T, level Level) (*bytes.Buffer, Logger) {
	t.Helper()
	var buf bytes.Buffer
	return &buf, NewLogger(&buf, level)
}

// assertContains checks that got contains the substring want.
func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("output %q does not contain %q", got, want)
	}
}

// assertNotContains checks that got does not contain the substring want.
func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("output %q should not contain %q", got, want)
	}
}

func TestLogger_OutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		logFunc    func(Logger)
		wantLevel  string
		wantMsg    string
		wantFields []string
	}{
		{
			name:      "debug message no fields",
			logFunc:   func(l Logger) { l.Debug("starting") },
			wantLevel: "DEBUG",
			wantMsg:   "starting",
		},
		{
			name:      "info message with fields",
			logFunc:   func(l Logger) { l.Info("request", String("method", "GET"), Int("status", 200)) },
			wantLevel: "INFO",
			wantMsg:   "request",
			wantFields: []string{
				"method=GET",
				"status=200",
			},
		},
		{
			name:      "warn message",
			logFunc:   func(l Logger) { l.Warn("slow query", Duration("elapsed", 2*time.Second)) },
			wantLevel: "WARN",
			wantMsg:   "slow query",
			wantFields: []string{
				"elapsed=2s",
			},
		},
		{
			name:      "error message with error field",
			logFunc:   func(l Logger) { l.Error("failed", Err(errors.New("timeout"))) },
			wantLevel: "ERROR",
			wantMsg:   "failed",
			wantFields: []string{
				"error=timeout",
			},
		},
		{
			name:      "message with bool and int64 fields",
			logFunc:   func(l Logger) { l.Info("status", Bool("ok", true), Int64("bytes", 4096)) },
			wantLevel: "INFO",
			wantMsg:   "status",
			wantFields: []string{
				"ok=true",
				"bytes=4096",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, logger := newTestLogger(t, LevelDebug)
			tt.logFunc(logger)

			output := buf.String()

			// Verify the output ends with a newline
			if output == "" {
				t.Fatal("expected log output, got empty string")
			}
			if output[len(output)-1] != '\n' {
				t.Errorf("log output should end with newline, got %q", output)
			}

			// Verify level and message are present
			assertContains(t, output, tt.wantLevel)
			assertContains(t, output, tt.wantMsg)

			// Verify fields are present
			for _, field := range tt.wantFields {
				assertContains(t, output, field)
			}

			// Verify timestamp format (RFC3339 starts with a year)
			parts := strings.SplitN(output, " ", 2)
			if len(parts) < 2 {
				t.Fatalf("expected at least timestamp and content, got %q", output)
			}
			_, err := time.Parse(time.RFC3339, parts[0])
			if err != nil {
				t.Errorf("first token %q is not a valid RFC3339 timestamp: %v", parts[0], err)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		name         string
		loggerLevel  Level
		logFunc      func(Logger)
		shouldOutput bool
	}{
		{
			name:         "debug logger allows debug",
			loggerLevel:  LevelDebug,
			logFunc:      func(l Logger) { l.Debug("msg") },
			shouldOutput: true,
		},
		{
			name:         "debug logger allows info",
			loggerLevel:  LevelDebug,
			logFunc:      func(l Logger) { l.Info("msg") },
			shouldOutput: true,
		},
		{
			name:         "info logger filters debug",
			loggerLevel:  LevelInfo,
			logFunc:      func(l Logger) { l.Debug("msg") },
			shouldOutput: false,
		},
		{
			name:         "info logger allows info",
			loggerLevel:  LevelInfo,
			logFunc:      func(l Logger) { l.Info("msg") },
			shouldOutput: true,
		},
		{
			name:         "info logger allows warn",
			loggerLevel:  LevelInfo,
			logFunc:      func(l Logger) { l.Warn("msg") },
			shouldOutput: true,
		},
		{
			name:         "info logger allows error",
			loggerLevel:  LevelInfo,
			logFunc:      func(l Logger) { l.Error("msg") },
			shouldOutput: true,
		},
		{
			name:         "warn logger filters debug",
			loggerLevel:  LevelWarn,
			logFunc:      func(l Logger) { l.Debug("msg") },
			shouldOutput: false,
		},
		{
			name:         "warn logger filters info",
			loggerLevel:  LevelWarn,
			logFunc:      func(l Logger) { l.Info("msg") },
			shouldOutput: false,
		},
		{
			name:         "warn logger allows warn",
			loggerLevel:  LevelWarn,
			logFunc:      func(l Logger) { l.Warn("msg") },
			shouldOutput: true,
		},
		{
			name:         "warn logger allows error",
			loggerLevel:  LevelWarn,
			logFunc:      func(l Logger) { l.Error("msg") },
			shouldOutput: true,
		},
		{
			name:         "error logger filters debug",
			loggerLevel:  LevelError,
			logFunc:      func(l Logger) { l.Debug("msg") },
			shouldOutput: false,
		},
		{
			name:         "error logger filters info",
			loggerLevel:  LevelError,
			logFunc:      func(l Logger) { l.Info("msg") },
			shouldOutput: false,
		},
		{
			name:         "error logger filters warn",
			loggerLevel:  LevelError,
			logFunc:      func(l Logger) { l.Warn("msg") },
			shouldOutput: false,
		},
		{
			name:         "error logger allows error",
			loggerLevel:  LevelError,
			logFunc:      func(l Logger) { l.Error("msg") },
			shouldOutput: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, logger := newTestLogger(t, tt.loggerLevel)
			tt.logFunc(logger)

			output := buf.String()
			if tt.shouldOutput && output == "" {
				t.Error("expected log output, got none")
			}
			if !tt.shouldOutput && output != "" {
				t.Errorf("expected no output, got %q", output)
			}
		})
	}
}

func TestLogger_WithFields(t *testing.T) {
	t.Run("prepends base fields", func(t *testing.T) {
		buf, logger := newTestLogger(t, LevelDebug)
		child := logger.WithFields(String("service", "api"), Int("version", 2))
		child.Info("hello")

		output := buf.String()
		assertContains(t, output, "service=api")
		assertContains(t, output, "version=2")
		assertContains(t, output, "hello")
	})

	t.Run("merges base and call fields", func(t *testing.T) {
		buf, logger := newTestLogger(t, LevelDebug)
		child := logger.WithFields(String("component", "server"))
		child.Info("request", String("path", "/health"))

		output := buf.String()
		assertContains(t, output, "component=server")
		assertContains(t, output, "path=/health")
	})

	t.Run("does not modify parent", func(t *testing.T) {
		buf, logger := newTestLogger(t, LevelDebug)

		_ = logger.WithFields(String("child", "yes"))
		logger.Info("parent message")

		output := buf.String()
		assertNotContains(t, output, "child=yes")
		assertContains(t, output, "parent message")
	})

	t.Run("chained WithFields", func(t *testing.T) {
		buf, logger := newTestLogger(t, LevelDebug)
		child1 := logger.WithFields(String("a", "1"))
		child2 := child1.WithFields(String("b", "2"))
		child2.Info("deep")

		output := buf.String()
		assertContains(t, output, "a=1")
		assertContains(t, output, "b=2")
		assertContains(t, output, "deep")
	})

	t.Run("preserves level from parent", func(t *testing.T) {
		buf, logger := newTestLogger(t, LevelWarn)
		child := logger.WithFields(String("k", "v"))

		child.Info("should be filtered")
		if buf.String() != "" {
			t.Errorf("expected info to be filtered at warn level, got %q", buf.String())
		}

		child.Warn("should appear")
		assertContains(t, buf.String(), "should appear")
	})
}

func TestNop(t *testing.T) {
	t.Run("does not panic", func(t *testing.T) {
		logger := Nop()
		logger.Debug("debug", String("k", "v"))
		logger.Info("info", Int("n", 1))
		logger.Warn("warn", Bool("ok", false))
		logger.Error("error", Err(errors.New("fail")))
	})

	t.Run("WithFields returns same nop logger", func(t *testing.T) {
		logger := Nop()
		child := logger.WithFields(String("k", "v"))

		// The nop logger's WithFields returns itself
		if logger != child {
			t.Error("Nop().WithFields() should return the same nop logger instance")
		}
	})

	t.Run("implements Logger interface", func(t *testing.T) {
		var _ Logger = Nop() //nolint:staticcheck // interface compliance check
	})
}

func TestDefault(t *testing.T) {
	t.Run("returns non-nil logger", func(t *testing.T) {
		logger := Default()
		if logger == nil {
			t.Fatal("Default() returned nil")
		}
	})

	t.Run("implements Logger interface", func(t *testing.T) {
		var _ Logger = Default() //nolint:staticcheck // interface compliance check
	})
}

func TestNewLogger(t *testing.T) {
	t.Run("returns non-nil logger", func(t *testing.T) {
		var buf bytes.Buffer
		logger := NewLogger(&buf, LevelInfo)
		if logger == nil {
			t.Fatal("NewLogger() returned nil")
		}
	})

	t.Run("implements Logger interface", func(t *testing.T) {
		var buf bytes.Buffer
		var _ Logger = NewLogger(&buf, LevelInfo) //nolint:staticcheck // interface compliance check
	})
}

func TestLogger_OutputFormat_FieldOrder(t *testing.T) {
	buf, logger := newTestLogger(t, LevelDebug)
	logger.Info("test", String("first", "1"), String("second", "2"))

	output := buf.String()

	firstIdx := strings.Index(output, "first=1")
	secondIdx := strings.Index(output, "second=2")

	if firstIdx == -1 || secondIdx == -1 {
		t.Fatalf("expected both fields in output, got %q", output)
	}
	if firstIdx >= secondIdx {
		t.Errorf("fields should appear in order: first=%d, second=%d", firstIdx, secondIdx)
	}
}

func TestLogger_OutputFormat_Structure(t *testing.T) {
	buf, logger := newTestLogger(t, LevelDebug)
	logger.Info("hello world", String("key", "val"))

	output := strings.TrimRight(buf.String(), "\n")
	parts := strings.SplitN(output, " ", 3)

	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts (timestamp level rest), got %d: %q", len(parts), output)
	}

	// First part: RFC3339 timestamp
	if _, err := time.Parse(time.RFC3339, parts[0]); err != nil {
		t.Errorf("first part should be RFC3339 timestamp, got %q: %v", parts[0], err)
	}

	// Second part: level
	if parts[1] != "INFO" {
		t.Errorf("second part should be level INFO, got %q", parts[1])
	}

	// Third part: message and fields
	assertContains(t, parts[2], "hello world")
	assertContains(t, parts[2], "key=val")
}
