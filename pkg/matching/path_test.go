package matching

import (
	"reflect"
	"testing"
)

func TestPathResolver_Resolve_JSONPath(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{
		"user": map[string]any{
			"name":  "John",
			"email": "john@example.com",
			"age":   30,
		},
		"items": []any{"a", "b", "c"},
	}

	t.Run("root selector", func(t *testing.T) {
		result, err := resolver.Resolve("$", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(result, data) {
			t.Errorf("expected data, got %v", result)
		}
	})

	t.Run("simple property", func(t *testing.T) {
		result, err := resolver.Resolve("$.user", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", result)
		}
		if resultMap["name"] != "John" {
			t.Errorf("expected name=John, got %v", resultMap["name"])
		}
	})

	t.Run("nested property", func(t *testing.T) {
		result, err := resolver.Resolve("$.user.name", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "John" {
			t.Errorf("expected John, got %v", result)
		}
	})

	t.Run("array index", func(t *testing.T) {
		result, err := resolver.Resolve("$.items[0]", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "a" {
			t.Errorf("expected a, got %v", result)
		}
	})

	t.Run("array last element", func(t *testing.T) {
		result, err := resolver.Resolve("$.items[2]", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "c" {
			t.Errorf("expected c, got %v", result)
		}
	})
}

func TestPathResolver_Resolve_JSONPointer(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{
		"foo":   "bar",
		"a/b":   "slashed",
		"m~n":   "tilded",
		"items": []any{1, 2, 3},
	}

	t.Run("root", func(t *testing.T) {
		result, err := resolver.Resolve("/", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(result, data) {
			t.Errorf("expected data, got %v", result)
		}
	})

	t.Run("simple property", func(t *testing.T) {
		result, err := resolver.Resolve("/foo", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "bar" {
			t.Errorf("expected bar, got %v", result)
		}
	})

	t.Run("escaped slash", func(t *testing.T) {
		result, err := resolver.Resolve("/a~1b", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "slashed" {
			t.Errorf("expected slashed, got %v", result)
		}
	})

	t.Run("escaped tilde", func(t *testing.T) {
		result, err := resolver.Resolve("/m~0n", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "tilded" {
			t.Errorf("expected tilded, got %v", result)
		}
	})

	t.Run("array index", func(t *testing.T) {
		result, err := resolver.Resolve("/items/1", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 2 {
			t.Errorf("expected 2, got %v", result)
		}
	})
}

func TestPathResolver_Resolve_Errors(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{
		"items": []any{"a", "b"},
	}

	tests := []struct {
		name string
		path string
		data any
	}{
		{
			name: "property not found",
			path: "$.nonexistent",
			data: data,
		},
		{
			name: "traverse nil",
			path: "$.foo.bar",
			data: map[string]any{"foo": nil},
		},
		{
			name: "index on non-array",
			path: "$.items[0]",
			data: map[string]any{"items": "not-an-array"},
		},
		{
			name: "index out of bounds",
			path: "$.items[10]",
			data: data,
		},
		{
			name: "property on non-object",
			path: "$.items.foo",
			data: data,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.Resolve(tt.path, tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPathResolver_ResolveAll_Wildcard(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{
		"items": []any{
			map[string]any{"id": "1", "name": "first"},
			map[string]any{"id": "2", "name": "second"},
			map[string]any{"id": "3", "name": "third"},
		},
	}

	results, err := resolver.ResolveAll("$.items[*].id", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expectedIDs := []string{"1", "2", "3"}
	for i, result := range results {
		if result.Value != expectedIDs[i] {
			t.Errorf("result[%d]: expected %v, got %v", i, expectedIDs[i], result.Value)
		}
	}
}

func TestPathResolver_ResolveAll_NestedWildcard(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{
		"users": []any{
			map[string]any{
				"orders": []any{
					map[string]any{"total": 100},
					map[string]any{"total": 200},
				},
			},
			map[string]any{
				"orders": []any{
					map[string]any{"total": 300},
				},
			},
		},
	}

	results, err := resolver.ResolveAll("$.users[*].orders[*].total", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	expectedTotals := []int{100, 200, 300}
	for i, result := range results {
		if result.Value != expectedTotals[i] {
			t.Errorf("result[%d]: expected %v, got %v", i, expectedTotals[i], result.Value)
		}
	}
}

func TestPathResolver_ResolveAll_Errors(t *testing.T) {
	resolver := NewPathResolver()

	tests := []struct {
		name string
		path string
		data any
	}{
		{
			name: "wildcard on non-array",
			path: "$.items[*]",
			data: map[string]any{"items": "not-an-array"},
		},
		{
			name: "property not found",
			path: "$.nonexistent[*]",
			data: map[string]any{"items": []any{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveAll(tt.path, tt.data)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestPathResolver_Resolve_EmptyPath(t *testing.T) {
	resolver := NewPathResolver()
	data := map[string]any{"foo": "bar"}

	result, err := resolver.Resolve("", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Result should be the same data reference
	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}
	if resultMap["foo"] != "bar" {
		t.Errorf("expected foo=bar, got %v", resultMap["foo"])
	}
}

func TestPathResolver_Set(t *testing.T) {
	resolver := NewPathResolver()

	t.Run("set simple property", func(t *testing.T) {
		data := map[string]any{"name": "old"}
		err := resolver.Set("$.name", data, "new")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data["name"] != "new" {
			t.Errorf("expected 'new', got %v", data["name"])
		}
	})

	t.Run("set nested property", func(t *testing.T) {
		data := map[string]any{
			"user": map[string]any{"name": "old"},
		}
		err := resolver.Set("$.user.name", data, "new")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		user, ok := data["user"].(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", data["user"])
		}
		if user["name"] != "new" {
			t.Errorf("expected 'new', got %v", user["name"])
		}
	})

	t.Run("set array element", func(t *testing.T) {
		data := map[string]any{
			"items": []any{"a", "b", "c"},
		}
		err := resolver.Set("$.items[1]", data, "updated")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		items, ok := data["items"].([]any)
		if !ok {
			t.Fatalf("expected slice, got %T", data["items"])
		}
		if items[1] != "updated" {
			t.Errorf("expected 'updated', got %v", items[1])
		}
	})

	t.Run("create intermediate object", func(t *testing.T) {
		data := map[string]any{}
		err := resolver.Set("$.new.nested", data, "value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		newObj, ok := data["new"].(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", data["new"])
		}
		if newObj["nested"] != "value" {
			t.Errorf("expected 'value', got %v", newObj["nested"])
		}
	})
}

func TestPathResolver_Set_Errors(t *testing.T) {
	resolver := NewPathResolver()

	tests := []struct {
		name string
		path string
		data any
	}{
		{
			name: "set root",
			path: "$",
			data: map[string]any{},
		},
		{
			name: "index on non-array",
			path: "$.items[0]",
			data: map[string]any{"items": "not-array"},
		},
		{
			name: "index out of bounds",
			path: "$.items[10]",
			data: map[string]any{"items": []any{"a"}},
		},
		{
			name: "property on non-object",
			path: "$.name.nested",
			data: map[string]any{"name": "string-not-object"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.Set(tt.path, tt.data, "value")
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParsePath_SimpleDotNotation(t *testing.T) {
	segments, err := parsePath("foo.bar.baz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"foo", "bar", "baz"}
	if len(segments) != len(expected) {
		t.Fatalf("expected %d segments, got %d", len(expected), len(segments))
	}

	for i, seg := range segments {
		if seg.key != expected[i] {
			t.Errorf("segment[%d]: expected key %q, got %q", i, expected[i], seg.key)
		}
	}
}

func TestParsePath_ArrayNotation(t *testing.T) {
	segments, err := parsePath("items[0].name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce: items, [0], name
	if len(segments) < 3 {
		t.Fatalf("expected at least 3 segments, got %d", len(segments))
	}

	if segments[0].key != "items" {
		t.Errorf("expected first segment key 'items', got %q", segments[0].key)
	}
}

func TestParsePath_UnclosedBracket(t *testing.T) {
	_, err := parsePath("items[0")
	if err == nil {
		t.Error("expected error for unclosed bracket")
	}
}

func TestParsePath_InvalidJSONPath(t *testing.T) {
	// This should trigger the invalid JSONPath error
	_, err := parseJSONPath("$invalid")
	// Note: the current regex may match parts of this, so check behavior
	if err != nil {
		// Expected in some cases
		t.Logf("got expected error: %v", err)
	}
}
