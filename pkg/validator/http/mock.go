package httpvalidator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/validator"
)

// MockMode controls how the mock server handles unmatched requests.
type MockMode string

const (
	// MockModeStrict rejects requests not matching any interaction.
	MockModeStrict MockMode = "strict"
	// MockModeLenient logs a warning and returns 404 for unmatched requests.
	MockModeLenient MockMode = "lenient"
	// MockModeRecord captures all requests for later contract generation.
	MockModeRecord MockMode = "record"
)

// MockServer is an HTTP mock server that responds based on contract interactions.
type MockServer struct {
	interactions []contract.Interaction
	mode         MockMode
	server       *http.Server
	listener     net.Listener
	validator    *Validator

	mu               sync.RWMutex
	receivedRequests []RecordedRequest
	matchedRequests  map[string]int // interaction ID -> match count
}

// RecordedRequest represents a captured HTTP request.
type RecordedRequest struct {
	Method  string
	Path    string
	Headers map[string]string
	Body    []byte
	Matched bool
	Error   string
}

// MockServerOption configures a MockServer.
type MockServerOption func(*MockServer)

// WithMode sets the mock server mode.
func WithMode(mode MockMode) MockServerOption {
	return func(s *MockServer) {
		s.mode = mode
	}
}

// NewMockServer creates a new mock server with the given interactions.
func NewMockServer(interactions []contract.Interaction, opts ...MockServerOption) *MockServer {
	s := &MockServer{
		interactions:    interactions,
		mode:            MockModeStrict,
		validator:       NewValidator(),
		matchedRequests: make(map[string]int),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the mock server on an ephemeral port.
func (s *MockServer) Start() error {
	return s.StartContext(context.Background())
}

// StartContext starts the mock server on an ephemeral port with context.
func (s *MockServer) StartContext(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	s.server = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			// Log error but don't panic
			fmt.Printf("mock server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the mock server.
func (s *MockServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// URL returns the base URL of the mock server.
func (s *MockServer) URL() string {
	if s.listener == nil {
		return ""
	}
	return fmt.Sprintf("http://%s", s.listener.Addr().String())
}

// ServeHTTP handles incoming HTTP requests.
func (s *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Read and buffer the request body
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	// Record the request
	recorded := RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.Path,
		Headers: make(map[string]string),
		Body:    bodyBytes,
	}
	for k, v := range r.Header {
		if len(v) > 0 {
			recorded.Headers[k] = v[0]
		}
	}

	// Find matching interaction
	interaction, err := s.findMatchingInteraction(r)
	if err != nil {
		recorded.Error = err.Error()
		s.recordRequest(&recorded)

		switch s.mode {
		case MockModeStrict:
			http.Error(w, err.Error(), http.StatusBadRequest)
		case MockModeLenient, MockModeRecord:
			http.Error(w, "no matching interaction", http.StatusNotFound)
		}
		return
	}

	// Validate the request against the contract
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) // Reset body
	result := s.validator.ValidateRequest(interaction, r)
	if !result.Success {
		recorded.Error = fmt.Sprintf("request validation failed: %v", formatValidationErrors(result.Errors))
		s.recordRequest(&recorded)

		if s.mode == MockModeStrict {
			http.Error(w, recorded.Error, http.StatusBadRequest)
			return
		}
	}

	// Record successful match
	recorded.Matched = true
	s.recordRequest(&recorded)
	s.incrementMatchCount(interaction.ID)

	// Build and send response
	s.sendResponse(w, interaction)
}

// findMatchingInteraction finds an interaction that matches the request.
func (s *MockServer) findMatchingInteraction(r *http.Request) (*contract.Interaction, error) {
	for i := range s.interactions {
		interaction := &s.interactions[i]
		if interaction.Payload.HTTP == nil {
			continue
		}

		expected := interaction.Payload.HTTP.Request

		// Check method
		if r.Method != expected.Method {
			continue
		}

		// Check path (support path templates with regex)
		if !matchPath(expected.Path, r.URL.Path) {
			continue
		}

		// Check query parameters
		if !matchQueryParams(expected.Query, r.URL.Query()) {
			continue
		}

		return interaction, nil
	}

	return nil, fmt.Errorf("no interaction matches %s %s", r.Method, r.URL.Path)
}

// matchPath checks if a request path matches an expected path (with optional placeholders).
func matchPath(expected, actual string) bool {
	// Exact match
	if expected == actual {
		return true
	}

	// Check for path parameters like /users/{id}
	if strings.Contains(expected, "{") {
		pattern := regexp.QuoteMeta(expected)
		pattern = regexp.MustCompile(`\\{[^}]+\\}`).ReplaceAllString(pattern, `[^/]+`)
		pattern = "^" + pattern + "$"

		matched, err := regexp.MatchString(pattern, actual)
		if err != nil {
			// Invalid regex pattern - fall back to exact match which will fail
			return false
		}
		return matched
	}

	return false
}

// matchQueryParams checks if actual query params satisfy expected ones.
func matchQueryParams(expected map[string]string, actual map[string][]string) bool {
	for key, expectedValue := range expected {
		actualValues, ok := actual[key]
		if !ok || len(actualValues) == 0 {
			return false
		}
		// Check if any of the actual values match
		found := false
		for _, v := range actualValues {
			if v == expectedValue {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// sendResponse sends the canned response for an interaction.
func (s *MockServer) sendResponse(w http.ResponseWriter, interaction *contract.Interaction) {
	response := interaction.Payload.HTTP.Response

	// Set headers
	for key, value := range response.Headers {
		// Process template substitutions
		value = s.processTemplate(value, interaction)
		w.Header().Set(key, value)
	}

	// Set default Content-Type if not specified
	if w.Header().Get("Content-Type") == "" && response.Body != nil {
		w.Header().Set("Content-Type", "application/json")
	}

	// Write status code
	w.WriteHeader(response.Status)

	// Write body
	if response.Body != nil {
		body := s.processBodyTemplate(response.Body, interaction)
		if bodyBytes, err := json.Marshal(body); err == nil {
			if _, err := w.Write(bodyBytes); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

// processTemplate processes template substitutions in a string value.
func (s *MockServer) processTemplate(value string, _ *contract.Interaction) string {
	// Replace ${now()} with current timestamp
	if strings.Contains(value, "${now()}") {
		// Implementation would use time.Now().Format(time.RFC3339)
		value = strings.ReplaceAll(value, "${now()}", "2024-01-15T10:30:00Z")
	}

	// Replace ${uuid()} with a new UUID
	if strings.Contains(value, "${uuid()}") {
		// Implementation would use uuid.New().String()
		value = strings.ReplaceAll(value, "${uuid()}", "550e8400-e29b-41d4-a716-446655440000")
	}

	return value
}

// processBodyTemplate processes template substitutions in a body value.
func (s *MockServer) processBodyTemplate(body any, interaction *contract.Interaction) any {
	switch v := body.(type) {
	case map[string]any:
		result := make(map[string]any)
		for key, val := range v {
			result[key] = s.processBodyTemplate(val, interaction)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = s.processBodyTemplate(val, interaction)
		}
		return result
	case string:
		return s.processTemplate(v, interaction)
	default:
		return body
	}
}

// recordRequest records a request for later analysis.
func (s *MockServer) recordRequest(req *RecordedRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedRequests = append(s.receivedRequests, *req)
}

// incrementMatchCount increments the match counter for an interaction.
func (s *MockServer) incrementMatchCount(interactionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.matchedRequests[interactionID]++
}

// ReceivedRequests returns all recorded requests.
func (s *MockServer) ReceivedRequests() []RecordedRequest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]RecordedRequest, len(s.receivedRequests))
	copy(result, s.receivedRequests)
	return result
}

// MatchCounts returns the number of times each interaction was matched.
func (s *MockServer) MatchCounts() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]int)
	for k, v := range s.matchedRequests {
		result[k] = v
	}
	return result
}

// Verify checks that all interactions were matched at least once.
func (s *MockServer) Verify() validator.ValidationResult {
	result := validator.NewSuccessResult()

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, interaction := range s.interactions {
		if interaction.Payload.HTTP == nil {
			continue
		}

		count := s.matchedRequests[interaction.ID]
		if count == 0 {
			result.AddError(&validator.ValidationError{
				Path:    interaction.ID,
				Message: fmt.Sprintf("interaction %q was never matched", interaction.Description),
			})
		}
	}

	// Check for validation errors in recorded requests
	for _, req := range s.receivedRequests {
		if req.Error != "" {
			result.AddError(&validator.ValidationError{
				Path:    req.Path,
				Message: fmt.Sprintf("%s %s: %s", req.Method, req.Path, req.Error),
			})
		}
	}

	return result
}

// Reset clears all recorded state.
func (s *MockServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.receivedRequests = nil
	s.matchedRequests = make(map[string]int)
}

// formatValidationErrors formats validation errors for display.
func formatValidationErrors(errors []validator.ValidationError) string {
	if len(errors) == 0 {
		return ""
	}

	messages := make([]string, 0, len(errors))
	for _, err := range errors {
		messages = append(messages, err.Message)
	}
	return strings.Join(messages, "; ")
}
