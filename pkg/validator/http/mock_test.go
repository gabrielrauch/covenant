package httpvalidator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newHTTPInteraction builds a contract.Interaction with an HTTP payload.
func newHTTPInteraction(t *testing.T, id, desc string, req contract.HTTPRequest, resp contract.HTTPResponse) contract.Interaction {
	t.Helper()
	return contract.Interaction{
		ID:          id,
		Description: desc,
		Protocol:    contract.ProtocolHTTP,
		Payload: contract.Payload{
			HTTP: &contract.HTTPPayload{
				Request:  req,
				Response: resp,
			},
		},
	}
}

// startMock creates and starts a MockServer, registering cleanup via t.Cleanup.
func startMock(t *testing.T, interactions []contract.Interaction, opts ...MockServerOption) *MockServer {
	t.Helper()
	ms := NewMockServer(interactions, opts...)
	if err := ms.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	t.Cleanup(func() {
		if err := ms.Stop(); err != nil {
			t.Errorf("Stop() error: %v", err)
		}
	})
	return ms
}

// doRequest performs an HTTP request against the mock server and returns the response.
func doRequest(t *testing.T, method, url string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, body)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext error: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request error: %v", err)
	}
	return resp
}

// readBody reads and returns the response body as a string.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	return string(b)
}

// ---------------------------------------------------------------------------
// Start / Stop / URL lifecycle
// ---------------------------------------------------------------------------

func TestMockServer_StartStopURL(t *testing.T) {
	t.Run("URL is empty before Start", func(t *testing.T) {
		ms := NewMockServer(nil)
		if got := ms.URL(); got != "" {
			t.Errorf("URL() before Start = %q, want empty", got)
		}
	})

	t.Run("Start populates URL then Stop clears server", func(t *testing.T) {
		ms := NewMockServer(nil)
		if err := ms.Start(); err != nil {
			t.Fatalf("Start() error: %v", err)
		}

		u := ms.URL()
		if u == "" {
			t.Fatal("URL() after Start is empty")
		}
		if !strings.HasPrefix(u, "http://127.0.0.1:") {
			t.Errorf("URL() = %q, want prefix http://127.0.0.1:", u)
		}

		if err := ms.Stop(); err != nil {
			t.Fatalf("Stop() error: %v", err)
		}
	})

	t.Run("Stop on unstarted server returns nil", func(t *testing.T) {
		ms := NewMockServer(nil)
		if err := ms.Stop(); err != nil {
			t.Errorf("Stop() on unstarted server = %v, want nil", err)
		}
	})

	t.Run("default mode is strict", func(t *testing.T) {
		ms := NewMockServer(nil)
		if ms.mode != MockModeStrict {
			t.Errorf("default mode = %q, want %q", ms.mode, MockModeStrict)
		}
	})

	t.Run("WithMode sets mode", func(t *testing.T) {
		ms := NewMockServer(nil, WithMode(MockModeLenient))
		if ms.mode != MockModeLenient {
			t.Errorf("mode = %q, want %q", ms.mode, MockModeLenient)
		}
	})
}

// ---------------------------------------------------------------------------
// ServeHTTP: matching vs non-matching in Strict/Lenient modes
// ---------------------------------------------------------------------------

