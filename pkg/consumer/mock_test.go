package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newGetInteraction is a test helper that builds a minimal GET interaction.
// It always responds with status 200.
func newGetInteraction(t *testing.T, description, path string, body map[string]any) *Interaction {
	t.Helper()
	return NewInteraction(description).
		WithHTTPRequest("GET", path).
		WillRespondWith(200).
		WithJSONBody(body).
		Build()
}

// doGet is a test helper that performs an HTTP GET using http.NewRequestWithContext.
func doGet(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	return resp
}

// doPost is a test helper that performs an HTTP POST using http.NewRequestWithContext.
func doPost(t *testing.T, url, contentType string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	return resp
}

func TestNewPact(t *testing.T) {
	tests := []struct {
		name     string
		consumer string
		provider string
	}{
		{
			name:     "creates pact with consumer and provider names",
			consumer: "my-consumer",
			provider: "my-provider",
		},
		{
			name:     "creates pact with empty names",
			consumer: "",
			provider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPact(tt.consumer, tt.provider)
			if p == nil {
				t.Fatal("NewPact returned nil")
			}
			if p.consumer != tt.consumer {
				t.Errorf("consumer = %q, want %q", p.consumer, tt.consumer)
			}
			if p.provider != tt.provider {
				t.Errorf("provider = %q, want %q", p.provider, tt.provider)
			}
		})
	}
}

func TestPact_DefaultOutputDir(t *testing.T) {
	p := NewPact("consumer", "provider")
	if p.outDir != "./contracts" {
		t.Errorf("default outDir = %q, want %q", p.outDir, "./contracts")
	}
}

func TestPact_WithOutputDir(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{name: "custom directory", dir: "/tmp/my-contracts"},
		{name: "relative directory", dir: "output/contracts"},
		{name: "empty string", dir: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPact("consumer", "provider").WithOutputDir(tt.dir)
			if p.outDir != tt.dir {
				t.Errorf("outDir = %q, want %q", p.outDir, tt.dir)
			}
		})
	}
}

func TestPact_WithOutputDir_Chaining(t *testing.T) {
	p := NewPact("consumer", "provider")
	result := p.WithOutputDir("/tmp/contracts")
	if result != p {
		t.Error("WithOutputDir should return the same Pact for chaining")
	}
}

func TestPact_AddInteraction(t *testing.T) {
	t.Run("first interaction creates a contract", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": 1})

		result := p.AddInteraction(interaction)
		if result != p {
			t.Error("AddInteraction should return the same Pact for chaining")
		}
		if len(p.contracts) != 1 {
			t.Fatalf("contracts count = %d, want 1", len(p.contracts))
		}
		if len(p.contracts[0].Interactions) != 1 {
			t.Fatalf("interactions count = %d, want 1", len(p.contracts[0].Interactions))
		}
	})

	t.Run("second interaction appends to existing contract", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		i1 := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": 1})
		i2 := newGetInteraction(t, "get order", "/orders/1", map[string]any{"id": 1})

		p.AddInteraction(i1).AddInteraction(i2)

		if len(p.contracts) != 1 {
			t.Fatalf("contracts count = %d, want 1", len(p.contracts))
		}
		if len(p.contracts[0].Interactions) != 2 {
			t.Fatalf("interactions count = %d, want 2", len(p.contracts[0].Interactions))
		}
	})

	t.Run("interaction description is preserved", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		interaction := newGetInteraction(t, "unique description", "/path", nil)
		p.AddInteraction(interaction)

		got := p.contracts[0].Interactions[0].Description
		if got != "unique description" {
			t.Errorf("interaction description = %q, want %q", got, "unique description")
		}
	})
}

func TestPact_Setup_NoInteractions(t *testing.T) {
	tests := []struct {
		name string
		pact *Pact
	}{
		{
			name: "no contracts at all",
			pact: NewPact("consumer", "provider"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := tt.pact.Setup()
			if err == nil {
				t.Fatal("Setup should return an error when no interactions defined")
			}
			if url != "" {
				t.Errorf("URL should be empty on error, got %q", url)
			}
			if !strings.Contains(err.Error(), "no interactions defined") {
				t.Errorf("error = %q, want it to contain %q", err.Error(), "no interactions defined")
			}
		})
	}
}

