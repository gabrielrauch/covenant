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