func TestMockServer_ServeHTTP(t *testing.T) {
	interaction := newHTTPInteraction(t, "get-users", "get users",
		contract.HTTPRequest{
			Method: "GET",
			Path:   "/api/users",
		},
		contract.HTTPResponse{
			Status: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]any{
				"users": []any{},
			},
		},
	)

	t.Run("matching request returns canned response", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{interaction})
		resp := doRequest(t, "GET", ms.URL()+"/api/users", nil)
		body := readBody(t, resp)

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var parsed map[string]any
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			t.Fatalf("body is not valid JSON: %v", err)
		}
		if _, ok := parsed["users"]; !ok {
			t.Errorf("body missing 'users' key: %s", body)
		}
	})

	t.Run("unmatched request in strict mode returns 400", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{interaction})
		resp := doRequest(t, "GET", ms.URL()+"/unknown", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("unmatched request in lenient mode returns 404", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{interaction}, WithMode(MockModeLenient))
		resp := doRequest(t, "GET", ms.URL()+"/unknown", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("unmatched request in record mode returns 404", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{interaction}, WithMode(MockModeRecord))
		resp := doRequest(t, "GET", ms.URL()+"/unknown", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})

	t.Run("wrong method does not match", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{interaction})
		resp := doRequest(t, "POST", ms.URL()+"/api/users", nil)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want %d (strict, wrong method)", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("request with body that matches interaction with body", func(t *testing.T) {
		postInteraction := newHTTPInteraction(t, "create-user", "create user",
			contract.HTTPRequest{
				Method: "POST",
				Path:   "/api/users",
				Body: map[string]any{
					"name": "Alice",
				},
			},
			contract.HTTPResponse{
				Status: 201,
				Body: map[string]any{
					"id":   "new-id",
					"name": "Alice",
				},
			},
		)
		ms := startMock(t, []contract.Interaction{postInteraction})
		resp := doRequest(t, "POST", ms.URL()+"/api/users", strings.NewReader(`{"name":"Alice"}`))
		body := readBody(t, resp)

		if resp.StatusCode != 201 {
			t.Errorf("status = %d, want 201; body = %s", resp.StatusCode, body)
		}
	})

	t.Run("request body validation failure in strict mode returns 400", func(t *testing.T) {
		postInteraction := newHTTPInteraction(t, "create-user-strict", "create user",
			contract.HTTPRequest{
				Method: "POST",
				Path:   "/api/items",
				Body: map[string]any{
					"count": 5,
				},
			},
			contract.HTTPResponse{
				Status: 201,
			},
		)
		ms := startMock(t, []contract.Interaction{postInteraction})
		// Send a body with wrong type (string instead of number)
		resp := doRequest(t, "POST", ms.URL()+"/api/items", strings.NewReader(`{"count":"not-a-number"}`))
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want %d (strict, body type mismatch)", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("request body validation failure in lenient mode still returns canned response", func(t *testing.T) {
		postInteraction := newHTTPInteraction(t, "create-item-lenient", "create item",
			contract.HTTPRequest{
				Method: "POST",
				Path:   "/api/items",
				Body: map[string]any{
					"count": 5,
				},
			},
			contract.HTTPResponse{
				Status: 201,
			},
		)
		ms := startMock(t, []contract.Interaction{postInteraction}, WithMode(MockModeLenient))
		resp := doRequest(t, "POST", ms.URL()+"/api/items", strings.NewReader(`{"count":"not-a-number"}`))
		defer resp.Body.Close()

		// Lenient mode should still send the canned response
		if resp.StatusCode != 201 {
			t.Errorf("status = %d, want 201 (lenient mode, body mismatch)", resp.StatusCode)
		}
	})
}

// ---------------------------------------------------------------------------
// matchPath
// ---------------------------------------------------------------------------