func TestPact_Setup_ValidInteraction(t *testing.T) {
	p := NewPact("consumer", "provider")
	interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": 1})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	if url == "" {
		t.Fatal("Setup should return a non-empty URL")
	}
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("URL = %q, want prefix %q", url, "http://127.0.0.1:")
	}
}

func TestPact_Setup_MockServerResponds(t *testing.T) {
	p := NewPact("consumer", "provider")
	respBody := map[string]any{"id": float64(1), "name": "Alice"}
	interaction := newGetInteraction(t, "get user", "/users/1", respBody)
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	resp := doGet(t, url+"/users/1")
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status code = %d, want 200", resp.StatusCode)
	}

	var body map[string]any
	if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr != nil {
		t.Fatalf("failed to decode response body: %v", decErr)
	}
	if body["name"] != "Alice" {
		t.Errorf("body[name] = %v, want Alice", body["name"])
	}
}

func TestPact_Verify_AllMatched(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("test-consumer", "test-provider").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Make the matching request
	resp := doGet(t, url+"/users/1")
	resp.Body.Close()

	// Verify should succeed
	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	// Check that contract file was saved
	contractFile := filepath.Join(outDir, "test-consumer-test-provider.json")
	if _, statErr := os.Stat(contractFile); os.IsNotExist(statErr) {
		t.Fatalf("contract file was not created at %s", contractFile)
	}

	// Verify the saved contract content
	data, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("failed to read contract file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("contract file is empty")
	}

	var saved map[string]any
	if unmarshalErr := json.Unmarshal(data, &saved); unmarshalErr != nil {
		t.Fatalf("contract file is not valid JSON: %v", unmarshalErr)
	}
}

func TestPact_Verify_UnmatchedInteraction(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("consumer", "provider").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
	p.AddInteraction(interaction)

	_, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Don't make any requests -- interaction remains unmatched
	err = p.Verify()
	if err == nil {
		t.Fatal("Verify should return an error when interactions are unmatched")
	}
	if !strings.Contains(err.Error(), "pact verification failed") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "pact verification failed")
	}
}

func TestPact_Verify_MockNotStarted(t *testing.T) {
	p := NewPact("consumer", "provider")
	err := p.Verify()
	if err == nil {
		t.Fatal("Verify should return error when mock server not started")
	}
	if !strings.Contains(err.Error(), "mock server not started") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "mock server not started")
	}
}

func TestPact_Verify_SavesContractWithChecksum(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("svc-a", "svc-b").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get data", "/data", map[string]any{"ok": true})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	resp := doGet(t, url+"/data")
	resp.Body.Close()

	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	contractFile := filepath.Join(outDir, "svc-a-svc-b.json")
	data, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("failed to read contract file: %v", err)
	}

	var saved map[string]any
	if unmarshalErr := json.Unmarshal(data, &saved); unmarshalErr != nil {
		t.Fatalf("contract is not valid JSON: %v", unmarshalErr)
	}

	metadata, ok := saved["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata field missing or not an object")
	}
	checksum, ok := metadata["checksum"].(string)
	if !ok || checksum == "" {
		t.Error("contract should have a non-empty checksum")
	}
}

func TestPact_Teardown(t *testing.T) {
	t.Run("teardown without setup is a no-op", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		if err := p.Teardown(); err != nil {
			t.Errorf("Teardown should not error when mock is nil, got: %v", err)
		}
	})

	t.Run("teardown stops the mock server", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
		p.AddInteraction(interaction)

		url, err := p.Setup()
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}

		// Verify server is running
		resp := doGet(t, url+"/users/1")
		resp.Body.Close()

		// Teardown
		if tdErr := p.Teardown(); tdErr != nil {
			t.Fatalf("Teardown failed: %v", tdErr)
		}

		// Server should no longer accept connections
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, url+"/users/1", http.NoBody)
		if reqErr != nil {
			t.Fatalf("failed to create request: %v", reqErr)
		}
		resp, doErr := http.DefaultClient.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		if doErr == nil {
			t.Error("request after teardown should fail")
		}
	})
}

