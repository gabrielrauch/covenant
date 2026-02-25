package matching

import (
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestEngine_Match_ExactRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "test"},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "exact match",
			data:    map[string]any{"name": "test"},
			success: true,
		},
		{
			name:    "value mismatch",
			data:    map[string]any{"name": "other"},
			success: false,
		},
		{
			name:    "missing field",
			data:    map[string]any{"other": "value"},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_TypeRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.id": {Match: contract.MatchTypeValue},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "string value",
			data:    map[string]any{"id": "abc-123"},
			success: true,
		},
		{
			name:    "number value",
			data:    map[string]any{"id": 12345},
			success: true,
		},
		{
			name:    "null value",
			data:    map[string]any{"id": nil},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_RegexRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.email": {Match: contract.MatchRegex, Pattern: `^[a-z]+@[a-z]+\.[a-z]+$`},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "valid email",
			data:    map[string]any{"email": "test@example.com"},
			success: true,
		},
		{
			name:    "invalid email",
			data:    map[string]any{"email": "not-an-email"},
			success: false,
		},
		{
			name:    "non-string value",
			data:    map[string]any{"email": 12345},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_IntegerRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.count": {Match: contract.MatchInteger},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "integer value",
			data:    map[string]any{"count": 42},
			success: true,
		},
		{
			name:    "float with integer value",
			data:    map[string]any{"count": 42.0},
			success: true,
		},
		{
			name:    "float with decimal",
			data:    map[string]any{"count": 42.5},
			success: false,
		},
		{
			name:    "string value",
			data:    map[string]any{"count": "42"},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_DecimalRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.price": {Match: contract.MatchDecimal},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "float value",
			data:    map[string]any{"price": 19.99},
			success: true,
		},
		{
			name:    "integer value (also valid)",
			data:    map[string]any{"price": 20},
			success: true,
		},
		{
			name:    "string value",
			data:    map[string]any{"price": "19.99"},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_IncludeRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.message": {Match: contract.MatchInclude, Value: "error"},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "contains substring",
			data:    map[string]any{"message": "An error occurred"},
			success: true,
		},
		{
			name:    "does not contain",
			data:    map[string]any{"message": "All good"},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_OptionalRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.optional_field": {Match: contract.MatchOptional},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "field present",
			data:    map[string]any{"optional_field": "value"},
			success: true,
		},
		{
			name:    "field missing",
			data:    map[string]any{"other_field": "value"},
			success: true, // Optional allows missing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_NullOrRule(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.nullable_id": {
			Match: contract.MatchNullOr,
			Rule: &contract.MatchingRule{
				Match:   contract.MatchRegex,
				Pattern: `^[a-z0-9-]+$`,
			},
		},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name:    "null value",
			data:    map[string]any{"nullable_id": nil},
			success: true,
		},
		{
			name:    "valid value",
			data:    map[string]any{"nullable_id": "abc-123"},
			success: true,
		},
		{
			name:    "invalid value",
			data:    map[string]any{"nullable_id": "INVALID!"},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_Match_NestedPath(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.user.profile.email": {Match: contract.MatchRegex, Pattern: `@`},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	data := map[string]any{
		"user": map[string]any{
			"profile": map[string]any{
				"email": "user@example.com",
			},
		},
	}

	result := engine.Match(data)
	if !result.Success {
		t.Errorf("expected success, got errors: %v", result.Errors)
	}
}

func TestEngine_Match_ArrayWildcard(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.items[*].id": {Match: contract.MatchTypeValue},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name    string
		data    any
		success bool
	}{
		{
			name: "all items have id",
			data: map[string]any{
				"items": []any{
					map[string]any{"id": "1"},
					map[string]any{"id": "2"},
					map[string]any{"id": "3"},
				},
			},
			success: true,
		},
		{
			name: "one item missing id",
			data: map[string]any{
				"items": []any{
					map[string]any{"id": "1"},
					map[string]any{"name": "no id"},
					map[string]any{"id": "3"},
				},
			},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestEngine_LoadRules_InvalidRegex(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.test": {Match: contract.MatchRegex, Pattern: "[invalid"},
	})
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestEngine_LoadRules_MissingPattern(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.test": {Match: contract.MatchRegex}, // Missing pattern
	})
	if err == nil {
		t.Error("expected error for missing regex pattern")
	}
}

