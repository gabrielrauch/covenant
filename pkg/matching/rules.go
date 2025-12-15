// Package matching provides the matching rules engine for contract verification.
package matching

import (
	"fmt"
	"regexp"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// Rule represents a compiled matching rule ready for evaluation.
type Rule interface {
	// Match evaluates the rule against a value.
	Match(value any) *MatchError
	// String returns a description of the rule.
	String() string
}

// MatchError represents a matching failure.
type MatchError struct {
	Expected string
	Actual   string
	Message  string
}

func (e *MatchError) Error() string {
	return e.Message
}

// Compile converts a MatchingRule definition into a compiled Rule.
func Compile(def contract.MatchingRule) (Rule, error) {
	switch def.Match {
	case contract.MatchExact:
		return &exactRule{expected: def.Value}, nil
	case contract.MatchType_:
		return &typeRule{}, nil
	case contract.MatchRegex:
		if def.Pattern == "" {
			return nil, fmt.Errorf("regex rule requires pattern")
		}
		re, err := regexp.Compile(def.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
		return &regexRule{pattern: re, patternStr: def.Pattern}, nil
	case contract.MatchInteger:
		return &integerRule{}, nil
	case contract.MatchDecimal:
		return &decimalRule{}, nil
	case contract.MatchInclude:
		if def.Value == nil {
			return nil, fmt.Errorf("include rule requires value")
		}
		substr, ok := def.Value.(string)
		if !ok {
			return nil, fmt.Errorf("include rule value must be a string")
		}
		return &includeRule{substring: substr}, nil
	case contract.MatchEachLike:
		return &eachLikeRule{min: def.Min, max: def.Max}, nil
	case contract.MatchOptional:
		return &optionalRule{}, nil
	case contract.MatchNullOr:
		if def.Rule == nil {
			return nil, fmt.Errorf("null_or rule requires inner rule")
		}
		inner, err := Compile(*def.Rule)
		if err != nil {
			return nil, fmt.Errorf("invalid inner rule for null_or: %w", err)
		}
		return &nullOrRule{inner: inner}, nil
	default:
		return nil, fmt.Errorf("unknown match type: %s", def.Match)
	}
}

// exactRule matches values that are exactly equal.
type exactRule struct {
	expected any
}

func (r *exactRule) Match(value any) *MatchError {
	if !deepEqual(r.expected, value) {
		return &MatchError{
			Expected: fmt.Sprintf("%v", r.expected),
			Actual:   fmt.Sprintf("%v", value),
			Message:  fmt.Sprintf("expected exact value %v, got %v", r.expected, value),
		}
	}
	return nil
}

func (r *exactRule) String() string {
	return fmt.Sprintf("exact(%v)", r.expected)
}

// typeRule matches values that have the same JSON type as the expected value.
type typeRule struct{}

func (r *typeRule) Match(value any) *MatchError {
	// Type rule accepts any non-nil value
	if value == nil {
		return &MatchError{
			Expected: "non-null value",
			Actual:   "null",
			Message:  "expected non-null value, got null",
		}
	}
	return nil
}

func (r *typeRule) String() string {
	return "type()"
}

// regexRule matches string values against a regular expression.
type regexRule struct {
	pattern    *regexp.Regexp
	patternStr string
}

func (r *regexRule) Match(value any) *MatchError {
	str, ok := value.(string)
	if !ok {
		return &MatchError{
			Expected: "string",
			Actual:   fmt.Sprintf("%T", value),
			Message:  fmt.Sprintf("regex rule requires string value, got %T", value),
		}
	}
	if !r.pattern.MatchString(str) {
		return &MatchError{
			Expected: fmt.Sprintf("match pattern %s", r.patternStr),
			Actual:   str,
			Message:  fmt.Sprintf("value %q does not match pattern %s", str, r.patternStr),
		}
	}
	return nil
}

func (r *regexRule) String() string {
	return fmt.Sprintf("regex(%s)", r.patternStr)
}

// integerRule matches integer values.
type integerRule struct{}

func (r *integerRule) Match(value any) *MatchError {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	case float64:
		// JSON numbers are float64, check if it's a whole number
		if v == float64(int64(v)) {
			return nil
		}
		return &MatchError{
			Expected: "integer",
			Actual:   fmt.Sprintf("%v", v),
			Message:  fmt.Sprintf("expected integer, got decimal %v", v),
		}
	case float32:
		if v == float32(int32(v)) {
			return nil
		}
		return &MatchError{
			Expected: "integer",
			Actual:   fmt.Sprintf("%v", v),
			Message:  fmt.Sprintf("expected integer, got decimal %v", v),
		}
	default:
		return &MatchError{
			Expected: "integer",
			Actual:   fmt.Sprintf("%T", value),
			Message:  fmt.Sprintf("expected integer, got %T", value),
		}
	}
}

func (r *integerRule) String() string {
	return "integer()"
}

// decimalRule matches decimal (floating-point) numbers.
type decimalRule struct{}

func (r *decimalRule) Match(value any) *MatchError {
	switch value.(type) {
	case float32, float64, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return nil
	default:
		return &MatchError{
			Expected: "decimal",
			Actual:   fmt.Sprintf("%T", value),
			Message:  fmt.Sprintf("expected decimal number, got %T", value),
		}
	}
}

func (r *decimalRule) String() string {
	return "decimal()"
}

// includeRule matches strings that contain a substring.
type includeRule struct {
	substring string
}

func (r *includeRule) Match(value any) *MatchError {
	str, ok := value.(string)
	if !ok {
		return &MatchError{
			Expected: "string",
			Actual:   fmt.Sprintf("%T", value),
			Message:  fmt.Sprintf("include rule requires string value, got %T", value),
		}
	}
	if !contains(str, r.substring) {
		return &MatchError{
			Expected: fmt.Sprintf("string containing %q", r.substring),
			Actual:   str,
			Message:  fmt.Sprintf("value %q does not contain %q", str, r.substring),
		}
	}
	return nil
}

func (r *includeRule) String() string {
	return fmt.Sprintf("include(%q)", r.substring)
}

// eachLikeRule matches arrays where elements follow a schema pattern.
type eachLikeRule struct {
	min *int
	max *int
}

func (r *eachLikeRule) Match(value any) *MatchError {
	arr, ok := value.([]any)
	if !ok {
		return &MatchError{
			Expected: "array",
			Actual:   fmt.Sprintf("%T", value),
			Message:  fmt.Sprintf("each_like rule requires array value, got %T", value),
		}
	}

	if r.min != nil && len(arr) < *r.min {
		return &MatchError{
			Expected: fmt.Sprintf("array with at least %d elements", *r.min),
			Actual:   fmt.Sprintf("array with %d elements", len(arr)),
			Message:  fmt.Sprintf("array has %d elements, expected at least %d", len(arr), *r.min),
		}
	}

	if r.max != nil && len(arr) > *r.max {
		return &MatchError{
			Expected: fmt.Sprintf("array with at most %d elements", *r.max),
			Actual:   fmt.Sprintf("array with %d elements", len(arr)),
			Message:  fmt.Sprintf("array has %d elements, expected at most %d", len(arr), *r.max),
		}
	}

	return nil
}

func (r *eachLikeRule) String() string {
	minStr := "*"
	maxStr := "*"
	if r.min != nil {
		minStr = fmt.Sprintf("%d", *r.min)
	}
	if r.max != nil {
		maxStr = fmt.Sprintf("%d", *r.max)
	}
	return fmt.Sprintf("each_like(%s..%s)", minStr, maxStr)
}

// optionalRule always matches, indicating the field is optional.
type optionalRule struct{}

func (r *optionalRule) Match(value any) *MatchError {
	return nil // Always matches
}

func (r *optionalRule) String() string {
	return "optional()"
}

// nullOrRule matches null or delegates to an inner rule.
type nullOrRule struct {
	inner Rule
}

func (r *nullOrRule) Match(value any) *MatchError {
	if value == nil {
		return nil
	}
	return r.inner.Match(value)
}

func (r *nullOrRule) String() string {
	return fmt.Sprintf("null_or(%s)", r.inner.String())
}

// deepEqual performs a deep equality check.
func deepEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle numeric comparisons (JSON unmarshals numbers as float64)
	switch av := a.(type) {
	case float64:
		switch bv := b.(type) {
		case float64:
			return av == bv
		case int:
			return av == float64(bv)
		case int64:
			return av == float64(bv)
		}
	case int:
		switch bv := b.(type) {
		case float64:
			return float64(av) == bv
		case int:
			return av == bv
		case int64:
			return int64(av) == bv
		}
	case int64:
		switch bv := b.(type) {
		case float64:
			return float64(av) == bv
		case int:
			return av == int64(bv)
		case int64:
			return av == bv
		}
	}

	// Use fmt comparison for other types
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

// contains checks if a string contains a substring.
func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && searchSubstring(s, substr))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
