package matching

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// Engine evaluates matching rules against data.
type Engine struct {
	resolver *PathResolver
	rules    map[string]Rule
}

// NewEngine creates a new matching engine.
func NewEngine() *Engine {
	return &Engine{
		resolver: NewPathResolver(),
		rules:    make(map[string]Rule),
	}
}

// LoadRules loads matching rules from a contract's rule definitions.
func (e *Engine) LoadRules(rules contract.MatchingRules) error {
	for path, def := range rules {
		rule, err := Compile(def)
		if err != nil {
			return fmt.Errorf("failed to compile rule for path %s: %w", path, err)
		}
		e.rules[path] = rule
	}
	return nil
}

// AddRule adds a compiled rule for a specific path.
func (e *Engine) AddRule(path string, rule Rule) {
	e.rules[path] = rule
}

// MatchResult contains the result of matching data against rules.
type MatchResult struct {
	Success bool
	Errors  []MatchFailure
}

// MatchFailure represents a single matching failure.
type MatchFailure struct {
	Path     string
	Rule     string
	Expected string
	Actual   string
	Message  string
}

// Match evaluates all loaded rules against the provided data.
func (e *Engine) Match(data any) MatchResult {
	result := MatchResult{Success: true}

	// Normalize data to map[string]any for consistent path resolution
	normalized, err := normalizeData(data)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, MatchFailure{
			Path:    "$",
			Message: fmt.Sprintf("failed to normalize data: %v", err),
		})
		return result
	}

	for path, rule := range e.rules {
		// Check if path contains wildcard
		if containsWildcard(path) {
			values, err := e.resolver.ResolveAll(path, normalized)
			if err != nil {
				// Path doesn't exist - check if rule is optional
				if _, ok := rule.(*optionalRule); ok {
					continue
				}
				result.Success = false
				result.Errors = append(result.Errors, MatchFailure{
					Path:    path,
					Rule:    rule.String(),
					Message: fmt.Sprintf("path resolution failed: %v", err),
				})
				continue
			}

			for _, pv := range values {
				if matchErr := rule.Match(pv.Value); matchErr != nil {
					result.Success = false
					result.Errors = append(result.Errors, MatchFailure{
						Path:     pv.Path,
						Rule:     rule.String(),
						Expected: matchErr.Expected,
						Actual:   matchErr.Actual,
						Message:  matchErr.Message,
					})
				}
			}
		} else {
			value, err := e.resolver.Resolve(path, normalized)
			if err != nil {
				// Path doesn't exist - check if rule is optional
				if _, ok := rule.(*optionalRule); ok {
					continue
				}
				result.Success = false
				result.Errors = append(result.Errors, MatchFailure{
					Path:    path,
					Rule:    rule.String(),
					Message: fmt.Sprintf("path resolution failed: %v", err),
				})
				continue
			}

			if matchErr := rule.Match(value); matchErr != nil {
				result.Success = false
				result.Errors = append(result.Errors, MatchFailure{
					Path:     path,
					Rule:     rule.String(),
					Expected: matchErr.Expected,
					Actual:   matchErr.Actual,
					Message:  matchErr.Message,
				})
			}
		}
	}

	return result
}

// MatchValue matches a single value against a rule at a specific path.
func (e *Engine) MatchValue(path string, data any) *MatchFailure {
	rule, ok := e.rules[path]
	if !ok {
		return nil // No rule for this path
	}

	normalized, err := normalizeData(data)
	if err != nil {
		return &MatchFailure{
			Path:    path,
			Message: fmt.Sprintf("failed to normalize data: %v", err),
		}
	}

	value, err := e.resolver.Resolve(path, normalized)
	if err != nil {
		if _, ok := rule.(*optionalRule); ok {
			return nil
		}
		return &MatchFailure{
			Path:    path,
			Rule:    rule.String(),
			Message: fmt.Sprintf("path resolution failed: %v", err),
		}
	}

	if matchErr := rule.Match(value); matchErr != nil {
		return &MatchFailure{
			Path:     path,
			Rule:     rule.String(),
			Expected: matchErr.Expected,
			Actual:   matchErr.Actual,
			Message:  matchErr.Message,
		}
	}

	return nil
}

// MatchWithDefaults matches data using interaction rules merged with contract defaults.
func MatchWithDefaults(data any, interactionRules, contractRules contract.MatchingRules) MatchResult {
	engine := NewEngine()

	// Load contract-level defaults first
	if err := engine.LoadRules(contractRules); err != nil {
		return MatchResult{
			Success: false,
			Errors: []MatchFailure{{
				Path:    "$",
				Message: fmt.Sprintf("failed to load contract rules: %v", err),
			}},
		}
	}

	// Override with interaction-level rules
	if err := engine.LoadRules(interactionRules); err != nil {
		return MatchResult{
			Success: false,
			Errors: []MatchFailure{{
				Path:    "$",
				Message: fmt.Sprintf("failed to load interaction rules: %v", err),
			}},
		}
	}

	return engine.Match(data)
}