func TestMatchStructure_Objects(t *testing.T) {
	expected := map[string]any{
		"name":  "test",
		"count": 42,
	}

	tests := []struct {
		name    string
		actual  any
		success bool
	}{
		{
			name: "matching structure",
			actual: map[string]any{
				"name":  "different",
				"count": 100,
			},
			success: true,
		},
		{
			name: "missing field",
			actual: map[string]any{
				"name": "test",
			},
			success: false,
		},
		{
			name:    "not an object",
			actual:  "string",
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestMatchStructure_Arrays(t *testing.T) {
	expected := []any{
		map[string]any{"id": "1", "name": "item"},
	}

	tests := []struct {
		name    string
		actual  any
		success bool
	}{
		{
			name: "matching array elements",
			actual: []any{
				map[string]any{"id": "a", "name": "first"},
				map[string]any{"id": "b", "name": "second"},
			},
			success: true,
		},
		{
			name: "missing field in element",
			actual: []any{
				map[string]any{"id": "a"},
			},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestMatchStructure_NullExpected(t *testing.T) {
	// null expected means any value is acceptable
	result := MatchStructure(nil, "any value")
	if !result.Success {
		t.Errorf("expected success for null expected, got errors: %v", result.Errors)
	}
}

func TestMatchStructure_NullActual(t *testing.T) {
	result := MatchStructure("expected", nil)
	if result.Success {
		t.Error("expected failure for null actual when expected is non-null")
	}
}

func TestMatchWithDefaults(t *testing.T) {
	contractRules := contract.MatchingRules{
		"$.response.id": {Match: contract.MatchTypeValue},
	}
	interactionRules := contract.MatchingRules{
		"$.response.status": {Match: contract.MatchExact, Value: "ok"},
	}

	data := map[string]any{
		"response": map[string]any{
			"id":     "123",
			"status": "ok",
		},
	}

	result := MatchWithDefaults(data, interactionRules, contractRules)
	if !result.Success {
		t.Errorf("expected success, got errors: %v", result.Errors)
	}
}

func TestEngine_MultipleRules(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadRules(contract.MatchingRules{
		"$.name":   {Match: contract.MatchTypeValue},
		"$.email":  {Match: contract.MatchRegex, Pattern: `@`},
		"$.age":    {Match: contract.MatchInteger},
		"$.active": {Match: contract.MatchExact, Value: true},
	})
	if err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}

	tests := []struct {
		name       string
		data       any
		success    bool
		errorCount int
	}{
		{
			name: "all rules pass",
			data: map[string]any{
				"name":   "John",
				"email":  "john@example.com",
				"age":    30,
				"active": true,
			},
			success:    true,
			errorCount: 0,
		},
		{
			name: "one rule fails",
			data: map[string]any{
				"name":   "John",
				"email":  "invalid",
				"age":    30,
				"active": true,
			},
			success:    false,
			errorCount: 1,
		},
		{
			name: "multiple rules fail",
			data: map[string]any{
				"name":   nil,
				"email":  "invalid",
				"age":    30.5,
				"active": false,
			},
			success:    false,
			errorCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v", tt.success, result.Success)
			}
			if len(result.Errors) != tt.errorCount {
				t.Errorf("expected %d errors, got %d: %v",
					tt.errorCount, len(result.Errors), result.Errors)
			}
		})
	}
}

// --- New tests for uncovered code paths ---

func newEngine(t *testing.T, rules contract.MatchingRules) *Engine {
	t.Helper()
	engine := NewEngine()
	if err := engine.LoadRules(rules); err != nil {
		t.Fatalf("LoadRules failed: %v", err)
	}
	return engine
}

func TestNormalizeData_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name:    "nil returns nil",
			data:    nil,
			wantErr: false,
		},
		{
			name:    "string passthrough",
			data:    "hello",
			wantErr: false,
		},
		{
			name:    "float64 passthrough",
			data:    float64(3.14),
			wantErr: false,
		},
		{
			name:    "bool passthrough",
			data:    true,
			wantErr: false,
		},
		{
			name:    "map passthrough",
			data:    map[string]any{"key": "val"},
			wantErr: false,
		},
		{
			name:    "slice passthrough",
			data:    []any{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "channel is not JSON-serializable",
			data:    make(chan int),
			wantErr: true,
		},
		{
			name:    "func is not JSON-serializable",
			data:    func() {},
			wantErr: true,
		},
		{
			name:    "struct normalizes via marshal/unmarshal",
			data:    struct{ Name string }{"test"},
			wantErr: false,
		},
		{
			name:    "int normalizes via marshal/unmarshal",
			data:    42,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeData(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("normalizeData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeData_EngineMatchWithChannel(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "test"},
	})

	result := engine.Match(make(chan int))
	if result.Success {
		t.Error("expected failure when normalizing channel data")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if result.Errors[0].Path != "$" {
		t.Errorf("expected error path '$', got %q", result.Errors[0].Path)
	}
}

func TestMatchValue_NoRuleForPath(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "test"},
	})

	// MatchValue for a path with no rule should return nil (no failure)
	failure := engine.MatchValue("$.nonexistent", map[string]any{"name": "test"})
	if failure != nil {
		t.Errorf("expected nil for path with no rule, got %+v", failure)
	}
}

func TestMatchValue_PathResolutionFailure(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.nested.deep.field": {Match: contract.MatchTypeValue},
	})

	// Data does not contain the nested path
	failure := engine.MatchValue("$.nested.deep.field", map[string]any{"other": "value"})
	if failure == nil {
		t.Fatal("expected failure for unresolvable path")
	}
	if failure.Path != "$.nested.deep.field" {
		t.Errorf("expected path '$.nested.deep.field', got %q", failure.Path)
	}
}