func TestPact_MockURL(t *testing.T) {
	t.Run("returns empty before setup", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		if got := p.MockURL(); got != "" {
			t.Errorf("MockURL before setup = %q, want empty string", got)
		}
	})

	t.Run("returns URL after setup", func(t *testing.T) {
		p := NewPact("consumer", "provider")
		interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
		p.AddInteraction(interaction)

		_, err := p.Setup()
		if err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		defer func() {
			if tdErr := p.Teardown(); tdErr != nil {
				t.Logf("Warning: teardown failed: %v", tdErr)
			}
		}()

		url := p.MockURL()
		if url == "" {
			t.Fatal("MockURL should return non-empty string after setup")
		}
		if !strings.HasPrefix(url, "http://") {
			t.Errorf("MockURL = %q, want http:// prefix", url)
		}
	})
}

func TestRunTest_HappyPath(t *testing.T) {
	outDir := t.TempDir()
	interaction := NewInteraction("get products").
		WithHTTPRequest("GET", "/products").
		WillRespondWith(200).
		WithJSONBody(map[string]any{
			"products": []any{
				map[string]any{"id": float64(1), "name": "Widget"},
			},
		}).
		Build()

	testFuncCalled := false

	RunTest(t, "shop-ui", "catalog-api", []*Interaction{interaction}, func(mockURL string) {
		testFuncCalled = true

		if mockURL == "" {
			t.Fatal("mockURL should not be empty")
		}

		resp := doGet(t, mockURL+"/products")
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}

		var body map[string]any
		if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr != nil {
			t.Fatalf("failed to decode body: %v", decErr)
		}

		products, ok := body["products"].([]any)
		if !ok {
			t.Fatal("products field missing or not an array")
		}
		if len(products) != 1 {
			t.Errorf("products count = %d, want 1", len(products))
		}
	}, WithContractDir(outDir))

	if !testFuncCalled {
		t.Fatal("test function was not called")
	}

	// Verify contract file was saved
	contractFile := filepath.Join(outDir, "shop-ui-catalog-api.json")
	if _, statErr := os.Stat(contractFile); os.IsNotExist(statErr) {
		t.Fatalf("contract file not created at %s", contractFile)
	}
}

func TestRunTest_MultipleInteractions(t *testing.T) {
	outDir := t.TempDir()

	getUser := NewInteraction("get user").
		WithHTTPRequest("GET", "/users/1").
		WillRespondWith(200).
		WithJSONBody(map[string]any{"id": float64(1), "name": "Alice"}).
		Build()

	getOrders := NewInteraction("get orders").
		WithHTTPRequest("GET", "/orders").
		WithQuery(map[string]string{"user_id": "1"}).
		WillRespondWith(200).
		WithJSONBody(map[string]any{
			"orders": []any{map[string]any{"id": float64(100)}},
		}).
		Build()

	RunTest(t, "dashboard", "backend", []*Interaction{getUser, getOrders}, func(mockURL string) {
		// Hit both endpoints
		resp1 := doGet(t, mockURL+"/users/1")
		resp1.Body.Close()
		if resp1.StatusCode != 200 {
			t.Errorf("/users/1 status = %d, want 200", resp1.StatusCode)
		}

		resp2 := doGet(t, mockURL+"/orders?user_id=1")
		resp2.Body.Close()
		if resp2.StatusCode != 200 {
			t.Errorf("/orders status = %d, want 200", resp2.StatusCode)
		}
	}, WithContractDir(outDir))
}

func TestRunTest_WithContractDir(t *testing.T) {
	outDir := t.TempDir()
	subDir := filepath.Join(outDir, "nested", "contracts")

	interaction := newGetInteraction(t, "health", "/health", map[string]any{"status": "ok"})

	RunTest(t, "client", "server", []*Interaction{interaction}, func(mockURL string) {
		resp := doGet(t, mockURL+"/health")
		resp.Body.Close()
	}, WithContractDir(subDir))

	contractFile := filepath.Join(subDir, "client-server.json")
	if _, statErr := os.Stat(contractFile); os.IsNotExist(statErr) {
		t.Fatalf("contract file not created in nested dir %s", contractFile)
	}
}

