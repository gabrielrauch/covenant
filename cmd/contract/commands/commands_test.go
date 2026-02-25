package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/broker/brokerapi"
	"github.com/gabrielrauch/covenant/pkg/contract"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

// ─── common.go: getEnv ───────────────────────────────────────────────────────

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		defaultVal string
		envVal     string
		want       string
	}{
		{
			name:       "returns default when env not set",
			key:        "TEST_GETENV_MISSING",
			defaultVal: "fallback",
			want:       "fallback",
		},
		{
			name:       "returns env value when set",
			key:        "TEST_GETENV_SET",
			defaultVal: "fallback",
			envVal:     "from-env",
			want:       "from-env",
		},
		{
			name:       "returns default when env is empty string",
			key:        "TEST_GETENV_EMPTY",
			defaultVal: "fallback",
			envVal:     "",
			want:       "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				t.Setenv(tt.key, tt.envVal)
			}
			got := getEnv(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnv(%q, %q) = %q, want %q", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

// ─── http.go: NewHTTPClient ──────────────────────────────────────────────────

func TestNewHTTPClient(t *testing.T) {
	tests := []struct {
		name      string
		brokerURL string
	}{
		{name: "localhost URL", brokerURL: "http://localhost:8080"},
		{name: "custom URL", brokerURL: "https://broker.example.com"},
		{name: "with trailing slash", brokerURL: "http://localhost:8080/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewHTTPClient(tt.brokerURL)
			if c == nil {
				t.Fatal("NewHTTPClient returned nil")
			}
			if c.baseURL != tt.brokerURL {
				t.Errorf("baseURL = %q, want %q", c.baseURL, tt.brokerURL)
			}
			if c.client == nil {
				t.Error("client is nil")
			}
		})
	}
}

// ─── http.go: Get ────────────────────────────────────────────────────────────

func TestHTTPClient_Get(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		query          url.Values
		serverStatus   int
		serverBody     string
		wantErr        bool
		wantErrContain string
		wantBody       string
	}{
		{
			name:         "successful GET with no query",
			path:         "/test",
			query:        nil,
			serverStatus: http.StatusOK,
			serverBody:   `{"result":"ok"}`,
			wantBody:     `{"result":"ok"}`,
		},
		{
			name:         "successful GET with query params",
			path:         "/search",
			query:        url.Values{"q": {"hello"}, "page": {"1"}},
			serverStatus: http.StatusOK,
			serverBody:   `{"items":[]}`,
			wantBody:     `{"items":[]}`,
		},
		{
			name:           "non-200 status returns error",
			path:           "/notfound",
			serverStatus:   http.StatusNotFound,
			serverBody:     `{"error":"not found"}`,
			wantErr:        true,
			wantErrContain: "request failed",
		},
		{
			name:           "500 server error",
			path:           "/error",
			serverStatus:   http.StatusInternalServerError,
			serverBody:     `internal server error`,
			wantErr:        true,
			wantErrContain: "request failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("expected GET, got %s", r.Method)
				}
				if tt.query != nil {
					for key, vals := range tt.query {
						got := r.URL.Query().Get(key)
						if got != vals[0] {
							t.Errorf("query param %q = %q, want %q", key, got, vals[0])
						}
					}
				}
				w.WriteHeader(tt.serverStatus)
				fmt.Fprint(w, tt.serverBody)
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			body, err := client.Get(context.Background(), tt.path, tt.query)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContain)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", string(body), tt.wantBody)
			}
		})
	}
}

func TestHTTPClient_Get_ConnectionError(t *testing.T) {
	client := NewHTTPClient("http://127.0.0.1:0")
	_, err := client.Get(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error %q should contain 'request failed'", err.Error())
	}
}

// ─── http.go: GetJSON ────────────────────────────────────────────────────────

func TestHTTPClient_GetJSON(t *testing.T) {
	tests := []struct {
		name       string
		serverBody string
		status     int
		wantErr    bool
	}{
		{
			name:       "successful JSON decode",
			serverBody: `{"name":"test","value":42}`,
			status:     http.StatusOK,
		},
		{
			name:       "invalid JSON returns error",
			serverBody: `{invalid`,
			status:     http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "non-200 status returns error",
			serverBody: `error`,
			status:     http.StatusBadRequest,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				fmt.Fprint(w, tt.serverBody)
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			var result map[string]any
			err := client.GetJSON(context.Background(), "/data", nil, &result)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result["name"] != "test" {
				t.Errorf("result[name] = %v, want 'test'", result["name"])
			}
		})
	}
}