func TestMatchPath(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		want     bool
	}{
		{
			name:     "exact match",
			expected: "/api/users",
			actual:   "/api/users",
			want:     true,
		},
		{
			name:     "exact match root",
			expected: "/",
			actual:   "/",
			want:     true,
		},
		{
			name:     "no match different path",
			expected: "/api/users",
			actual:   "/api/orders",
			want:     false,
		},
		{
			name:     "no match trailing slash",
			expected: "/api/users",
			actual:   "/api/users/",
			want:     false,
		},
		{
			name:     "placeholder single segment",
			expected: "/api/users/{id}",
			actual:   "/api/users/123",
			want:     true,
		},
		{
			name:     "placeholder uuid style",
			expected: "/api/users/{id}",
			actual:   "/api/users/550e8400-e29b-41d4-a716-446655440000",
			want:     true,
		},
		{
			name:     "placeholder multiple segments",
			expected: "/api/users/{userId}/orders/{orderId}",
			actual:   "/api/users/42/orders/99",
			want:     true,
		},
		{
			name:     "placeholder does not match slash",
			expected: "/api/users/{id}",
			actual:   "/api/users/12/extra",
			want:     false,
		},
		{
			name:     "placeholder wrong prefix",
			expected: "/api/users/{id}",
			actual:   "/api/orders/123",
			want:     false,
		},
		{
			name:     "no match empty actual",
			expected: "/api/users",
			actual:   "",
			want:     false,
		},
		{
			name:     "both empty",
			expected: "",
			actual:   "",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchPath(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("matchPath(%q, %q) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// matchQueryParams
// ---------------------------------------------------------------------------

func TestMatchQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string]string
		actual   map[string][]string
		want     bool
	}{
		{
			name:     "nil expected matches anything",
			expected: nil,
			actual:   map[string][]string{"foo": {"bar"}},
			want:     true,
		},
		{
			name:     "empty expected matches anything",
			expected: map[string]string{},
			actual:   map[string][]string{"foo": {"bar"}},
			want:     true,
		},
		{
			name:     "present and matching",
			expected: map[string]string{"page": "1", "limit": "10"},
			actual:   map[string][]string{"page": {"1"}, "limit": {"10"}},
			want:     true,
		},
		{
			name:     "missing key",
			expected: map[string]string{"page": "1"},
			actual:   map[string][]string{},
			want:     false,
		},
		{
			name:     "key present but empty values",
			expected: map[string]string{"page": "1"},
			actual:   map[string][]string{"page": {}},
			want:     false,
		},
		{
			name:     "key present but wrong value",
			expected: map[string]string{"page": "1"},
			actual:   map[string][]string{"page": {"2"}},
			want:     false,
		},
		{
			name:     "partial match missing one key",
			expected: map[string]string{"page": "1", "limit": "10"},
			actual:   map[string][]string{"page": {"1"}},
			want:     false,
		},
		{
			name:     "actual has extra params which is ok",
			expected: map[string]string{"page": "1"},
			actual:   map[string][]string{"page": {"1"}, "sort": {"asc"}},
			want:     true,
		},
		{
			name:     "multi-valued actual with correct value present",
			expected: map[string]string{"tag": "go"},
			actual:   map[string][]string{"tag": {"rust", "go", "python"}},
			want:     true,
		},
		{
			name:     "multi-valued actual without correct value",
			expected: map[string]string{"tag": "java"},
			actual:   map[string][]string{"tag": {"rust", "go", "python"}},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchQueryParams(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("matchQueryParams(%v, %v) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// processTemplate
// ---------------------------------------------------------------------------

//nolint:gocyclo // test function
func TestProcessTemplate(t *testing.T) {
	ms := NewMockServer(nil)

	tests := []struct {
		name  string
		input string
		check func(t *testing.T, got string)
	}{
		{
			name:  "no template markers",
			input: "plain-value",
			check: func(t *testing.T, got string) {
				t.Helper()
				if got != "plain-value" {
					t.Errorf("got %q, want %q", got, "plain-value")
				}
			},
		},
		{
			name:  "now() replaced with timestamp",
			input: "${now()}",
			check: func(t *testing.T, got string) {
				t.Helper()
				if got == "${now()}" {
					t.Error("${now()} was not replaced")
				}
				// The implementation returns a fixed timestamp
				if got != "2024-01-15T10:30:00Z" {
					t.Errorf("got %q, want %q", got, "2024-01-15T10:30:00Z")
				}
			},
		},
		{
			name:  "uuid() replaced with UUID-like value",
			input: "${uuid()}",
			check: func(t *testing.T, got string) {
				t.Helper()
				if got == "${uuid()}" {
					t.Error("${uuid()} was not replaced")
				}
				// The implementation returns a fixed UUID
				if got != "550e8400-e29b-41d4-a716-446655440000" {
					t.Errorf("got %q, want %q", got, "550e8400-e29b-41d4-a716-446655440000")
				}
			},
		},
		{
			name:  "now() embedded in string",
			input: "generated-at-${now()}-end",
			check: func(t *testing.T, got string) {
				t.Helper()
				if strings.Contains(got, "${now()}") {
					t.Error("${now()} was not replaced")
				}
				if !strings.HasPrefix(got, "generated-at-") || !strings.HasSuffix(got, "-end") {
					t.Errorf("unexpected result: %q", got)
				}
			},
		},
		{
			name:  "uuid() embedded in string",
			input: "id-${uuid()}-suffix",
			check: func(t *testing.T, got string) {
				t.Helper()
				if strings.Contains(got, "${uuid()}") {
					t.Error("${uuid()} was not replaced")
				}
				if !strings.HasPrefix(got, "id-") || !strings.HasSuffix(got, "-suffix") {
					t.Errorf("unexpected result: %q", got)
				}
			},
		},
		{
			name:  "both now and uuid in one string",
			input: "${now()}-${uuid()}",
			check: func(t *testing.T, got string) {
				t.Helper()
				if strings.Contains(got, "${now()}") || strings.Contains(got, "${uuid()}") {
					t.Error("template markers were not replaced")
				}
				parts := strings.SplitN(got, "-550e8400", 2)
				if len(parts) != 2 {
					t.Errorf("unexpected result: %q", got)
				}
			},
		},
		{
			name:  "empty string",
			input: "",
			check: func(t *testing.T, got string) {
				t.Helper()
				if got != "" {
					t.Errorf("got %q, want empty", got)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ms.processTemplate(tt.input, nil)
			tt.check(t, got)
		})
	}
}

// ---------------------------------------------------------------------------
// processBodyTemplate
// ---------------------------------------------------------------------------

//nolint:gocyclo // test function
func TestProcessBodyTemplate(t *testing.T) {
	ms := NewMockServer(nil)

	t.Run("string value with template", func(t *testing.T) {
		got := ms.processBodyTemplate("${now()}", nil)
		s, ok := got.(string)
		if !ok {
			t.Fatalf("expected string, got %T", got)
		}
		if s == "${now()}" {
			t.Error("template was not replaced")
		}
	})

	t.Run("map with nested templates", func(t *testing.T) {
		input := map[string]any{
			"timestamp": "${now()}",
			"id":        "${uuid()}",
			"plain":     "hello",
		}
		got := ms.processBodyTemplate(input, nil)
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("expected map, got %T", got)
		}
		if m["timestamp"] == "${now()}" {
			t.Error("timestamp template not replaced")
		}
		if m["id"] == "${uuid()}" {
			t.Error("id template not replaced")
		}
		if m["plain"] != "hello" {
			t.Errorf("plain value changed: %v", m["plain"])
		}
	})

	t.Run("slice with templates", func(t *testing.T) {
		input := []any{"${now()}", "static", "${uuid()}"}
		got := ms.processBodyTemplate(input, nil)
		s, ok := got.([]any)
		if !ok {
			t.Fatalf("expected slice, got %T", got)
		}
		if len(s) != 3 {
			t.Fatalf("slice len = %d, want 3", len(s))
		}
		if s[0] == "${now()}" {
			t.Error("first element not replaced")
		}
		if s[1] != "static" {
			t.Errorf("second element changed: %v", s[1])
		}
		if s[2] == "${uuid()}" {
			t.Error("third element not replaced")
		}
	})

	t.Run("non-string types pass through", func(t *testing.T) {
		for _, v := range []any{42, 3.14, true, nil} {
			got := ms.processBodyTemplate(v, nil)
			if got != v {
				t.Errorf("processBodyTemplate(%v) = %v, want %v", v, got, v)
			}
		}
	})

	t.Run("deeply nested map and slice", func(t *testing.T) {
		input := map[string]any{
			"outer": map[string]any{
				"inner": []any{
					map[string]any{
						"ts": "${now()}",
					},
				},
			},
		}
		got := ms.processBodyTemplate(input, nil)
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatal("type assertion failed: got is not map[string]any")
		}
		outer, ok := m["outer"].(map[string]any)
		if !ok {
			t.Fatal("type assertion failed: outer is not map[string]any")
		}
		inner, ok := outer["inner"].([]any)
		if !ok {
			t.Fatal("type assertion failed: inner is not []any")
		}
		leaf, ok := inner[0].(map[string]any)
		if !ok {
			t.Fatal("type assertion failed: leaf is not map[string]any")
		}
		if leaf["ts"] == "${now()}" {
			t.Error("deeply nested template not replaced")
		}
	})
}

// ---------------------------------------------------------------------------
// ReceivedRequests / MatchCounts / Verify / Reset
// ---------------------------------------------------------------------------

func TestMockServer_ReceivedRequests(t *testing.T) {
	interaction := newHTTPInteraction(t, "ix-1", "get items",
		contract.HTTPRequest{Method: "GET", Path: "/items"},
		contract.HTTPResponse{Status: 200, Body: map[string]any{"items": []any{}}},
	)
	ms := startMock(t, []contract.Interaction{interaction})

	// Initially empty
	if reqs := ms.ReceivedRequests(); len(reqs) != 0 {
		t.Errorf("initial ReceivedRequests len = %d, want 0", len(reqs))
	}

	// Make a matching request
	resp := doRequest(t, "GET", ms.URL()+"/items", nil)
	resp.Body.Close()

	reqs := ms.ReceivedRequests()
	if len(reqs) != 1 {
		t.Fatalf("ReceivedRequests len = %d, want 1", len(reqs))
	}
	if reqs[0].Method != "GET" {
		t.Errorf("recorded method = %q, want GET", reqs[0].Method)
	}
	if reqs[0].Path != "/items" {
		t.Errorf("recorded path = %q, want /items", reqs[0].Path)
	}
	if !reqs[0].Matched {
		t.Error("request should be marked as matched")
	}

	// Make an unmatched request (strict mode returns 400)
	resp = doRequest(t, "GET", ms.URL()+"/nope", nil)
	resp.Body.Close()

	reqs = ms.ReceivedRequests()
	if len(reqs) != 2 {
		t.Fatalf("ReceivedRequests len = %d, want 2", len(reqs))
	}
	if reqs[1].Matched {
		t.Error("unmatched request should not be marked as matched")
	}
	if reqs[1].Error == "" {
		t.Error("unmatched request should have an error message")
	}
}

func TestMockServer_ReceivedRequests_ReturnsCopy(t *testing.T) {
	ms := NewMockServer(nil)
	// Manually record so we don't need a running server
	ms.recordRequest(&RecordedRequest{Method: "GET", Path: "/a"})

	reqs := ms.ReceivedRequests()
	reqs[0].Method = "MODIFIED"

	// Original should be unchanged
	original := ms.ReceivedRequests()
	if original[0].Method != "GET" {
		t.Error("ReceivedRequests should return a copy, not a reference")
	}
}

func TestMockServer_MatchCounts(t *testing.T) {
	ix1 := newHTTPInteraction(t, "ix-a", "get items",
		contract.HTTPRequest{Method: "GET", Path: "/a"},
		contract.HTTPResponse{Status: 200},
	)
	ix2 := newHTTPInteraction(t, "ix-b", "get other",
		contract.HTTPRequest{Method: "GET", Path: "/b"},
		contract.HTTPResponse{Status: 200},
	)

	ms := startMock(t, []contract.Interaction{ix1, ix2})

	// Hit /a twice, /b once
	for i := 0; i < 2; i++ {
		resp := doRequest(t, "GET", ms.URL()+"/a", nil)
		resp.Body.Close()
	}
	resp := doRequest(t, "GET", ms.URL()+"/b", nil)
	resp.Body.Close()

	counts := ms.MatchCounts()
	if counts["ix-a"] != 2 {
		t.Errorf("ix-a count = %d, want 2", counts["ix-a"])
	}
	if counts["ix-b"] != 1 {
		t.Errorf("ix-b count = %d, want 1", counts["ix-b"])
	}
}

func TestMockServer_MatchCounts_ReturnsCopy(t *testing.T) {
	ms := NewMockServer(nil)
	ms.incrementMatchCount("ix-1")

	counts := ms.MatchCounts()
	counts["ix-1"] = 999

	original := ms.MatchCounts()
	if original["ix-1"] != 1 {
		t.Error("MatchCounts should return a copy, not a reference")
	}
}

func TestMockServer_Verify(t *testing.T) {
	t.Run("all interactions matched", func(t *testing.T) {
		ix := newHTTPInteraction(t, "v-1", "get users",
			contract.HTTPRequest{Method: "GET", Path: "/users"},
			contract.HTTPResponse{Status: 200},
		)
		ms := startMock(t, []contract.Interaction{ix})
		resp := doRequest(t, "GET", ms.URL()+"/users", nil)
		resp.Body.Close()

		result := ms.Verify()
		if !result.Success {
			t.Errorf("Verify() failed: %v", result.Errors)
		}
	})

	t.Run("unmatched interaction fails verify", func(t *testing.T) {
		ix := newHTTPInteraction(t, "v-2", "get orders",
			contract.HTTPRequest{Method: "GET", Path: "/orders"},
			contract.HTTPResponse{Status: 200},
		)
		ms := startMock(t, []contract.Interaction{ix})
		// Do not make any requests

		result := ms.Verify()
		if result.Success {
			t.Error("Verify() should fail when interaction was never matched")
		}
		found := false
		for _, e := range result.Errors {
			if strings.Contains(e.Message, "never matched") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected 'never matched' error, got: %v", result.Errors)
		}
	})

	t.Run("error requests are reported", func(t *testing.T) {
		ix := newHTTPInteraction(t, "v-3", "get items",
			contract.HTTPRequest{Method: "GET", Path: "/items"},
			contract.HTTPResponse{Status: 200},
		)
		ms := startMock(t, []contract.Interaction{ix})

		// Unmatched request produces an error in the recorded request
		resp := doRequest(t, "GET", ms.URL()+"/wrong", nil)
		resp.Body.Close()

		result := ms.Verify()
		if result.Success {
			t.Error("Verify() should fail when there are error requests")
		}
	})

	t.Run("interaction without HTTP payload is skipped", func(t *testing.T) {
		ix := contract.Interaction{
			ID:          "non-http",
			Description: "async interaction",
			Payload:     contract.Payload{}, // no HTTP payload
		}
		ms := NewMockServer([]contract.Interaction{ix})
		result := ms.Verify()
		if !result.Success {
			t.Errorf("Verify() should skip non-HTTP interactions, got errors: %v", result.Errors)
		}
	})
}

func TestMockServer_Reset(t *testing.T) {
	ix := newHTTPInteraction(t, "r-1", "get items",
		contract.HTTPRequest{Method: "GET", Path: "/items"},
		contract.HTTPResponse{Status: 200, Body: map[string]any{"ok": true}},
	)
	ms := startMock(t, []contract.Interaction{ix})

	// Make a request to populate state
	resp := doRequest(t, "GET", ms.URL()+"/items", nil)
	resp.Body.Close()

	if len(ms.ReceivedRequests()) == 0 {
		t.Fatal("expected at least one recorded request before reset")
	}
	if len(ms.MatchCounts()) == 0 {
		t.Fatal("expected non-empty match counts before reset")
	}

	ms.Reset()

	if len(ms.ReceivedRequests()) != 0 {
		t.Errorf("ReceivedRequests after Reset = %d, want 0", len(ms.ReceivedRequests()))
	}
	if len(ms.MatchCounts()) != 0 {
		t.Errorf("MatchCounts after Reset = %d, want 0", len(ms.MatchCounts()))
	}
}

// ---------------------------------------------------------------------------
// findMatchingInteraction edge cases
// ---------------------------------------------------------------------------

func TestMockServer_FindMatchingInteraction_QueryParams(t *testing.T) {
	ix := newHTTPInteraction(t, "q-1", "search",
		contract.HTTPRequest{
			Method: "GET",
			Path:   "/search",
			Query:  map[string]string{"q": "hello"},
		},
		contract.HTTPResponse{Status: 200},
	)
	ms := startMock(t, []contract.Interaction{ix})

	t.Run("correct query matches", func(t *testing.T) {
		resp := doRequest(t, "GET", ms.URL()+"/search?q=hello", nil)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("wrong query value does not match", func(t *testing.T) {
		resp := doRequest(t, "GET", ms.URL()+"/search?q=world", nil)
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Error("wrong query value should not match")
		}
	})

	t.Run("missing query param does not match", func(t *testing.T) {
		resp := doRequest(t, "GET", ms.URL()+"/search", nil)
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Error("missing query param should not match")
		}
	})
}

func TestMockServer_FindMatchingInteraction_PathPlaceholder(t *testing.T) {
	ix := newHTTPInteraction(t, "p-1", "get user by id",
		contract.HTTPRequest{
			Method: "GET",
			Path:   "/users/{id}",
		},
		contract.HTTPResponse{Status: 200},
	)

	t.Run("placeholder matches any segment in lenient mode", func(t *testing.T) {
		// In lenient mode, path validation mismatch (/users/{id} vs /users/42)
		// does not block the canned response from being sent.
		ms := startMock(t, []contract.Interaction{ix}, WithMode(MockModeLenient))
		resp := doRequest(t, "GET", ms.URL()+"/users/42", nil)
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("placeholder in strict mode returns 400 due to exact path validation", func(t *testing.T) {
		// findMatchingInteraction matches via matchPath, but ValidateRequest
		// does exact path comparison, causing a validation failure in strict mode.
		ms := startMock(t, []contract.Interaction{ix})
		resp := doRequest(t, "GET", ms.URL()+"/users/42", nil)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want %d (strict, path template mismatch)", resp.StatusCode, http.StatusBadRequest)
		}
	})

	t.Run("placeholder does not match extra segments", func(t *testing.T) {
		ms := startMock(t, []contract.Interaction{ix})
		resp := doRequest(t, "GET", ms.URL()+"/users/42/orders", nil)
		defer resp.Body.Close()
		if resp.StatusCode == 200 {
			t.Error("placeholder should not match extra path segments")
		}
	})
}

func TestMockServer_FindMatchingInteraction_SkipsNonHTTP(t *testing.T) {
	// An interaction with no HTTP payload should be silently skipped
	nonHTTP := contract.Interaction{
		ID:      "async-ix",
		Payload: contract.Payload{},
	}
	httpIx := newHTTPInteraction(t, "http-ix", "get root",
		contract.HTTPRequest{Method: "GET", Path: "/"},
		contract.HTTPResponse{Status: 200},
	)

	ms := startMock(t, []contract.Interaction{nonHTTP, httpIx})
	resp := doRequest(t, "GET", ms.URL()+"/", nil)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200 (should skip non-HTTP interaction)", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// sendResponse: default Content-Type, headers with templates
// ---------------------------------------------------------------------------

func TestMockServer_SendResponse_DefaultContentType(t *testing.T) {
	ix := newHTTPInteraction(t, "ct-1", "get data",
		contract.HTTPRequest{Method: "GET", Path: "/data"},
		contract.HTTPResponse{
			Status: 200,
			Body:   map[string]any{"key": "value"},
			// No explicit Content-Type header
		},
	)
	ms := startMock(t, []contract.Interaction{ix})

	resp := doRequest(t, "GET", ms.URL()+"/data", nil)
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json (default)", ct)
	}
}

func TestMockServer_SendResponse_NilBody(t *testing.T) {
	ix := newHTTPInteraction(t, "nb-1", "no body",
		contract.HTTPRequest{Method: "DELETE", Path: "/resource"},
		contract.HTTPResponse{
			Status: 204,
			// nil Body
		},
	)
	ms := startMock(t, []contract.Interaction{ix})

	resp := doRequest(t, "DELETE", ms.URL()+"/resource", nil)
	body := readBody(t, resp)

	if resp.StatusCode != 204 {
		t.Errorf("status = %d, want 204", resp.StatusCode)
	}
	if body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
}

func TestMockServer_SendResponse_TemplateInHeaders(t *testing.T) {
	ix := newHTTPInteraction(t, "th-1", "get with template headers",
		contract.HTTPRequest{Method: "GET", Path: "/templated"},
		contract.HTTPResponse{
			Status: 200,
			Headers: map[string]string{
				"X-Request-Id": "${uuid()}",
				"X-Timestamp":  "${now()}",
			},
		},
	)
	ms := startMock(t, []contract.Interaction{ix})

	resp := doRequest(t, "GET", ms.URL()+"/templated", nil)
	defer resp.Body.Close()

	xid := resp.Header.Get("X-Request-Id")
	if xid == "" || xid == "${uuid()}" {
		t.Errorf("X-Request-Id not processed: %q", xid)
	}
	xts := resp.Header.Get("X-Timestamp")
	if xts == "" || xts == "${now()}" {
		t.Errorf("X-Timestamp not processed: %q", xts)
	}
}

func TestMockServer_SendResponse_TemplateInBody(t *testing.T) {
	ix := newHTTPInteraction(t, "tb-1", "body template",
		contract.HTTPRequest{Method: "GET", Path: "/body-tmpl"},
		contract.HTTPResponse{
			Status: 200,
			Body: map[string]any{
				"created_at": "${now()}",
				"trace_id":   "${uuid()}",
			},
		},
	)
	ms := startMock(t, []contract.Interaction{ix})

	resp := doRequest(t, "GET", ms.URL()+"/body-tmpl", nil)
	defer resp.Body.Close()
	body := readBody(t, resp)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if parsed["created_at"] == "${now()}" {
		t.Error("body template ${now()} not replaced")
	}
	if parsed["trace_id"] == "${uuid()}" {
		t.Error("body template ${uuid()} not replaced")
	}
}

// ---------------------------------------------------------------------------
// formatValidationErrors
// ---------------------------------------------------------------------------

func TestFormatValidationErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "empty",
			input: nil,
			want:  "",
		},
		{
			name:  "single error",
			input: []string{"field missing"},
			want:  "field missing",
		},
		{
			name:  "multiple errors joined with semicolons",
			input: []string{"error one", "error two", "error three"},
			want:  "error one; error two; error three",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build []validator.ValidationError from input strings
			var errs []validator.ValidationError
			for _, msg := range tt.input {
				errs = append(errs, validator.ValidationError{Message: msg})
			}
			got := formatValidationErrors(errs)
			if got != tt.want {
				t.Errorf("formatValidationErrors() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Multiple interactions: first-match wins
// ---------------------------------------------------------------------------

func TestMockServer_FirstMatchWins(t *testing.T) {
	ix1 := newHTTPInteraction(t, "first", "first handler",
		contract.HTTPRequest{Method: "GET", Path: "/dup"},
		contract.HTTPResponse{
			Status: 200,
			Body:   map[string]any{"which": "first"},
		},
	)
	ix2 := newHTTPInteraction(t, "second", "second handler",
		contract.HTTPRequest{Method: "GET", Path: "/dup"},
		contract.HTTPResponse{
			Status: 200,
			Body:   map[string]any{"which": "second"},
		},
	)

	ms := startMock(t, []contract.Interaction{ix1, ix2})
	resp := doRequest(t, "GET", ms.URL()+"/dup", nil)
	defer resp.Body.Close()
	body := readBody(t, resp)

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("body parse error: %v", err)
	}
	if parsed["which"] != "first" {
		t.Errorf("expected first interaction to win, got %v", parsed["which"])
	}

	counts := ms.MatchCounts()
	if counts["first"] != 1 {
		t.Errorf("first match count = %d, want 1", counts["first"])
	}
	if counts["second"] != 0 {
		t.Errorf("second match count = %d, want 0", counts["second"])
	}
}

// ---------------------------------------------------------------------------
// RecordedRequest captures headers
// ---------------------------------------------------------------------------

func TestMockServer_RecordedRequestHeaders(t *testing.T) {
	ix := newHTTPInteraction(t, "hdr-1", "get with headers",
		contract.HTTPRequest{Method: "GET", Path: "/hdr"},
		contract.HTTPResponse{Status: 200},
	)
	ms := startMock(t, []contract.Interaction{ix})

	req, err := http.NewRequestWithContext(context.Background(), "GET", ms.URL()+"/hdr", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Custom", "custom-value")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	reqs := ms.ReceivedRequests()
	if len(reqs) != 1 {
		t.Fatalf("ReceivedRequests len = %d, want 1", len(reqs))
	}
	if reqs[0].Headers["X-Custom"] != "custom-value" {
		t.Errorf("recorded header X-Custom = %q, want %q", reqs[0].Headers["X-Custom"], "custom-value")
	}
}