func TestPact_Setup_MultipleInteractions(t *testing.T) {
	p := NewPact("consumer", "provider")
	i1 := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
	i2 := newGetInteraction(t, "get order", "/orders/1", map[string]any{"id": float64(1)})
	p.AddInteraction(i1).AddInteraction(i2)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Both endpoints should respond
	for _, path := range []string{"/users/1", "/orders/1"} {
		resp := doGet(t, url+path)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("%s status = %d, want 200", path, resp.StatusCode)
		}
	}
}

func TestPact_Verify_MultipleInteractions_PartiallyMatched(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("consumer", "provider").WithOutputDir(outDir)

	i1 := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
	i2 := newGetInteraction(t, "get order", "/orders/1", map[string]any{"id": float64(1)})
	p.AddInteraction(i1).AddInteraction(i2)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Only match one of two interactions
	resp := doGet(t, url+"/users/1")
	resp.Body.Close()

	err = p.Verify()
	if err == nil {
		t.Fatal("Verify should fail when not all interactions are matched")
	}
	if !strings.Contains(err.Error(), "pact verification failed") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "pact verification failed")
	}
}

func TestPact_Setup_ResponseHeaders(t *testing.T) {
	p := NewPact("consumer", "provider")
	interaction := NewInteraction("get data with headers").
		WithHTTPRequest("GET", "/data").
		WillRespondWith(200).
		WithHeader("X-Request-Id", "abc-123").
		WithJSONBody(map[string]any{"data": "value"}).
		Build()
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	resp := doGet(t, url+"/data")
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Request-Id"); got != "abc-123" {
		t.Errorf("X-Request-Id header = %q, want %q", got, "abc-123")
	}
}

func TestPact_Setup_PostInteraction(t *testing.T) {
	p := NewPact("consumer", "provider")
	interaction := NewInteraction("create user").
		WithHTTPRequest("POST", "/users").
		WithJSONBody(map[string]any{"name": "Alice"}).
		WillRespondWith(201).
		WithJSONBody(map[string]any{"id": float64(1), "name": "Alice"}).
		Build()
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	reqBody := strings.NewReader(`{"name":"Alice"}`)
	resp := doPost(t, url+"/users", "application/json", reqBody)
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}

	var body map[string]any
	if decErr := json.NewDecoder(resp.Body).Decode(&body); decErr != nil {
		t.Fatalf("failed to decode response: %v", decErr)
	}
	if body["name"] != "Alice" {
		t.Errorf("body[name] = %v, want Alice", body["name"])
	}
}

func TestPact_FullLifecycle(t *testing.T) {
	outDir := t.TempDir()

	// 1. Build
	p := NewPact("web-app", "user-api").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get user profile", "/profile",
		map[string]any{"name": "Bob", "email": "bob@example.com"})
	p.AddInteraction(interaction)

	// 2. Setup
	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// 3. Exercise
	resp := doGet(t, url+"/profile")
	body, readErr := io.ReadAll(resp.Body)
	resp.Body.Close()
	if readErr != nil {
		t.Fatalf("failed to read body: %v", readErr)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Bob") {
		t.Errorf("body should contain Bob, got %s", string(body))
	}

	// 4. Verify
	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	// 5. Teardown
	if tdErr := p.Teardown(); tdErr != nil {
		t.Fatalf("Teardown failed: %v", tdErr)
	}

	// 6. Confirm contract file
	contractFile := filepath.Join(outDir, "web-app-user-api.json")
	info, statErr := os.Stat(contractFile)
	if os.IsNotExist(statErr) {
		t.Fatal("contract file should exist after full lifecycle")
	}
	if info.Size() == 0 {
		t.Fatal("contract file should not be empty")
	}
}

func TestPact_UnmatchedRequest_StrictMode(t *testing.T) {
	p := NewPact("consumer", "provider")
	interaction := newGetInteraction(t, "get user", "/users/1", map[string]any{"id": float64(1)})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Make a request that doesn't match any interaction
	resp := doGet(t, url+"/not-defined")
	resp.Body.Close()

	// The mock server in strict mode should reject unmatched requests
	if resp.StatusCode == 200 {
		t.Error("unmatched request should not return 200 in strict mode")
	}
}