// ─── http.go: PostJSON ───────────────────────────────────────────────────────

func TestHTTPClient_PostJSON(t *testing.T) {
	tests := []struct {
		name           string
		body           any
		expectedStatus int
		serverStatus   int
		serverBody     string
		wantErr        bool
	}{
		{
			name:           "successful POST with 201",
			body:           map[string]string{"key": "value"},
			expectedStatus: http.StatusCreated,
			serverStatus:   http.StatusCreated,
			serverBody:     `{"id":"123"}`,
		},
		{
			name:           "successful POST with 200",
			body:           map[string]int{"count": 1},
			expectedStatus: http.StatusOK,
			serverStatus:   http.StatusOK,
			serverBody:     `{"ok":true}`,
		},
		{
			name:           "status mismatch returns error",
			body:           map[string]string{"key": "value"},
			expectedStatus: http.StatusCreated,
			serverStatus:   http.StatusBadRequest,
			serverBody:     `{"error":"bad request"}`,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if ct := r.Header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}
				bodyBytes, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if len(bodyBytes) == 0 {
					t.Error("expected non-empty body")
				}
				w.WriteHeader(tt.serverStatus)
				fmt.Fprint(w, tt.serverBody)
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			respBody, err := client.PostJSON(context.Background(), "/create", tt.body, tt.expectedStatus)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(respBody) != tt.serverBody {
				t.Errorf("body = %q, want %q", string(respBody), tt.serverBody)
			}
		})
	}
}

func TestHTTPClient_PostJSON_ConnectionError(t *testing.T) {
	client := NewHTTPClient("http://127.0.0.1:0")
	_, err := client.PostJSON(context.Background(), "/test", map[string]string{"a": "b"}, 200)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

// ─── http.go: PostJSONResult ─────────────────────────────────────────────────

func TestHTTPClient_PostJSONResult(t *testing.T) {
	tests := []struct {
		name           string
		body           any
		expectedStatus int
		serverStatus   int
		serverBody     string
		target         any
		wantErr        bool
	}{
		{
			name:           "successful POST and unmarshal",
			body:           map[string]string{"name": "test"},
			expectedStatus: http.StatusOK,
			serverStatus:   http.StatusOK,
			serverBody:     `{"id":"abc","version":"1.0"}`,
			target:         &publishResult{},
		},
		{
			name:           "nil target skips unmarshal",
			body:           map[string]string{"name": "test"},
			expectedStatus: http.StatusOK,
			serverStatus:   http.StatusOK,
			serverBody:     `{"id":"abc"}`,
			target:         nil,
		},
		{
			name:           "server error propagates",
			body:           map[string]string{"name": "test"},
			expectedStatus: http.StatusCreated,
			serverStatus:   http.StatusBadRequest,
			serverBody:     `bad`,
			target:         &publishResult{},
			wantErr:        true,
		},
		{
			name:           "invalid JSON response returns error",
			body:           map[string]string{"name": "test"},
			expectedStatus: http.StatusOK,
			serverStatus:   http.StatusOK,
			serverBody:     `{not json}`,
			target:         &publishResult{},
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.serverStatus)
				fmt.Fprint(w, tt.serverBody)
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			err := client.PostJSONResult(context.Background(), "/create", tt.body, tt.expectedStatus, tt.target)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if pr, ok := tt.target.(*publishResult); ok {
				if pr.ID == "" {
					t.Error("expected non-empty ID after unmarshal")
				}
			}
		})
	}
}

// ─── verify.go: parseVerifyFlags ─────────────────────────────────────────────

func TestParseVerifyFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		check   func(t *testing.T, cfg *verifyConfig)
	}{
		{
			name: "all required flags provided",
			args: []string{
				"--provider-url", "http://localhost:9090",
				"--provider", "orders-api",
				"--provider-version", "1.2.3",
			},
			check: func(t *testing.T, cfg *verifyConfig) {
				t.Helper()
				if cfg.providerURL != "http://localhost:9090" {
					t.Errorf("providerURL = %q, want %q", cfg.providerURL, "http://localhost:9090")
				}
				if cfg.providerName != "orders-api" {
					t.Errorf("providerName = %q, want %q", cfg.providerName, "orders-api")
				}
				if cfg.providerVersion != "1.2.3" {
					t.Errorf("providerVersion = %q, want %q", cfg.providerVersion, "1.2.3")
				}
				if !cfg.publishResults {
					t.Error("publishResults should default to true")
				}
			},
		},
		{
			name: "custom broker and contracts dir",
			args: []string{
				"--provider-url", "http://localhost:9090",
				"--provider", "orders-api",
				"--provider-version", "1.0.0",
				"--broker", "http://custom-broker:8080",
				"--contracts", "/tmp/contracts",
				"--publish=false",
			},
			check: func(t *testing.T, cfg *verifyConfig) {
				t.Helper()
				if cfg.brokerURL != "http://custom-broker:8080" {
					t.Errorf("brokerURL = %q, want %q", cfg.brokerURL, "http://custom-broker:8080")
				}
				if cfg.contractDir != "/tmp/contracts" {
					t.Errorf("contractDir = %q, want %q", cfg.contractDir, "/tmp/contracts")
				}
				if cfg.publishResults {
					t.Error("publishResults should be false")
				}
			},
		},
		{
			name:    "missing provider-url",
			args:    []string{"--provider", "api", "--provider-version", "1.0"},
			wantErr: true,
		},
		{
			name:    "missing provider",
			args:    []string{"--provider-url", "http://localhost", "--provider-version", "1.0"},
			wantErr: true,
		},
		{
			name:    "missing provider-version",
			args:    []string{"--provider-url", "http://localhost", "--provider", "api"},
			wantErr: true,
		},
		{
			name:    "all required flags missing",
			args:    []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := parseVerifyFlags(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestParseVerifyFlags_BrokerEnvVar(t *testing.T) {
	t.Setenv("COVENANT_BROKER_URL", "http://env-broker:9999")

	cfg, err := parseVerifyFlags([]string{
		"--provider-url", "http://localhost:9090",
		"--provider", "test-api",
		"--provider-version", "1.0.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.brokerURL != "http://env-broker:9999" {
		t.Errorf("brokerURL = %q, want %q (from env)", cfg.brokerURL, "http://env-broker:9999")
	}
}

// ─── verify.go: findContractFiles ────────────────────────────────────────────

func TestFindContractFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     []string
		wantErr   bool
		wantCount int
	}{
		{
			name:      "finds JSON files",
			files:     []string{"a.json", "b.json", "c.json"},
			wantCount: 3,
		},
		{
			name:      "only matches JSON files",
			files:     []string{"a.json", "b.txt", "c.yaml"},
			wantCount: 1,
		},
		{
			name:    "empty directory returns error",
			files:   []string{},
			wantErr: true,
		},
		{
			name:    "no JSON files returns error",
			files:   []string{"readme.md", "config.yaml"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range tt.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("{}"), 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
			}

			files, err := findContractFiles(dir)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(files) != tt.wantCount {
				t.Errorf("found %d files, want %d", len(files), tt.wantCount)
			}
		})
	}
}

func TestFindContractFiles_NonexistentDir(t *testing.T) {
	_, err := findContractFiles("/nonexistent/path/abc123")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

// ─── publish.go: ensureTagInContract ─────────────────────────────────────────

func TestEnsureTagInContract(t *testing.T) {
	tests := []struct {
		name         string
		existingTags []string
		tag          string
		wantTags     []string
	}{
		{
			name:         "adds tag to empty tags",
			existingTags: nil,
			tag:          "prod",
			wantTags:     []string{"prod"},
		},
		{
			name:         "adds tag to existing tags",
			existingTags: []string{"staging"},
			tag:          "prod",
			wantTags:     []string{"staging", "prod"},
		},
		{
			name:         "does not duplicate existing tag",
			existingTags: []string{"prod", "staging"},
			tag:          "prod",
			wantTags:     []string{"prod", "staging"},
		},
		{
			name:         "empty tag is a no-op",
			existingTags: []string{"prod"},
			tag:          "",
			wantTags:     []string{"prod"},
		},
		{
			name:         "empty tag with nil tags",
			existingTags: nil,
			tag:          "",
			wantTags:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &contract.Contract{
				Metadata: contract.Metadata{
					Tags: tt.existingTags,
				},
			}

			ensureTagInContract(c, tt.tag)

			if len(c.Metadata.Tags) != len(tt.wantTags) {
				t.Fatalf("tags length = %d, want %d; tags = %v", len(c.Metadata.Tags), len(tt.wantTags), c.Metadata.Tags)
			}
			for i, tag := range tt.wantTags {
				if c.Metadata.Tags[i] != tag {
					t.Errorf("tags[%d] = %q, want %q", i, c.Metadata.Tags[i], tag)
				}
			}
		})
	}
}

// ─── candeploy.go: printCanDeployResult ──────────────────────────────────────

func TestPrintCanDeployResult(t *testing.T) {
	tests := []struct {
		name           string
		result         canDeployResult
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "safe to deploy with environment",
			result: canDeployResult{
				OK:      true,
				Service: "orders-api",
				Version: "1.2.3",
				Env:     "production",
			},
			wantContains: []string{
				"orders-api",
				"1.2.3",
				"production",
				"Safe to deploy",
			},
		},
		{
			name: "not safe to deploy",
			result: canDeployResult{
				OK:      false,
				Service: "orders-api",
				Version: "1.0.0",
			},
			wantContains: []string{
				"orders-api",
				"NOT safe to deploy",
			},
		},
		{
			name: "with reasons",
			result: canDeployResult{
				OK:      false,
				Service: "api",
				Version: "2.0",
				Reasons: []struct {
					ContractID string `json:"contract_id"`
					Consumer   string `json:"consumer"`
					Provider   string `json:"provider"`
					Status     string `json:"status"`
					Message    string `json:"message"`
				}{
					{
						ContractID: "c-1",
						Consumer:   "web-app",
						Provider:   "api",
						Status:     "verified",
						Message:    "all passed",
					},
					{
						ContractID: "c-2",
						Consumer:   "mobile",
						Provider:   "api",
						Status:     "failed",
						Message:    "verification failed",
					},
				},
			},
			wantContains: []string{
				"NOT safe to deploy",
				"Reasons:",
				"web-app",
				"mobile",
				"verified",
				"failed",
			},
		},
		{
			name: "no environment shown when empty",
			result: canDeployResult{
				OK:      true,
				Service: "svc",
				Version: "1.0",
			},
			wantContains:   []string{"Safe to deploy"},
			wantNotContain: []string{"Environment:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := captureStdout(t, func() {
				printCanDeployResult(&tt.result)
			})

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q:\n%s", want, output)
				}
			}
			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("output should not contain %q:\n%s", notWant, output)
				}
			}
		})
	}
}