func TestMatchValue_PathResolutionFailureOptional(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.missing.path": {Match: contract.MatchOptional},
	})

	// Optional rule with unresolvable path should return nil
	failure := engine.MatchValue("$.missing.path", map[string]any{"other": "value"})
	if failure != nil {
		t.Errorf("expected nil for optional rule with unresolvable path, got %+v", failure)
	}
}

func TestMatchValue_NormalizeError(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.field": {Match: contract.MatchTypeValue},
	})

	// Pass non-serializable data to MatchValue
	failure := engine.MatchValue("$.field", make(chan int))
	if failure == nil {
		t.Fatal("expected failure for non-serializable data")
	}
	if failure.Path != "$.field" {
		t.Errorf("expected path '$.field', got %q", failure.Path)
	}
}

func TestMatchValue_SuccessfulMatch(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "hello"},
	})

	failure := engine.MatchValue("$.name", map[string]any{"name": "hello"})
	if failure != nil {
		t.Errorf("expected nil for successful match, got %+v", failure)
	}
}

func TestMatchValue_RuleFailure(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "hello"},
	})

	failure := engine.MatchValue("$.name", map[string]any{"name": "world"})
	if failure == nil {
		t.Fatal("expected failure for mismatched value")
	}
	if failure.Path != "$.name" {
		t.Errorf("expected path '$.name', got %q", failure.Path)
	}
	if failure.Expected == "" {
		t.Error("expected non-empty Expected field")
	}
}

