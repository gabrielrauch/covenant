// Package log provides structured logging for Covenant.
package log

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a logging level.
type Level int

const (
	// LevelDebug is for verbose debugging information.
	LevelDebug Level = iota
	// LevelInfo is for general operational information.
	LevelInfo
	// LevelWarn is for warning messages.
	LevelWarn
	// LevelError is for error messages.
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Field represents a key-value pair for structured logging.
type Field struct {
	Key   string
	Value any
}

// String creates a string field.
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an integer field.
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Int64 creates an int64 field.
func Int64(key string, value int64) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a boolean field.
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field.
func Err(err error) Field {
	return Field{Key: "error", Value: err}
}

// Duration creates a duration field.
func Duration(key string, value time.Duration) Field {
	return Field{Key: key, Value: value}
}

// Logger defines the interface for structured logging.
type Logger interface {
	Debug(msg string, fields ...Field)
	Info(msg string, fields ...Field)
	Warn(msg string, fields ...Field)
	Error(msg string, fields ...Field)
	WithFields(fields ...Field) Logger
}

// defaultLogger is a simple structured logger implementation.
type defaultLogger struct {
	mu     sync.Mutex
	out    io.Writer
	level  Level
	fields []Field
}

// NewLogger creates a new default logger.
func NewLogger(out io.Writer, level Level) Logger {
	return &defaultLogger{
		out:   out,
		level: level,
	}
}

// Default returns a default logger writing to stderr at INFO level.
func Default() Logger {
	return NewLogger(os.Stderr, LevelInfo)
}

// Nop returns a no-op logger that discards all output.
func Nop() Logger {
	return &nopLogger{}
}

func (l *defaultLogger) log(level Level, msg string, fields ...Field) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Build log line
	timestamp := time.Now().UTC().Format(time.RFC3339)
	allFields := l.fields
	allFields = append(allFields, fields...)

	// Format: timestamp level msg key=value key=value ...
	fmt.Fprintf(l.out, "%s %s %s", timestamp, level, msg)
	for _, f := range allFields {
		fmt.Fprintf(l.out, " %s=%v", f.Key, f.Value)
	}
	fmt.Fprintln(l.out)
}

func (l *defaultLogger) Debug(msg string, fields ...Field) {
	l.log(LevelDebug, msg, fields...)
}

func (l *defaultLogger) Info(msg string, fields ...Field) {
	l.log(LevelInfo, msg, fields...)
}

func (l *defaultLogger) Warn(msg string, fields ...Field) {
	l.log(LevelWarn, msg, fields...)
}

func (l *defaultLogger) Error(msg string, fields ...Field) {
	l.log(LevelError, msg, fields...)
}

func (l *defaultLogger) WithFields(fields ...Field) Logger {
	return &defaultLogger{
		out:    l.out,
		level:  l.level,
		fields: append(l.fields, fields...),
	}
}

// nopLogger discards all log output.
type nopLogger struct{}

func (n *nopLogger) Debug(_ string, _ ...Field) {}
func (n *nopLogger) Info(_ string, _ ...Field)  {}
func (n *nopLogger) Warn(_ string, _ ...Field)  {}
func (n *nopLogger) Error(_ string, _ ...Field) {}
func (n *nopLogger) WithFields(_ ...Field) Logger {
	return n
}