// ─── fetch.go: fetchContractList ─────────────────────────────────────────────

func TestFetchContractList(t *testing.T) {
	tests := []struct {
		name       string
		serverBody string
		status     int
		query      url.Values
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "returns contract list",
			serverBody: `[{"id":"c1","consumer":"web","provider":"api","version":"1.0"},{"id":"c2","consumer":"mobile","provider":"api","version":"2.0"}]`,
			status:     http.StatusOK,
			query:      url.Values{"provider": {"api"}},
			wantCount:  2,
		},
		{
			name:       "empty list",
			serverBody: `[]`,
			status:     http.StatusOK,
			wantCount:  0,
		},
		{
			name:    "server error",
			status:  http.StatusInternalServerError,
			wantErr: true,
		},
		{
			name:       "invalid JSON",
			serverBody: `not-json`,
			status:     http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/contracts" {
					t.Errorf("path = %q, want /contracts", r.URL.Path)
				}
				w.WriteHeader(tt.status)
				if tt.serverBody != "" {
					fmt.Fprint(w, tt.serverBody)
				}
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			summaries, err := fetchContractList(context.Background(), client, tt.query)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(summaries) != tt.wantCount {
				t.Errorf("got %d summaries, want %d", len(summaries), tt.wantCount)
			}
		})
	}
}

