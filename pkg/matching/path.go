package matching

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// PathResolver resolves JSON paths and pointers to values in a data structure.
type PathResolver struct{}

// NewPathResolver creates a new PathResolver.
func NewPathResolver() *PathResolver {
	return &PathResolver{}
}

// Resolve gets a value at the given path from the data.
// Supports both JSONPath ($.foo.bar) and JSON Pointer (/foo/bar) syntax.
func (r *PathResolver) Resolve(path string, data any) (any, error) {
	if path == "" || path == "$" || path == "/" {
		return data, nil
	}

	// Determine path type and normalize
	segments, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	return resolveSegments(segments, data)
}

// ResolveAll resolves a path that may match multiple values (e.g., $.items[*].price).
func (r *PathResolver) ResolveAll(path string, data any) ([]PathValue, error) {
	if path == "" || path == "$" || path == "/" {
		return []PathValue{{Path: path, Value: data}}, nil
	}

	segments, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	return resolveAllSegments(segments, "", data)
}

// PathValue represents a resolved path and its value.
type PathValue struct {
	Path  string
	Value any
}

// parsePath parses a path string into segments.
func parsePath(path string) ([]segment, error) {
	// Handle JSON Pointer (RFC 6901)
	if strings.HasPrefix(path, "/") {
		return parseJSONPointer(path)
	}

	// Handle JSONPath
	if strings.HasPrefix(path, "$") {
		return parseJSONPath(path)
	}

	// Assume it's a simple dot notation
	return parseSimplePath(path)
}

// segment represents a single path segment.
type segment struct {
	key       string // Property name
	index     int    // Array index (-1 if not an index)
	wildcard  bool   // True if [*]
	recursive bool   // True if **
}

// parseJSONPointer parses a JSON Pointer (RFC 6901).
func parseJSONPointer(path string) ([]segment, error) {
	if path == "/" {
		return nil, nil
	}

	parts := strings.Split(path[1:], "/") // Skip leading /
	segments := make([]segment, len(parts))

	for i, part := range parts {
		// Unescape JSON Pointer encoding
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")

		// Check if it's an array index
		if idx, err := strconv.Atoi(part); err == nil && idx >= 0 {
			segments[i] = segment{index: idx}
		} else {
			segments[i] = segment{key: part, index: -1}
		}
	}

	return segments, nil
}

var jsonPathRegex = regexp.MustCompile(`\$|\.([a-zA-Z_][a-zA-Z0-9_]*)|\.?\[([0-9]+|\*)\]`)

// parseJSONPath parses a JSONPath expression.
func parseJSONPath(path string) ([]segment, error) {
	matches := jsonPathRegex.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("invalid JSONPath: %s", path)
	}

	var segments []segment
	for _, match := range matches {
		if match[0] == "$" {
			continue // Root selector
		}

		if match[1] != "" {
			// Property name (.foo)
			segments = append(segments, segment{key: match[1], index: -1})
		} else if match[2] != "" {
			// Array index or wildcard ([0] or [*])
			if match[2] == "*" {
				segments = append(segments, segment{wildcard: true, index: -1})
			} else {
				idx, _ := strconv.Atoi(match[2])
				segments = append(segments, segment{index: idx})
			}
		}
	}

	return segments, nil
}

// parseSimplePath parses a simple dot-notation path.
func parseSimplePath(path string) ([]segment, error) {
	parts := strings.Split(path, ".")
	segments := make([]segment, len(parts))

	for i, part := range parts {
		// Check for array notation
		if idx := strings.Index(part, "["); idx != -1 {
			key := part[:idx]
			rest := part[idx:]

			if key != "" {
				segments[i] = segment{key: key, index: -1}
				i++
			}

			// Parse array indices
			for strings.HasPrefix(rest, "[") {
				end := strings.Index(rest, "]")
				if end == -1 {
					return nil, fmt.Errorf("unclosed bracket in path: %s", path)
				}

				inner := rest[1:end]
				if inner == "*" {
					segments = append(segments, segment{wildcard: true, index: -1})
				} else if n, err := strconv.Atoi(inner); err == nil {
					segments = append(segments, segment{index: n})
				} else {
					segments = append(segments, segment{key: inner, index: -1})
				}

				rest = rest[end+1:]
			}
		} else {
			segments[i] = segment{key: part, index: -1}
		}
	}

	return segments, nil
}