func TestMatchStructure_ObjectVsNonObject(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		success  bool
	}{
		{
			name:     "object expected, string actual",
			expected: map[string]any{"key": "val"},
			actual:   "a string",
			success:  false,
		},
		{
			name:     "object expected, number actual",
			expected: map[string]any{"key": "val"},
			actual:   float64(42),
			success:  false,
		},
		{
			name:     "object expected, bool actual",
			expected: map[string]any{"key": "val"},
			actual:   true,
			success:  false,
		},
		{
			name:     "object expected, array actual",
			expected: map[string]any{"key": "val"},
			actual:   []any{1, 2, 3},
			success:  false,
		},
		{
			name:     "object expected, nil actual",
			expected: map[string]any{"key": "val"},
			actual:   nil,
			success:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(tt.expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestMatchStructure_ArrayVsNonArray(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		success  bool
	}{
		{
			name:     "array expected, string actual",
			expected: []any{"item"},
			actual:   "a string",
			success:  false,
		},
		{
			name:     "array expected, number actual",
			expected: []any{"item"},
			actual:   float64(42),
			success:  false,
		},
		{
			name:     "array expected, object actual",
			expected: []any{"item"},
			actual:   map[string]any{"key": "val"},
			success:  false,
		},
		{
			name:     "array expected, bool actual",
			expected: []any{"item"},
			actual:   true,
			success:  false,
		},
		{
			name:     "array expected, nil actual",
			expected: []any{"item"},
			actual:   nil,
			success:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(tt.expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestMatchStructure_PrimitiveTypeMismatches(t *testing.T) {
	tests := []struct {
		name     string
		expected any
		actual   any
		success  bool
	}{
		{
			name:     "string vs string (same type)",
			expected: "hello",
			actual:   "world",
			success:  true,
		},
		{
			name:     "string vs number",
			expected: "hello",
			actual:   float64(42),
			success:  false,
		},
		{
			name:     "string vs bool",
			expected: "hello",
			actual:   true,
			success:  false,
		},
		{
			name:     "number vs string",
			expected: float64(42),
			actual:   "hello",
			success:  false,
		},
		{
			name:     "number vs number (same type)",
			expected: float64(1),
			actual:   float64(999),
			success:  true,
		},
		{
			name:     "number vs bool",
			expected: float64(1),
			actual:   true,
			success:  false,
		},
		{
			name:     "bool vs string",
			expected: true,
			actual:   "true",
			success:  false,
		},
		{
			name:     "bool vs number",
			expected: true,
			actual:   float64(1),
			success:  false,
		},
		{
			name:     "bool vs bool (same type)",
			expected: true,
			actual:   false,
			success:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(tt.expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestMatchStructure_EmptyArrays(t *testing.T) {
	// Both empty: no elements to compare, should succeed
	result := MatchStructure([]any{}, []any{})
	if !result.Success {
		t.Errorf("expected success for empty arrays, got errors: %v", result.Errors)
	}

	// Expected empty, actual has elements: still succeeds (no template to check)
	result = MatchStructure([]any{}, []any{"a", "b"})
	if !result.Success {
		t.Errorf("expected success for empty expected array, got errors: %v", result.Errors)
	}

	// Expected has template, actual is empty: succeeds (no actual elements to check)
	result = MatchStructure([]any{"template"}, []any{})
	if !result.Success {
		t.Errorf("expected success for empty actual array with template, got errors: %v", result.Errors)
	}
}

func TestMatchStructure_NestedObjectMismatch(t *testing.T) {
	expected := map[string]any{
		"user": map[string]any{
			"name": "test",
			"age":  float64(30),
		},
	}

	tests := []struct {
		name    string
		actual  any
		success bool
	}{
		{
			name: "nested types match",
			actual: map[string]any{
				"user": map[string]any{
					"name": "other",
					"age":  float64(25),
				},
			},
			success: true,
		},
		{
			name: "nested type mismatch",
			actual: map[string]any{
				"user": map[string]any{
					"name": "other",
					"age":  "not a number",
				},
			},
			success: false,
		},
		{
			name: "nested value is not an object",
			actual: map[string]any{
				"user": "not an object",
			},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestTypesCompatible(t *testing.T) {
	tests := []struct {
		name       string
		expected   any
		actual     any
		compatible bool
	}{
		{
			name:       "nil expected",
			expected:   nil,
			actual:     "anything",
			compatible: true,
		},
		{
			name:       "nil actual",
			expected:   "anything",
			actual:     nil,
			compatible: true,
		},
		{
			name:       "both nil",
			expected:   nil,
			actual:     nil,
			compatible: true,
		},
		{
			name:       "same type string",
			expected:   "a",
			actual:     "b",
			compatible: true,
		},
		{
			name:       "same type float64",
			expected:   float64(1),
			actual:     float64(2),
			compatible: true,
		},
		{
			name:       "same type bool",
			expected:   true,
			actual:     false,
			compatible: true,
		},
		{
			name:       "float64 vs int (numeric compat)",
			expected:   float64(1),
			actual:     42,
			compatible: true,
		},
		{
			name:       "int vs float64 (numeric compat)",
			expected:   42,
			actual:     float64(1),
			compatible: true,
		},
		{
			name:       "int vs int64 (numeric compat)",
			expected:   42,
			actual:     int64(100),
			compatible: true,
		},
		{
			name:       "uint vs float32 (numeric compat)",
			expected:   uint(5),
			actual:     float32(3.14),
			compatible: true,
		},
		{
			name:       "string vs int (incompatible)",
			expected:   "hello",
			actual:     42,
			compatible: false,
		},
		{
			name:       "string vs bool (incompatible)",
			expected:   "hello",
			actual:     true,
			compatible: false,
		},
		{
			name:       "bool vs float64 (incompatible)",
			expected:   true,
			actual:     float64(1),
			compatible: false,
		},
		{
			name:       "string vs slice (incompatible)",
			expected:   "hello",
			actual:     []any{1},
			compatible: false,
		},
		{
			name:       "string vs map (incompatible)",
			expected:   "hello",
			actual:     map[string]any{},
			compatible: false,
		},
		{
			name:       "map vs slice (incompatible)",
			expected:   map[string]any{},
			actual:     []any{},
			compatible: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typesCompatible(tt.expected, tt.actual)
			if got != tt.compatible {
				t.Errorf("typesCompatible(%T, %T) = %v, want %v",
					tt.expected, tt.actual, got, tt.compatible)
			}
		})
	}
}

func TestTypeName(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected string
	}{
		{name: "nil", value: nil, expected: "null"},
		{name: "string", value: "hello", expected: "string"},
		{name: "float64", value: float64(3.14), expected: "number"},
		{name: "float32", value: float32(3.14), expected: "number"},
		{name: "int", value: 42, expected: "number"},
		{name: "int64", value: int64(42), expected: "number"},
		{name: "int32", value: int32(42), expected: "number"},
		{name: "bool", value: true, expected: "boolean"},
		{name: "array", value: []any{1, 2}, expected: "array"},
		{name: "object", value: map[string]any{"k": "v"}, expected: "object"},
		{name: "unknown type (chan)", value: make(chan int), expected: "chan int"},
		{name: "unknown type (uint)", value: uint(5), expected: "uint"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := typeName(tt.value)
			if got != tt.expected {
				t.Errorf("typeName(%v) = %q, want %q", tt.value, got, tt.expected)
			}
		})
	}
}

func TestEachLikeRule_ThroughEngine(t *testing.T) {
	minVal := 2
	maxVal := 5

	tests := []struct {
		name    string
		rules   contract.MatchingRules
		data    any
		success bool
	}{
		{
			name: "array within bounds",
			rules: contract.MatchingRules{
				"$.tags": {Match: contract.MatchEachLike, Min: &minVal, Max: &maxVal},
			},
			data: map[string]any{
				"tags": []any{"go", "test", "coverage"},
			},
			success: true,
		},
		{
			name: "array below minimum",
			rules: contract.MatchingRules{
				"$.tags": {Match: contract.MatchEachLike, Min: &minVal, Max: &maxVal},
			},
			data: map[string]any{
				"tags": []any{"only-one"},
			},
			success: false,
		},
		{
			name: "array above maximum",
			rules: contract.MatchingRules{
				"$.tags": {Match: contract.MatchEachLike, Min: &minVal, Max: &maxVal},
			},
			data: map[string]any{
				"tags": []any{"a", "b", "c", "d", "e", "f"},
			},
			success: false,
		},
		{
			name: "non-array value",
			rules: contract.MatchingRules{
				"$.tags": {Match: contract.MatchEachLike, Min: &minVal},
			},
			data: map[string]any{
				"tags": "not-an-array",
			},
			success: false,
		},
		{
			name: "no bounds",
			rules: contract.MatchingRules{
				"$.items": {Match: contract.MatchEachLike},
			},
			data: map[string]any{
				"items": []any{},
			},
			success: true,
		},
		{
			name: "nested each_like",
			rules: contract.MatchingRules{
				"$.data.items": {Match: contract.MatchEachLike, Min: &minVal},
			},
			data: map[string]any{
				"data": map[string]any{
					"items": []any{"a", "b", "c"},
				},
			},
			success: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newEngine(t, tt.rules)
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestWildcard_Paths(t *testing.T) {
	tests := []struct {
		name    string
		rules   contract.MatchingRules
		data    any
		success bool
	}{
		{
			name: "wildcard match all elements pass",
			rules: contract.MatchingRules{
				"$.items[*].name": {Match: contract.MatchTypeValue},
			},
			data: map[string]any{
				"items": []any{
					map[string]any{"name": "a"},
					map[string]any{"name": "b"},
				},
			},
			success: true,
		},
		{
			name: "wildcard match one element fails",
			rules: contract.MatchingRules{
				"$.items[*].name": {Match: contract.MatchTypeValue},
			},
			data: map[string]any{
				"items": []any{
					map[string]any{"name": "a"},
					map[string]any{"name": nil},
				},
			},
			success: false,
		},
		{
			name: "wildcard with regex rule",
			rules: contract.MatchingRules{
				"$.emails[*]": {Match: contract.MatchRegex, Pattern: `@`},
			},
			data: map[string]any{
				"emails": []any{"a@b.com", "c@d.com"},
			},
			success: true,
		},
		{
			name: "wildcard with regex rule failure",
			rules: contract.MatchingRules{
				"$.emails[*]": {Match: contract.MatchRegex, Pattern: `@`},
			},
			data: map[string]any{
				"emails": []any{"a@b.com", "invalid"},
			},
			success: false,
		},
		{
			name: "wildcard path resolution failure - missing array",
			rules: contract.MatchingRules{
				"$.items[*].name": {Match: contract.MatchTypeValue},
			},
			data: map[string]any{
				"other": "value",
			},
			success: false,
		},
		{
			name: "wildcard path resolution failure with optional rule",
			rules: contract.MatchingRules{
				"$.items[*].name": {Match: contract.MatchOptional},
			},
			data: map[string]any{
				"other": "value",
			},
			success: true, // Optional rule skips missing wildcard paths
		},
		{
			name: "wildcard with exact match",
			rules: contract.MatchingRules{
				"$.statuses[*]": {Match: contract.MatchExact, Value: "active"},
			},
			data: map[string]any{
				"statuses": []any{"active", "active", "active"},
			},
			success: true,
		},
		{
			name: "wildcard with exact match one different",
			rules: contract.MatchingRules{
				"$.statuses[*]": {Match: contract.MatchExact, Value: "active"},
			},
			data: map[string]any{
				"statuses": []any{"active", "inactive", "active"},
			},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := newEngine(t, tt.rules)
			result := engine.Match(tt.data)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}

func TestWildcard_NonWildcardPathResolutionFailure(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.deeply.nested.path": {Match: contract.MatchTypeValue},
	})

	result := engine.Match(map[string]any{"other": "val"})
	if result.Success {
		t.Error("expected failure for non-wildcard path resolution failure")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
}

func TestWildcard_NonWildcardOptionalPathMissing(t *testing.T) {
	engine := newEngine(t, contract.MatchingRules{
		"$.deeply.nested.path": {Match: contract.MatchOptional},
	})

	result := engine.Match(map[string]any{"other": "val"})
	if !result.Success {
		t.Errorf("expected success for optional rule with missing path, got errors: %v", result.Errors)
	}
}

func TestContainsWildcard(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{path: "$.items[*].name", expected: true},
		{path: "$.items[*]", expected: true},
		{path: "$.*", expected: true},
		{path: "$.name", expected: false},
		{path: "$.items[0].name", expected: false},
		{path: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := containsWildcard(tt.path)
			if got != tt.expected {
				t.Errorf("containsWildcard(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestMatchWithDefaults_InvalidContractRules(t *testing.T) {
	contractRules := contract.MatchingRules{
		"$.field": {Match: contract.MatchRegex}, // Missing pattern
	}
	interactionRules := contract.MatchingRules{}

	result := MatchWithDefaults(map[string]any{}, interactionRules, contractRules)
	if result.Success {
		t.Error("expected failure for invalid contract rules")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if result.Errors[0].Path != "$" {
		t.Errorf("expected error path '$', got %q", result.Errors[0].Path)
	}
}

func TestMatchWithDefaults_InvalidInteractionRules(t *testing.T) {
	contractRules := contract.MatchingRules{}
	interactionRules := contract.MatchingRules{
		"$.field": {Match: contract.MatchRegex}, // Missing pattern
	}

	result := MatchWithDefaults(map[string]any{}, interactionRules, contractRules)
	if result.Success {
		t.Error("expected failure for invalid interaction rules")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if result.Errors[0].Path != "$" {
		t.Errorf("expected error path '$', got %q", result.Errors[0].Path)
	}
}

func TestMatchWithDefaults_InteractionOverridesContract(t *testing.T) {
	contractRules := contract.MatchingRules{
		"$.name": {Match: contract.MatchTypeValue}, // Just type check
	}
	interactionRules := contract.MatchingRules{
		"$.name": {Match: contract.MatchExact, Value: "specific"}, // Override with exact
	}

	// Match succeeds when interaction rule is satisfied
	result := MatchWithDefaults(
		map[string]any{"name": "specific"},
		interactionRules,
		contractRules,
	)
	if !result.Success {
		t.Errorf("expected success, got errors: %v", result.Errors)
	}

	// Match fails when interaction rule is not satisfied (even though type rule would pass)
	result = MatchWithDefaults(
		map[string]any{"name": "other"},
		interactionRules,
		contractRules,
	)
	if result.Success {
		t.Error("expected failure when interaction override is not satisfied")
	}
}

func TestEngine_AddRule(t *testing.T) {
	engine := NewEngine()
	engine.AddRule("$.name", &exactRule{expected: "test"})

	result := engine.Match(map[string]any{"name": "test"})
	if !result.Success {
		t.Errorf("expected success, got errors: %v", result.Errors)
	}

	result = engine.Match(map[string]any{"name": "other"})
	if result.Success {
		t.Error("expected failure for mismatched value")
	}
}

func TestMatchStructure_ArrayWithNestedObjects(t *testing.T) {
	expected := []any{
		map[string]any{
			"id":   float64(1),
			"data": map[string]any{"value": "x"},
		},
	}

	tests := []struct {
		name    string
		actual  any
		success bool
	}{
		{
			name: "nested structure matches",
			actual: []any{
				map[string]any{
					"id":   float64(99),
					"data": map[string]any{"value": "y"},
				},
			},
			success: true,
		},
		{
			name: "nested structure type mismatch",
			actual: []any{
				map[string]any{
					"id":   "not-a-number",
					"data": map[string]any{"value": "y"},
				},
			},
			success: false,
		},
		{
			name: "nested object replaced with primitive",
			actual: []any{
				map[string]any{
					"id":   float64(1),
					"data": "not-an-object",
				},
			},
			success: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MatchStructure(expected, tt.actual)
			if result.Success != tt.success {
				t.Errorf("expected success=%v, got success=%v, errors=%v",
					tt.success, result.Success, result.Errors)
			}
		})
	}
}
