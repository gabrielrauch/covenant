package matching

import (
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

func TestCompile_AllRuleTypes(t *testing.T) {
	minVal := 1
	maxVal := 10

	tests := []struct {
		name    string
		def     contract.MatchingRule
		wantErr bool
	}{
		{
			name:    "exact rule",
			def:     contract.MatchingRule{Match: contract.MatchExact, Value: "test"},
			wantErr: false,
		},
		{
			name:    "type rule",
			def:     contract.MatchingRule{Match: contract.MatchTypeValue},
			wantErr: false,
		},
		{
			name:    "regex rule valid",
			def:     contract.MatchingRule{Match: contract.MatchRegex, Pattern: `^[a-z]+$`},
			wantErr: false,
		},
		{
			name:    "regex rule invalid pattern",
			def:     contract.MatchingRule{Match: contract.MatchRegex, Pattern: `[invalid`},
			wantErr: true,
		},
		{
			name:    "regex rule missing pattern",
			def:     contract.MatchingRule{Match: contract.MatchRegex},
			wantErr: true,
		},
		{
			name:    "integer rule",
			def:     contract.MatchingRule{Match: contract.MatchInteger},
			wantErr: false,
		},
		{
			name:    "decimal rule",
			def:     contract.MatchingRule{Match: contract.MatchDecimal},
			wantErr: false,
		},
		{
			name:    "include rule valid",
			def:     contract.MatchingRule{Match: contract.MatchInclude, Value: "substring"},
			wantErr: false,
		},
		{
			name:    "include rule missing value",
			def:     contract.MatchingRule{Match: contract.MatchInclude},
			wantErr: true,
		},
		{
			name:    "include rule non-string value",
			def:     contract.MatchingRule{Match: contract.MatchInclude, Value: 123},
			wantErr: true,
		},
		{
			name:    "each_like rule",
			def:     contract.MatchingRule{Match: contract.MatchEachLike, Min: &minVal, Max: &maxVal},
			wantErr: false,
		},
		{
			name:    "optional rule",
			def:     contract.MatchingRule{Match: contract.MatchOptional},
			wantErr: false,
		},
		{
			name: "null_or rule valid",
			def: contract.MatchingRule{
				Match: contract.MatchNullOr,
				Rule:  &contract.MatchingRule{Match: contract.MatchTypeValue},
			},
			wantErr: false,
		},
		{
			name:    "null_or rule missing inner",
			def:     contract.MatchingRule{Match: contract.MatchNullOr},
			wantErr: true,
		},
		{
			name: "null_or rule invalid inner",
			def: contract.MatchingRule{
				Match: contract.MatchNullOr,
				Rule:  &contract.MatchingRule{Match: contract.MatchRegex}, // Missing pattern
			},
			wantErr: true,
		},
		{
			name:    "unknown match type",
			def:     contract.MatchingRule{Match: "unknown_type"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile(tt.def)
			if (err != nil) != tt.wantErr {
				t.Errorf("Compile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExactRule_Match(t *testing.T) {
	rule := &exactRule{expected: "test"}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "exact match", value: "test", wantErr: false},
		{name: "mismatch", value: "other", wantErr: true},
		{name: "nil value", value: nil, wantErr: true},
		{name: "number vs string", value: 123, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestExactRule_NumericComparison(t *testing.T) {
	rule := &exactRule{expected: float64(42)}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "float64 match", value: float64(42), wantErr: false},
		{name: "int match", value: 42, wantErr: false},
		{name: "int64 match", value: int64(42), wantErr: false},
		{name: "different value", value: float64(43), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypeRule_Match(t *testing.T) {
	rule := &typeRule{}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "string", value: "hello", wantErr: false},
		{name: "number", value: 42, wantErr: false},
		{name: "boolean", value: true, wantErr: false},
		{name: "object", value: map[string]any{}, wantErr: false},
		{name: "array", value: []any{}, wantErr: false},
		{name: "nil", value: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRegexRule_Match(t *testing.T) {
	rule, err := compileRegexRule(contract.MatchingRule{
		Match:   contract.MatchRegex,
		Pattern: `^[a-z]+@[a-z]+\.[a-z]+$`,
	})
	if err != nil {
		t.Fatalf("failed to compile rule: %v", err)
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "valid email", value: "test@example.com", wantErr: false},
		{name: "invalid email", value: "not-an-email", wantErr: true},
		{name: "non-string", value: 12345, wantErr: true},
		{name: "empty string", value: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIntegerRule_Match(t *testing.T) {
	rule := &integerRule{}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "int", value: 42, wantErr: false},
		{name: "int64", value: int64(42), wantErr: false},
		{name: "uint", value: uint(42), wantErr: false},
		{name: "float64 whole", value: float64(42), wantErr: false},
		{name: "float64 decimal", value: float64(42.5), wantErr: true},
		{name: "float32 whole", value: float32(42), wantErr: false},
		{name: "float32 decimal", value: float32(42.5), wantErr: true},
		{name: "string", value: "42", wantErr: true},
		{name: "nil", value: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecimalRule_Match(t *testing.T) {
	rule := &decimalRule{}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "float64", value: float64(42.5), wantErr: false},
		{name: "float32", value: float32(42.5), wantErr: false},
		{name: "int", value: 42, wantErr: false},
		{name: "int64", value: int64(42), wantErr: false},
		{name: "string", value: "42.5", wantErr: true},
		{name: "bool", value: true, wantErr: true},
		{name: "nil", value: nil, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIncludeRule_Match(t *testing.T) {
	rule := &includeRule{substring: "error"}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "contains substring", value: "An error occurred", wantErr: false},
		{name: "exact substring", value: "error", wantErr: false},
		{name: "missing substring", value: "All good", wantErr: true},
		{name: "non-string", value: 12345, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIncludeRule_EmptySubstring(t *testing.T) {
	rule := &includeRule{substring: ""}

	// Empty substring should match any string
	err := rule.Match("anything")
	if err != nil {
		t.Errorf("expected empty substring to match, got error: %v", err)
	}
}

func TestEachLikeRule_Match(t *testing.T) {
	minVal := 2
	maxVal := 4

	tests := []struct {
		name    string
		min     *int
		max     *int
		value   any
		wantErr bool
	}{
		{
			name:    "within bounds",
			min:     &minVal,
			max:     &maxVal,
			value:   []any{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "at minimum",
			min:     &minVal,
			max:     &maxVal,
			value:   []any{"a", "b"},
			wantErr: false,
		},
		{
			name:    "at maximum",
			min:     &minVal,
			max:     &maxVal,
			value:   []any{"a", "b", "c", "d"},
			wantErr: false,
		},
		{
			name:    "below minimum",
			min:     &minVal,
			max:     &maxVal,
			value:   []any{"a"},
			wantErr: true,
		},
		{
			name:    "above maximum",
			min:     &minVal,
			max:     &maxVal,
			value:   []any{"a", "b", "c", "d", "e"},
			wantErr: true,
		},
		{
			name:    "no bounds",
			min:     nil,
			max:     nil,
			value:   []any{},
			wantErr: false,
		},
		{
			name:    "min only",
			min:     &minVal,
			max:     nil,
			value:   []any{"a", "b", "c", "d", "e", "f"},
			wantErr: false,
		},
		{
			name:    "non-array",
			min:     nil,
			max:     nil,
			value:   "not-an-array",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &eachLikeRule{min: tt.min, max: tt.max}
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOptionalRule_Match(t *testing.T) {
	rule := &optionalRule{}

	// Optional rule should always match
	tests := []any{"string", 42, true, nil, []any{}, map[string]any{}}

	for _, value := range tests {
		err := rule.Match(value)
		if err != nil {
			t.Errorf("optional rule should match %v, got error: %v", value, err)
		}
	}
}

func TestNullOrRule_Match(t *testing.T) {
	inner := &typeRule{}
	rule := &nullOrRule{inner: inner}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "null", value: nil, wantErr: false},
		{name: "valid inner", value: "non-null", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := rule.Match(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Match() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNullOrRule_DelegateToInner(t *testing.T) {
	inner := &integerRule{}
	rule := &nullOrRule{inner: inner}

	// null is always valid
	if err := rule.Match(nil); err != nil {
		t.Errorf("null should match, got: %v", err)
	}

	// integer should match (delegated)
	if err := rule.Match(42); err != nil {
		t.Errorf("integer should match, got: %v", err)
	}

	// string should fail (delegated)
	if err := rule.Match("not-integer"); err == nil {
		t.Error("string should not match integer rule")
	}
}

func TestRuleString(t *testing.T) {
	minVal := 1
	maxVal := 10

	tests := []struct {
		rule     Rule
		expected string
	}{
		{&exactRule{expected: "test"}, "exact(test)"},
		{&typeRule{}, "type()"},
		{&integerRule{}, "integer()"},
		{&decimalRule{}, "decimal()"},
		{&includeRule{substring: "foo"}, `include("foo")`},
		{&optionalRule{}, "optional()"},
		{&eachLikeRule{min: &minVal, max: &maxVal}, "each_like(1..10)"},
		{&eachLikeRule{min: nil, max: nil}, "each_like(*..*)"},
		{&nullOrRule{inner: &typeRule{}}, "null_or(type())"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.rule.String()
			if got != tt.expected {
				t.Errorf("String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDeepEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     any
		expected bool
	}{
		{name: "both nil", a: nil, b: nil, expected: true},
		{name: "one nil", a: nil, b: "x", expected: false},
		{name: "other nil", a: "x", b: nil, expected: false},
		{name: "same string", a: "test", b: "test", expected: true},
		{name: "different string", a: "a", b: "b", expected: false},
		{name: "int vs float64", a: 42, b: float64(42), expected: true},
		{name: "int64 vs float64", a: int64(42), b: float64(42), expected: true},
		{name: "different numbers", a: 42, b: float64(43), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deepEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("deepEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s, substr string
		expected  bool
	}{
		{s: "hello world", substr: "world", expected: true},
		{s: "hello world", substr: "hello", expected: true},
		{s: "hello world", substr: "foo", expected: false},
		{s: "hello", substr: "", expected: true}, // empty substring
		{s: "", substr: "x", expected: false},    // empty string
		{s: "", substr: "", expected: true},      // both empty
	}

	for _, tt := range tests {
		got := contains(tt.s, tt.substr)
		if got != tt.expected {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.expected)
		}
	}
}

func TestMatchError(t *testing.T) {
	err := &MatchError{
		Expected: "integer",
		Actual:   "string",
		Message:  "expected integer, got string",
	}

	if err.Error() != "expected integer, got string" {
		t.Errorf("Error() = %q, want %q", err.Error(), "expected integer, got string")
	}
}