// ─── fetch.go: fetchAndSaveContract ──────────────────────────────────────────

func TestFetchAndSaveContract(t *testing.T) {
	validContract := contract.Contract{
		Metadata: contract.Metadata{
			ID:       "test-contract-id",
			Version:  "1.0.0",
			Consumer: contract.Participant{Name: "web-app"},
			Provider: contract.Participant{Name: "orders-api"},
		},
		Interactions: []contract.Interaction{
			{
				ID:          "int-1",
				Description: "get order",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/orders/1",
						},
						Response: contract.HTTPResponse{
							Status: 200,
						},
					},
				},
			},
		},
	}

	contractJSON, err := json.Marshal(validContract)
	if err != nil {
		t.Fatalf("failed to marshal test contract: %v", err)
	}

	tests := []struct {
		name       string
		summary    contractSummary
		serverBody string
		status     int
		wantErr    bool
	}{
		{
			name: "successfully fetches and saves contract",
			summary: contractSummary{
				ID:       "test-contract-id",
				Consumer: "web-app",
				Provider: "orders-api",
				Version:  "1.0.0",
			},
			serverBody: string(contractJSON),
			status:     http.StatusOK,
		},
		{
			name: "server error returns error",
			summary: contractSummary{
				ID:       "bad-id",
				Consumer: "web-app",
				Provider: "orders-api",
				Version:  "1.0.0",
			},
			status:  http.StatusNotFound,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantPath := fmt.Sprintf("/contracts/%s", tt.summary.ID)
				if r.URL.Path != wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, wantPath)
				}
				w.WriteHeader(tt.status)
				if tt.serverBody != "" {
					fmt.Fprint(w, tt.serverBody)
				}
			}))
			defer srv.Close()

			outDir := t.TempDir()
			client := NewHTTPClient(srv.URL)

			captureStdout(t, func() {
				err := fetchAndSaveContract(context.Background(), client, outDir, tt.summary)

				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
					return
				}

				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			if !tt.wantErr {
				expectedFile := filepath.Join(outDir, fmt.Sprintf("%s-%s-%s.json", tt.summary.Consumer, tt.summary.Provider, tt.summary.Version))
				if _, statErr := os.Stat(expectedFile); os.IsNotExist(statErr) {
					t.Errorf("expected file %q to exist", expectedFile)
				}
			}
		})
	}
}

// ─── verify.go: verifyInteraction with httptest ──────────────────────────────

func TestVerifyInteraction(t *testing.T) {
	tests := []struct {
		name        string
		interaction contract.Interaction
		serverFunc  http.HandlerFunc
		wantSuccess bool
	}{
		{
			name: "matching response passes",
			interaction: contract.Interaction{
				ID:          "test-1",
				Description: "get user",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/users/1",
						},
						Response: contract.HTTPResponse{
							Status: 200,
							Body:   map[string]any{"id": float64(1), "name": "Alice"},
						},
					},
				},
			},
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				if err := json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Alice"}); err != nil {
					t.Fatalf("failed to encode response: %v", err)
				}
			},
			wantSuccess: true,
		},
		{
			name: "wrong status code fails",
			interaction: contract.Interaction{
				ID:          "test-2",
				Description: "create order",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "POST",
							Path:   "/orders",
						},
						Response: contract.HTTPResponse{
							Status: 201,
						},
					},
				},
			},
			serverFunc: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(500)
			},
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.serverFunc)
			defer srv.Close()

			validator := httpvalidator.NewValidator()
			interaction := tt.interaction

			captureStdout(t, func() {
				result := verifyInteraction(
					context.Background(),
					srv.URL,
					validator,
					&interaction,
					nil,
				)

				if result.Success != tt.wantSuccess {
					t.Errorf("success = %v, want %v", result.Success, tt.wantSuccess)
				}
				if result.ID != tt.interaction.ID {
					t.Errorf("ID = %q, want %q", result.ID, tt.interaction.ID)
				}
				if result.Description != tt.interaction.Description {
					t.Errorf("Description = %q, want %q", result.Description, tt.interaction.Description)
				}
			})
		})
	}
}