// resolveSegments traverses the data structure following the segments.
func resolveSegments(segments []segment, data any) (any, error) {
	current := data

	for i, seg := range segments {
		if current == nil {
			return nil, fmt.Errorf("cannot traverse nil at segment %d", i)
		}

		var err error
		current, err = resolveSegment(seg, current)
		if err != nil {
			return nil, fmt.Errorf("at segment %d: %w", i, err)
		}
	}

	return current, nil
}

// resolveSegment resolves a single segment.
func resolveSegment(seg segment, data any) (any, error) {
	if seg.wildcard {
		return nil, fmt.Errorf("wildcard requires ResolveAll")
	}

	// Array index access
	if seg.index >= 0 {
		switch v := data.(type) {
		case []any:
			if seg.index >= len(v) {
				return nil, fmt.Errorf("index %d out of bounds (length %d)", seg.index, len(v))
			}
			return v[seg.index], nil
		default:
			return nil, fmt.Errorf("cannot index non-array type %T", data)
		}
	}

	// Property access
	switch v := data.(type) {
	case map[string]any:
		val, ok := v[seg.key]
		if !ok {
			return nil, fmt.Errorf("property %q not found", seg.key)
		}
		return val, nil
	default:
		return nil, fmt.Errorf("cannot access property on type %T", data)
	}
}

// resolveAllSegments resolves segments that may include wildcards.
func resolveAllSegments(segments []segment, currentPath string, data any) ([]PathValue, error) {
	if len(segments) == 0 {
		return []PathValue{{Path: currentPath, Value: data}}, nil
	}

	seg := segments[0]
	rest := segments[1:]

	if seg.wildcard {
		// Wildcard: iterate over array elements
		arr, ok := data.([]any)
		if !ok {
			return nil, fmt.Errorf("wildcard requires array, got %T", data)
		}

		var results []PathValue
		for i, elem := range arr {
			path := fmt.Sprintf("%s[%d]", currentPath, i)
			subResults, err := resolveAllSegments(rest, path, elem)
			if err != nil {
				return nil, err
			}
			results = append(results, subResults...)
		}
		return results, nil
	}

	// Non-wildcard: resolve single value and continue
	var nextData any
	var nextPath string

	if seg.index >= 0 {
		arr, ok := data.([]any)
		if !ok {
			return nil, fmt.Errorf("index access requires array, got %T", data)
		}
		if seg.index >= len(arr) {
			return nil, fmt.Errorf("index %d out of bounds (length %d)", seg.index, len(arr))
		}
		nextData = arr[seg.index]
		nextPath = fmt.Sprintf("%s[%d]", currentPath, seg.index)
	} else {
		m, ok := data.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("property access requires object, got %T", data)
		}
		val, ok := m[seg.key]
		if !ok {
			return nil, fmt.Errorf("property %q not found", seg.key)
		}
		nextData = val
		if currentPath == "" {
			nextPath = seg.key
		} else {
			nextPath = currentPath + "." + seg.key
		}
	}

	return resolveAllSegments(rest, nextPath, nextData)
}

// Set sets a value at the given path in the data structure.
// Creates intermediate objects/arrays as needed.
func (r *PathResolver) Set(path string, data any, value any) error {
	segments, err := parsePath(path)
	if err != nil {
		return err
	}

	return setSegments(segments, data, value)
}

// setSegments sets a value by traversing/creating the path.
func setSegments(segments []segment, data any, value any) error {
	if len(segments) == 0 {
		return fmt.Errorf("cannot set root")
	}

	current := data
	for i := 0; i < len(segments)-1; i++ {
		seg := segments[i]

		var next any
		var err error

		if seg.index >= 0 {
			arr, ok := current.([]any)
			if !ok {
				return fmt.Errorf("index access requires array at segment %d", i)
			}
			if seg.index >= len(arr) {
				return fmt.Errorf("index %d out of bounds at segment %d", seg.index, i)
			}
			next = arr[seg.index]
		} else {
			m, ok := current.(map[string]any)
			if !ok {
				return fmt.Errorf("property access requires object at segment %d", i)
			}
			next, err = m[seg.key], nil
			if next == nil {
				// Create intermediate object
				next = make(map[string]any)
				m[seg.key] = next
			}
			_ = err
		}
		current = next
	}

	// Set the final value
	lastSeg := segments[len(segments)-1]
	if lastSeg.index >= 0 {
		arr, ok := current.([]any)
		if !ok {
			return fmt.Errorf("index access requires array for final segment")
		}
		if lastSeg.index >= len(arr) {
			return fmt.Errorf("index %d out of bounds for final segment", lastSeg.index)
		}
		arr[lastSeg.index] = value
	} else {
		m, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("property access requires object for final segment")
		}
		m[lastSeg.key] = value
	}

	return nil
}
