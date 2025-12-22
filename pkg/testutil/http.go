package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// HTTPRequest is a builder for creating test HTTP requests.
type HTTPRequest struct {
	method  string
	path    string
	headers map[string]string
	body    any
}

// NewHTTPRequest creates a new HTTP request builder.
func NewHTTPRequest(method, path string) *HTTPRequest {
	return &HTTPRequest{
		method:  method,
		path:    path,
		headers: make(map[string]string),
	}
}

// WithHeader adds a header to the request.
func (r *HTTPRequest) WithHeader(key, value string) *HTTPRequest {
	r.headers[key] = value
	return r
}

// WithJSONBody sets the request body as JSON.
func (r *HTTPRequest) WithJSONBody(body any) *HTTPRequest {
	r.body = body
	r.headers["Content-Type"] = "application/json"
	return r
}

// Build creates an http.Request for testing.
func (r *HTTPRequest) Build(t *testing.T) *http.Request {
	t.Helper()

	var bodyReader io.Reader
	if r.body != nil {
		bodyBytes, err := json.Marshal(r.body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(r.method, r.path, bodyReader)
	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	return req
}

// HTTPResponse provides helper methods for validating HTTP responses.
type HTTPResponse struct {
	*httptest.ResponseRecorder
	t *testing.T
}

// NewHTTPResponse creates a new HTTP response recorder.
func NewHTTPResponse(t *testing.T) *HTTPResponse {
	return &HTTPResponse{
		ResponseRecorder: httptest.NewRecorder(),
		t:                t,
	}
}

// ExpectStatus asserts the response status code.
func (r *HTTPResponse) ExpectStatus(expected int) *HTTPResponse {
	r.t.Helper()
	if r.Code != expected {
		r.t.Errorf("expected status %d, got %d", expected, r.Code)
	}
	return r
}

// ExpectHeader asserts a response header value.
func (r *HTTPResponse) ExpectHeader(key, expected string) *HTTPResponse {
	r.t.Helper()
	actual := r.Header().Get(key)
	if actual != expected {
		r.t.Errorf("expected header %s to be %q, got %q", key, expected, actual)
	}
	return r
}

// ExpectJSONBody asserts the response body matches the expected JSON.
func (r *HTTPResponse) ExpectJSONBody(expected any) *HTTPResponse {
	r.t.Helper()

	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		r.t.Fatalf("failed to marshal expected body: %v", err)
	}

	var expectedJSON, actualJSON any
	if err := json.Unmarshal(expectedBytes, &expectedJSON); err != nil {
		r.t.Fatalf("failed to unmarshal expected: %v", err)
	}
	if err := json.Unmarshal(r.Body.Bytes(), &actualJSON); err != nil {
		r.t.Fatalf("failed to unmarshal actual body: %v", err)
	}

	// Re-marshal for consistent comparison
	expectedNorm, _ := json.Marshal(expectedJSON)
	actualNorm, _ := json.Marshal(actualJSON)

	if !bytes.Equal(expectedNorm, actualNorm) {
		r.t.Errorf("body mismatch:\nexpected: %s\nactual:   %s", expectedNorm, actualNorm)
	}

	return r
}

// JSONBody returns the response body parsed as JSON.
func (r *HTTPResponse) JSONBody() (any, error) {
	var result any
	if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
		return nil, err
	}
	return result, nil
}

// MustJSONBody returns the response body parsed as JSON, failing the test on error.
func (r *HTTPResponse) MustJSONBody() any {
	r.t.Helper()
	result, err := r.JSONBody()
	if err != nil {
		r.t.Fatalf("failed to parse JSON body: %v", err)
	}
	return result
}

// ServeHTTP is a convenience function for testing HTTP handlers.
func ServeHTTP(t *testing.T, handler http.Handler, req *http.Request) *HTTPResponse {
	t.Helper()
	resp := NewHTTPResponse(t)
	handler.ServeHTTP(resp, req)
	return resp
}

// JSONHandler creates a simple HTTP handler that returns JSON.
func JSONHandler(status int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body)
		}
	}
}

// ErrorHandler creates an HTTP handler that returns an error response.
func ErrorHandler(status int, message string) http.HandlerFunc {
	return JSONHandler(status, map[string]string{"error": message})
}

// EchoHandler creates an HTTP handler that echoes the request body.
func EchoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, r.Body)
	}
}