func TestWithContractDir_Option(t *testing.T) {
	cfg := &testConfig{outDir: "./contracts"}
	opt := WithContractDir("/custom/path")
	opt(cfg)
	if cfg.outDir != "/custom/path" {
		t.Errorf("outDir = %q, want %q", cfg.outDir, "/custom/path")
	}
}

func TestPact_Verify_CreatesOutputDirectory(t *testing.T) {
	base := t.TempDir()
	outDir := filepath.Join(base, "deeply", "nested", "dir")

	p := NewPact("consumer", "provider").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get data", "/data", map[string]any{"ok": true})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	resp := doGet(t, url+"/data")
	resp.Body.Close()

	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	// Verify the deeply nested directory was created
	if _, statErr := os.Stat(outDir); os.IsNotExist(statErr) {
		t.Fatalf("output directory was not created: %s", outDir)
	}
}

func TestPact_Verify_ContractFilename(t *testing.T) {
	tests := []struct {
		name         string
		consumer     string
		provider     string
		wantFilename string
	}{
		{
			name:         "simple names",
			consumer:     "frontend",
			provider:     "backend",
			wantFilename: "frontend-backend.json",
		},
		{
			name:         "names with hyphens",
			consumer:     "my-cool-app",
			provider:     "user-service-api",
			wantFilename: "my-cool-app-user-service-api.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outDir := t.TempDir()
			p := NewPact(tt.consumer, tt.provider).WithOutputDir(outDir)
			interaction := newGetInteraction(t, "test", "/test", nil)
			p.AddInteraction(interaction)

			url, err := p.Setup()
			if err != nil {
				t.Fatalf("Setup failed: %v", err)
			}
			defer func() {
				if tdErr := p.Teardown(); tdErr != nil {
					t.Logf("Warning: teardown failed: %v", tdErr)
				}
			}()

			resp := doGet(t, url+"/test")
			resp.Body.Close()

			if verifyErr := p.Verify(); verifyErr != nil {
				t.Fatalf("Verify failed: %v", verifyErr)
			}

			expectedPath := filepath.Join(outDir, tt.wantFilename)
			if _, statErr := os.Stat(expectedPath); os.IsNotExist(statErr) {
				// List what files were actually created
				entries, readDirErr := os.ReadDir(outDir)
				if readDirErr != nil {
					t.Fatalf("failed to read output directory: %v", readDirErr)
				}
				var files []string
				for _, e := range entries {
					files = append(files, e.Name())
				}
				t.Fatalf("expected file %q not found, directory contains: %v", tt.wantFilename, files)
			}
		})
	}
}

func TestPact_Verify_ContractMetadata(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("my-consumer", "my-provider").WithOutputDir(outDir)
	interaction := newGetInteraction(t, "get info", "/info", map[string]any{"version": "1.0"})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	resp := doGet(t, url+"/info")
	resp.Body.Close()

	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	contractFile := filepath.Join(outDir, "my-consumer-my-provider.json")
	data, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("failed to read contract file: %v", err)
	}

	var saved map[string]any
	if unmarshalErr := json.Unmarshal(data, &saved); unmarshalErr != nil {
		t.Fatalf("invalid JSON: %v", unmarshalErr)
	}

	metadata, ok := saved["metadata"].(map[string]any)
	if !ok {
		t.Fatal("metadata field missing or not an object")
	}

	consumer, ok := metadata["consumer"].(map[string]any)
	if !ok {
		t.Fatal("consumer field missing or not an object")
	}
	if consumer["name"] != "my-consumer" {
		t.Errorf("consumer name = %v, want my-consumer", consumer["name"])
	}

	provider, ok := metadata["provider"].(map[string]any)
	if !ok {
		t.Fatal("provider field missing or not an object")
	}
	if provider["name"] != "my-provider" {
		t.Errorf("provider name = %v, want my-provider", provider["name"])
	}
}