func TestVerifyInteraction_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server doesn't support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("failed to hijack connection: %v", err)
		}
		conn.Close()
	}))
	defer srv.Close()

	interaction := &contract.Interaction{
		ID:          "fail-1",
		Description: "broken endpoint",
		Protocol:    contract.ProtocolHTTP,
		Payload: contract.Payload{
			HTTP: &contract.HTTPPayload{
				Request: contract.HTTPRequest{
					Method: "GET",
					Path:   "/broken",
				},
				Response: contract.HTTPResponse{Status: 200},
			},
		},
	}

	validator := httpvalidator.NewValidator()

	captureStdout(t, func() {
		result := verifyInteraction(context.Background(), srv.URL, validator, interaction, nil)
		if result.Success {
			t.Error("expected failure for connection error")
		}
		if len(result.Errors) == 0 {
			t.Error("expected errors to be reported")
		}
	})
}

// ─── publish.go: publishContract via httptest ────────────────────────────────

func TestPublishContract(t *testing.T) {
	tests := []struct {
		name       string
		serverBody string
		status     int
		wantErr    bool
		wantID     string
	}{
		{
			name:       "successful publish",
			serverBody: `{"id":"contract-123","version":"1.0.0"}`,
			status:     http.StatusCreated,
			wantID:     "contract-123",
		},
		{
			name:    "server rejects contract",
			status:  http.StatusBadRequest,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/contracts" {
					t.Errorf("path = %q, want /contracts", r.URL.Path)
				}
				if r.Method != "POST" {
					t.Errorf("method = %q, want POST", r.Method)
				}
				w.WriteHeader(tt.status)
				if tt.serverBody != "" {
					fmt.Fprint(w, tt.serverBody)
				}
			}))
			defer srv.Close()

			client := NewHTTPClient(srv.URL)
			c := &contract.Contract{
				Metadata: contract.Metadata{
					ID:       "test-id",
					Version:  "1.0.0",
					Consumer: contract.Participant{Name: "web"},
					Provider: contract.Participant{Name: "api"},
				},
			}

			result, err := publishContract(context.Background(), client, c)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

// ─── candeploy.go: CanDeploy integration test with httptest ──────────────────