// MatchStructure performs structural matching without explicit rules.
// It verifies that actual data matches the expected structure/types.
func MatchStructure(expected, actual any) MatchResult {
	result := MatchResult{Success: true}
	matchStructureRecursive("$", expected, actual, &result)
	return result
}

func matchStructureRecursive(path string, expected, actual any, result *MatchResult) {
	// Handle nil cases
	if expected == nil {
		return // null in expected means any value is acceptable
	}

	if actual == nil {
		result.Success = false
		result.Errors = append(result.Errors, MatchFailure{
			Path:     path,
			Expected: fmt.Sprintf("%T", expected),
			Actual:   "null",
			Message:  fmt.Sprintf("expected %T, got null", expected),
		})
		return
	}

	// Normalize both to comparable types
	expNorm, err := normalizeData(expected)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, MatchFailure{
			Path:    path,
			Message: fmt.Sprintf("failed to normalize expected data: %v", err),
		})
		return
	}
	actNorm, err := normalizeData(actual)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, MatchFailure{
			Path:    path,
			Message: fmt.Sprintf("failed to normalize actual data: %v", err),
		})
		return
	}

	switch exp := expNorm.(type) {
	case map[string]any:
		act, ok := actNorm.(map[string]any)
		if !ok {
			result.Success = false
			result.Errors = append(result.Errors, MatchFailure{
				Path:     path,
				Expected: "object",
				Actual:   fmt.Sprintf("%T", actual),
				Message:  fmt.Sprintf("expected object, got %T", actual),
			})
			return
		}

		// Check each expected key exists in actual
		for key, expVal := range exp {
			actVal, ok := act[key]
			if !ok {
				result.Success = false
				result.Errors = append(result.Errors, MatchFailure{
					Path:     path + "." + key,
					Expected: "present",
					Actual:   "missing",
					Message:  fmt.Sprintf("expected property %q to be present", key),
				})
				continue
			}
			matchStructureRecursive(path+"."+key, expVal, actVal, result)
		}

	case []any:
		act, ok := actNorm.([]any)
		if !ok {
			result.Success = false
			result.Errors = append(result.Errors, MatchFailure{
				Path:     path,
				Expected: "array",
				Actual:   fmt.Sprintf("%T", actual),
				Message:  fmt.Sprintf("expected array, got %T", actual),
			})
			return
		}

		// For arrays, check structure of first element against all actual elements
		if len(exp) > 0 && len(act) > 0 {
			template := exp[0]
			for i, actElem := range act {
				matchStructureRecursive(fmt.Sprintf("%s[%d]", path, i), template, actElem, result)
			}
		}

	default:
		// For primitive types, just check type compatibility
		if !typesCompatible(exp, actNorm) {
			result.Success = false
			result.Errors = append(result.Errors, MatchFailure{
				Path:     path,
				Expected: typeName(exp),
				Actual:   typeName(actNorm),
				Message:  fmt.Sprintf("expected %s, got %s", typeName(exp), typeName(actNorm)),
			})
		}
	}
}

// normalizeData converts data to a consistent format for comparison.
func normalizeData(data any) (any, error) {
	if data == nil {
		return nil, nil
	}

	// If it's already a basic type, return as-is
	switch data.(type) {
	case map[string]any, []any, string, float64, bool:
		return data, nil
	}

	// Otherwise, marshal and unmarshal to normalize
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	var normalized any
	if err := json.Unmarshal(bytes, &normalized); err != nil {
		return nil, err
	}

	return normalized, nil
}

// containsWildcard checks if a path contains wildcard notation.
func containsWildcard(path string) bool {
	return path != "" && (path[len(path)-1] == '*' ||
		(len(path) > 1 && path[len(path)-2:] == "[*]") ||
		contains(path, "[*]"))
}

// typesCompatible checks if two values have compatible types.
func typesCompatible(expected, actual any) bool {
	if expected == nil || actual == nil {
		return true
	}

	expType := reflect.TypeOf(expected)
	actType := reflect.TypeOf(actual)

	// Same type
	if expType == actType {
		return true
	}

	// Numeric types are compatible
	expKind := expType.Kind()
	actKind := actType.Kind()

	numericKinds := map[reflect.Kind]bool{
		reflect.Int:     true,
		reflect.Int8:    true,
		reflect.Int16:   true,
		reflect.Int32:   true,
		reflect.Int64:   true,
		reflect.Uint:    true,
		reflect.Uint8:   true,
		reflect.Uint16:  true,
		reflect.Uint32:  true,
		reflect.Uint64:  true,
		reflect.Float32: true,
		reflect.Float64: true,
	}

	if numericKinds[expKind] && numericKinds[actKind] {
		return true
	}

	return false
}

// typeName returns a human-readable type name.
func typeName(v any) string {
	if v == nil {
		return "null"
	}

	switch v.(type) {
	case string:
		return "string"
	case float64, float32, int, int64, int32:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}