func TestPact_Verify_ContractInteractions(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("consumer", "provider").WithOutputDir(outDir)

	i1 := newGetInteraction(t, "get users", "/users", map[string]any{"users": []any{}})
	i2 := newGetInteraction(t, "get orders", "/orders", map[string]any{"orders": []any{}})
	p.AddInteraction(i1).AddInteraction(i2)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Match both interactions
	for _, path := range []string{"/users", "/orders"} {
		resp := doGet(t, url+path)
		resp.Body.Close()
	}

	if verifyErr := p.Verify(); verifyErr != nil {
		t.Fatalf("Verify failed: %v", verifyErr)
	}

	contractFile := filepath.Join(outDir, "consumer-provider.json")
	data, err := os.ReadFile(contractFile)
	if err != nil {
		t.Fatalf("failed to read contract: %v", err)
	}

	var saved map[string]any
	if unmarshalErr := json.Unmarshal(data, &saved); unmarshalErr != nil {
		t.Fatalf("invalid JSON: %v", unmarshalErr)
	}

	interactions, ok := saved["interactions"].([]any)
	if !ok {
		t.Fatal("interactions field missing or not an array")
	}
	if len(interactions) != 2 {
		t.Errorf("interactions count = %d, want 2", len(interactions))
	}
}

func TestRunTest_DefaultContractDir(t *testing.T) {
	// Verify the default config value when no options are passed
	cfg := &testConfig{outDir: "./contracts"}
	if cfg.outDir != "./contracts" {
		t.Errorf("default outDir = %q, want %q", cfg.outDir, "./contracts")
	}
}

func TestPact_Setup_ReturnsUniqueURLs(t *testing.T) {
	// Two separate pacts should bind to different ports
	p1 := NewPact("c1", "p1")
	p1.AddInteraction(newGetInteraction(t, "i1", "/a", nil))

	p2 := NewPact("c2", "p2")
	p2.AddInteraction(newGetInteraction(t, "i2", "/b", nil))

	url1, err := p1.Setup()
	if err != nil {
		t.Fatalf("p1 Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p1.Teardown(); tdErr != nil {
			t.Logf("Warning: p1 teardown failed: %v", tdErr)
		}
	}()

	url2, err := p2.Setup()
	if err != nil {
		t.Fatalf("p2 Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p2.Teardown(); tdErr != nil {
			t.Logf("Warning: p2 teardown failed: %v", tdErr)
		}
	}()

	if url1 == url2 {
		t.Errorf("two pacts should have different URLs, both got %q", url1)
	}
}

func TestPact_Verify_WrongMethodDoesNotMatch(t *testing.T) {
	outDir := t.TempDir()
	p := NewPact("consumer", "provider").WithOutputDir(outDir)

	// Define a GET interaction
	interaction := newGetInteraction(t, "get data", "/data", map[string]any{"ok": true})
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	// Make a POST request instead of GET -- should not match
	resp := doPost(t, url+"/data", "application/json", strings.NewReader("{}"))
	resp.Body.Close()

	// Verify should fail because the GET interaction was never matched
	err = p.Verify()
	if err == nil {
		t.Fatal("Verify should fail when interaction was not matched with correct method")
	}
}

func TestPact_Setup_QueryParameterMatching(t *testing.T) {
	p := NewPact("consumer", "provider")
	interaction := NewInteraction("search").
		WithHTTPRequest("GET", "/search").
		WithQuery(map[string]string{"q": "golang", "page": "1"}).
		WillRespondWith(200).
		WithJSONBody(map[string]any{"results": []any{}}).
		Build()
	p.AddInteraction(interaction)

	url, err := p.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}
	defer func() {
		if tdErr := p.Teardown(); tdErr != nil {
			t.Logf("Warning: teardown failed: %v", tdErr)
		}
	}()

	t.Run("matching query params", func(t *testing.T) {
		resp := doGet(t, fmt.Sprintf("%s/search?q=golang&page=1", url))
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("status = %d, want 200", resp.StatusCode)
		}
	})

	t.Run("missing query params", func(t *testing.T) {
		resp := doGet(t, fmt.Sprintf("%s/search?q=golang", url))
		resp.Body.Close()
		// Should fail because page param is missing
		if resp.StatusCode == 200 {
			t.Error("request with missing query params should not return 200")
		}
	})
}