func TestCanDeploy_Integration(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		serverBody string
		status     int
		wantErr    bool
	}{
		{
			name: "safe to deploy",
			args: []string{"--service", "api", "--version", "1.0"},
			serverBody: `{
				"ok": true,
				"service": "api",
				"version": "1.0",
				"reasons": []
			}`,
			status: http.StatusOK,
		},
		{
			name: "blocked deployment returns error",
			args: []string{"--service", "api", "--version", "1.0"},
			serverBody: `{
				"ok": false,
				"service": "api",
				"version": "1.0",
				"reasons": [{"status":"failed","message":"verification failed"}]
			}`,
			status:  http.StatusOK,
			wantErr: true,
		},
		{
			name:    "missing service flag",
			args:    []string{"--version", "1.0"},
			wantErr: true,
		},
		{
			name:    "missing version flag",
			args:    []string{"--service", "api"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.serverBody != "" {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tt.status)
					fmt.Fprint(w, tt.serverBody)
				}))
				defer srv.Close()
				args = append(args, "--broker", srv.URL)
			}

			captureStdout(t, func() {
				err := CanDeploy(context.Background(), args)
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
				} else {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

// ─── tag.go: Tag integration test with httptest ──────────────────────────────

func TestTag_Integration(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		handler http.HandlerFunc
		wantErr bool
	}{
		{
			name: "add tags",
			args: []string{"--contract", "c-1", "--add", "prod,staging"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("method = %q, want POST", r.Method)
				}
				w.WriteHeader(http.StatusNoContent)
			},
		},
		{
			name: "remove tags",
			args: []string{"--contract", "c-1", "--remove", "old-tag"},
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" {
					t.Errorf("method = %q, want DELETE", r.Method)
				}
				w.WriteHeader(http.StatusNoContent)
			},
		},
		{
			name:    "missing contract ID",
			args:    []string{"--add", "prod"},
			wantErr: true,
		},
		{
			name:    "no add or remove",
			args:    []string{"--contract", "c-1"},
			wantErr: true,
		},
		{
			name: "add tag server error",
			args: []string{"--contract", "c-1", "--add", "bad"},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
		{
			name: "remove tag server error",
			args: []string{"--contract", "c-1", "--remove", "bad"},
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := make([]string, len(tt.args))
			copy(args, tt.args)

			if tt.handler != nil {
				srv := httptest.NewServer(tt.handler)
				defer srv.Close()
				args = append(args, "--broker", srv.URL)
			}

			captureStdout(t, func() {
				err := Tag(context.Background(), args)
				if tt.wantErr {
					if err == nil {
						t.Fatal("expected error, got nil")
					}
				} else {
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

// ─── verify.go: publishVerificationResults ───────────────────────────────────

func TestPublishVerificationResults(t *testing.T) {
	t.Run("successful publish", func(t *testing.T) {
		var receivedBody []byte
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/verifications" {
				t.Errorf("path = %q, want /verifications", r.URL.Path)
			}
			if r.Method != "POST" {
				t.Errorf("method = %q, want POST", r.Method)
			}
			var readErr error
			receivedBody, readErr = io.ReadAll(r.Body)
			if readErr != nil {
				t.Fatalf("failed to read request body: %v", readErr)
			}
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		result := &brokerapi.VerificationResult{
			ContractID:      "c-test",
			ContractVersion: "1.0.0",
			Provider:        brokerapi.ServiceVersion{Name: "api", Version: "1.0"},
			Consumer:        brokerapi.ServiceVersion{Name: "web", Version: "1.0"},
		}

		captureStdout(t, func() {
			publishVerificationResults(context.Background(), srv.URL, result)
		})

		if len(receivedBody) == 0 {
			t.Error("expected body to be sent")
		}
	})

	t.Run("server error does not panic", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		result := &brokerapi.VerificationResult{
			ContractID: "c-test",
			Provider:   brokerapi.ServiceVersion{Name: "api", Version: "1.0"},
		}

		captureStdout(t, func() {
			publishVerificationResults(context.Background(), srv.URL, result)
		})
	})

	t.Run("unreachable server does not panic", func(t *testing.T) {
		result := &brokerapi.VerificationResult{
			ContractID: "c-test",
			Provider:   brokerapi.ServiceVersion{Name: "api", Version: "1.0"},
		}
		captureStdout(t, func() {
			publishVerificationResults(context.Background(), "http://127.0.0.1:0", result)
		})
	})
}

// ─── verify.go: verifyInteractions ───────────────────────────────────────────

func TestVerifyInteractions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		if err := json.NewEncoder(w).Encode(map[string]any{"id": 1}); err != nil {
			t.Fatalf("failed to encode response: %v", err)
		}
	}))
	defer srv.Close()

	c := &contract.Contract{
		Metadata: contract.Metadata{
			ID:       "test-id",
			Version:  "1.0.0",
			Consumer: contract.Participant{Name: "web"},
			Provider: contract.Participant{Name: "api"},
		},
		Interactions: []contract.Interaction{
			{
				ID:          "int-1",
				Description: "get item",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request:  contract.HTTPRequest{Method: "GET", Path: "/items/1"},
						Response: contract.HTTPResponse{Status: 200, Body: map[string]any{"id": float64(1)}},
					},
				},
			},
			{
				ID:          "int-2",
				Description: "async event (skipped)",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "events",
						Direction:   contract.AsyncDirectionPublish,
						Message:     contract.AsyncMessage{Payload: map[string]any{"type": "test"}},
					},
				},
			},
		},
	}

	cfg := &verifyConfig{
		providerURL: srv.URL,
	}
	validator := httpvalidator.NewValidator()
	result := &brokerapi.VerificationResult{}

	captureStdout(t, func() {
		passed, failed := verifyInteractions(context.Background(), cfg, validator, c, result)
		// Only the HTTP interaction should be processed; the async one is skipped
		if passed != 1 {
			t.Errorf("passed = %d, want 1", passed)
		}
		if failed != 0 {
			t.Errorf("failed = %d, want 0", failed)
		}
	})

	if len(result.InteractionResults) != 1 {
		t.Errorf("InteractionResults length = %d, want 1", len(result.InteractionResults))
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// captureStdout redirects stdout during fn execution and returns the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}

	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read captured stdout: %v", err)
	}
	r.Close()

	return buf.String()
}
