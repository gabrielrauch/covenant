package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// ---------- helpers ----------

// newHTTPContract builds a minimal valid contract for the given provider name
// with a single HTTP GET interaction that expects a JSON body response.
func newHTTPContract(t *testing.T, consumer, provider, path string, body map[string]any) *contract.Contract {
	t.Helper()
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        "contract-" + consumer + "-" + provider,
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: consumer, Version: "1.0.0"},
			Provider:  contract.Participant{Name: provider, Version: "1.0.0"},
			Status:    contract.StatusPublished,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "interaction-get-" + strings.TrimPrefix(path, "/"),
				Description: "GET " + path,
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   path,
						},
						Response: contract.HTTPResponse{
							Status: 200,
							Body:   mapToAny(body),
						},
					},
				},
			},
		},
	}
}

// newHTTPContractWithState builds a contract that requires a provider state.
func newHTTPContractWithState(t *testing.T, consumer, path string, body map[string]any, stateName string, stateParams map[string]any) *contract.Contract {
	t.Helper()
	c := newHTTPContract(t, consumer, "api", path, body)
	c.Interactions[0].ProviderStates = []contract.ProviderState{
		{Name: stateName, Params: stateParams},
	}
	return c
}

// newGRPCContract builds a contract with a gRPC interaction (unsupported by the verifier).
func newGRPCContract(t *testing.T, consumer, provider string) *contract.Contract {
	t.Helper()
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        "contract-grpc-" + consumer + "-" + provider,
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: consumer, Version: "1.0.0"},
			Provider:  contract.Participant{Name: provider, Version: "1.0.0"},
			Status:    contract.StatusPublished,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "interaction-grpc",
				Description: "gRPC call",
				Protocol:    contract.ProtocolGRPC,
				Payload: contract.Payload{
					GRPC: &contract.GRPCPayload{
						Service: "TestService",
						Method:  "GetItem",
						Request: contract.GRPCMessage{
							Message: map[string]any{"id": "1"},
						},
						Response: contract.GRPCResponse{
							Status:  "OK",
							Message: map[string]any{"name": "item1"},
						},
						StreamType: contract.StreamTypeUnary,
					},
				},
			},
		},
	}
}

// newAsyncContract builds a contract with an async interaction (unsupported by the verifier).
func newAsyncContract(t *testing.T, consumer, provider string) *contract.Contract {
	t.Helper()
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:        "contract-async-" + consumer + "-" + provider,
			Version:   "1.0.0",
			Consumer:  contract.Participant{Name: consumer, Version: "1.0.0"},
			Provider:  contract.Participant{Name: provider, Version: "1.0.0"},
			Status:    contract.StatusPublished,
			CreatedAt: time.Now().UTC(),
		},
		Interactions: []contract.Interaction{
			{
				ID:          "interaction-async",
				Description: "async message",
				Protocol:    contract.ProtocolAsync,
				Payload: contract.Payload{
					Async: &contract.AsyncPayload{
						Destination: "events.topic",
						Direction:   contract.AsyncDirectionPublish,
						Message: contract.AsyncMessage{
							Payload: map[string]any{"event": "created"},
						},
					},
				},
			},
		},
	}
}

// mapToAny converts a typed map to any (interface{}) for contract bodies.
func mapToAny(m map[string]any) any {
	return m
}

// startProviderServer creates a httptest.Server that serves JSON at
// the given routes. Each route maps a "METHOD /path" key to a handler
// that writes a status code and a JSON body.
type route struct {
	status int
	body   map[string]any
}

func startProviderServer(t *testing.T, routes map[string]route) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		rt, ok := routes[key]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(rt.status)
		if rt.body != nil {
			if err := json.NewEncoder(w).Encode(rt.body); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// ---------- tests ----------

//nolint:gocyclo // test function
func TestVerify(t *testing.T) {
	t.Run("no contracts returns error", func(t *testing.T) {
		v := NewVerifier("my-api", "1.0.0", "http://localhost:9999")
		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err == nil {
			t.Fatal("expected error when no contracts loaded")
		}
		if !strings.Contains(err.Error(), "no contracts") {
			t.Fatalf("unexpected error: %v", err)
		}
		if report != nil {
			t.Fatal("expected nil report on error")
		}
	})

	t.Run("skips contracts for other providers", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{})
		v := NewVerifier("my-api", "1.0.0", srv.URL)

		// Add a contract for a DIFFERENT provider.
		other := newHTTPContract(t, "consumer", "other-api", "/items",
			map[string]any{"id": "1"})
		v.AddContract(other)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Results) != 0 {
			t.Fatalf("expected 0 results for non-matching provider, got %d", len(report.Results))
		}
		if !report.Success {
			t.Fatal("report should still be considered successful when no matching contracts")
		}
	})

	t.Run("all interactions pass", func(t *testing.T) {
		body := map[string]any{"id": "42", "name": "widget"}
		srv := startProviderServer(t, map[string]route{
			"GET /items/42": {status: 200, body: body},
		})
		v := NewVerifier("my-api", "1.0.0", srv.URL)
		c := newHTTPContract(t, "consumer", "my-api", "/items/42", body)
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			for _, r := range report.Results {
				for _, ir := range r.InteractionResults {
					for _, e := range ir.Errors {
						t.Logf("  error: %s — %s", e.Path, e.Message)
					}
				}
			}
			t.Fatal("expected all interactions to pass")
		}
		if report.Provider != "my-api" {
			t.Errorf("Provider = %s, want my-api", report.Provider)
		}
		if report.ProviderVersion != "1.0.0" {
			t.Errorf("ProviderVersion = %s, want 1.0.0", report.ProviderVersion)
		}
		if report.VerifiedAt.IsZero() {
			t.Error("VerifiedAt should be set")
		}
		if len(report.Results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(report.Results))
		}
		r := report.Results[0]
		if r.Summary.Total != 1 || r.Summary.Passed != 1 || r.Summary.Failed != 0 {
			t.Errorf("unexpected summary: total=%d, passed=%d, failed=%d",
				r.Summary.Total, r.Summary.Passed, r.Summary.Failed)
		}
	})

	t.Run("multiple contracts all pass", func(t *testing.T) {
		body1 := map[string]any{"id": "1"}
		body2 := map[string]any{"id": "2"}
		srv := startProviderServer(t, map[string]route{
			"GET /a": {status: 200, body: body1},
			"GET /b": {status: 200, body: body2},
		})
		v := NewVerifier("api", "2.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "c1", "api", "/a", body1))
		v.AddContract(newHTTPContract(t, "c2", "api", "/b", body2))

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			t.Fatal("expected success with two passing contracts")
		}
		if len(report.Results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(report.Results))
		}
	})

	t.Run("AddContracts convenience", func(t *testing.T) {
		body := map[string]any{"ok": true}
		srv := startProviderServer(t, map[string]route{
			"GET /x": {status: 200, body: body},
			"GET /y": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		contracts := []*contract.Contract{
			newHTTPContract(t, "c1", "api", "/x", body),
			newHTTPContract(t, "c2", "api", "/y", body),
		}
		v.AddContracts(contracts)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(report.Results) != 2 {
			t.Fatalf("expected 2 results from AddContracts, got %d", len(report.Results))
		}
	})

	t.Run("failing interaction reports failure", func(t *testing.T) {
		// Contract expects status 200, but the provider returns 404.
		srv := startProviderServer(t, map[string]route{
			"GET /missing": {status: 404, body: map[string]any{"error": "not found"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContract(t, "consumer", "api", "/missing",
			map[string]any{"id": "1"})
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure when status code does not match")
		}
		if len(report.Results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(report.Results))
		}
		r := report.Results[0]
		if r.Success {
			t.Fatal("expected contract result to be failed")
		}
		if r.Summary.Failed != 1 {
			t.Errorf("expected 1 failed interaction, got %d", r.Summary.Failed)
		}
	})

	t.Run("FailFast stops after first failing contract", func(t *testing.T) {
		body := map[string]any{"id": "1"}
		// First contract will fail (status mismatch), second should be skipped.
		srv := startProviderServer(t, map[string]route{
			"GET /fail":    {status: 500, body: nil},
			"GET /succeed": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		// fail contract first
		v.AddContract(newHTTPContract(t, "c-fail", "api", "/fail", body))
		v.AddContract(newHTTPContract(t, "c-ok", "api", "/succeed", body))

		report, err := v.Verify(context.Background(), VerificationOptions{FailFast: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure")
		}
		// With FailFast, only the first (failing) contract should be verified.
		if len(report.Results) != 1 {
			t.Fatalf("expected 1 result with FailFast, got %d", len(report.Results))
		}
	})

	t.Run("FailFast does not stop on success", func(t *testing.T) {
		body := map[string]any{"ok": true}
		srv := startProviderServer(t, map[string]route{
			"GET /a": {status: 200, body: body},
			"GET /b": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "c1", "api", "/a", body))
		v.AddContract(newHTTPContract(t, "c2", "api", "/b", body))

		report, err := v.Verify(context.Background(), VerificationOptions{FailFast: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			t.Fatal("expected all passing even with FailFast")
		}
		if len(report.Results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(report.Results))
		}
	})

	t.Run("default timeout is applied when zero", func(t *testing.T) {
		body := map[string]any{"ok": true}
		srv := startProviderServer(t, map[string]route{
			"GET /t": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "c", "api", "/t", body))

		// Timeout=0 should be replaced with the default 30s.
		report, err := v.Verify(context.Background(), VerificationOptions{Timeout: 0})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			t.Fatal("expected success")
		}
	})

	t.Run("mixed matching and non-matching provider contracts", func(t *testing.T) {
		body := map[string]any{"data": "hello"}
		srv := startProviderServer(t, map[string]route{
			"GET /mine": {status: 200, body: body},
		})
		v := NewVerifier("my-api", "1.0.0", srv.URL)
		// One contract targets our provider, one targets a different provider.
		v.AddContract(newHTTPContract(t, "c1", "my-api", "/mine", body))
		v.AddContract(newHTTPContract(t, "c2", "other-api", "/theirs", body))

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Only the matching provider's contract should appear in results.
		if len(report.Results) != 1 {
			t.Fatalf("expected 1 result (skipping non-matching provider), got %d", len(report.Results))
		}
		if !report.Success {
			t.Fatal("expected success")
		}
	})
}

//nolint:gocyclo // test function
func TestVerifyInteraction(t *testing.T) {
	t.Run("missing state handler", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{
			"GET /items": {status: 200, body: map[string]any{"id": "1"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContractWithState(t, "consumer", "/items",
			map[string]any{"id": "1"}, "item exists", nil)
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure when state handler is missing")
		}
		ir := report.Results[0].InteractionResults[0]
		if ir.Success {
			t.Fatal("expected interaction to fail")
		}
		found := false
		for _, e := range ir.Errors {
			if strings.Contains(e.Message, "no handler for provider state") {
				found = true
			}
		}
		if !found {
			t.Error("expected error about missing state handler")
		}
	})

	t.Run("state handler error propagates", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{
			"GET /items": {status: 200, body: map[string]any{"id": "1"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContractWithState(t, "consumer", "/items",
			map[string]any{"id": "1"}, "item exists", nil)
		v.AddContract(c)
		v.OnProviderState("item exists", func(_ contract.ProviderState) error {
			return fmt.Errorf("database connection failed")
		})

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure when state handler returns error")
		}
		ir := report.Results[0].InteractionResults[0]
		found := false
		for _, e := range ir.Errors {
			if strings.Contains(e.Message, "state setup failed") &&
				strings.Contains(e.Message, "database connection failed") {
				found = true
			}
		}
		if !found {
			t.Error("expected error about state setup failure with original cause")
		}
	})

	t.Run("state handler is invoked with params", func(t *testing.T) {
		body := map[string]any{"id": "42", "name": "widget"}
		srv := startProviderServer(t, map[string]route{
			"GET /items": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		params := map[string]any{"id": "42"}
		c := newHTTPContractWithState(t, "inventory-app", "/items",
			body, "item exists", params)
		v.AddContract(c)

		var receivedState contract.ProviderState
		v.OnProviderState("item exists", func(state contract.ProviderState) error {
			receivedState = state
			return nil
		})

		_, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if receivedState.Name != "item exists" {
			t.Errorf("state name = %s, want item exists", receivedState.Name)
		}
		if receivedState.Params["id"] != "42" {
			t.Errorf("state param id = %v, want 42", receivedState.Params["id"])
		}
	})

	t.Run("unsupported protocol gRPC", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newGRPCContract(t, "consumer", "api")
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure for unsupported gRPC protocol")
		}
		ir := report.Results[0].InteractionResults[0]
		found := false
		for _, e := range ir.Errors {
			if strings.Contains(e.Message, "unsupported protocol") {
				found = true
			}
		}
		if !found {
			t.Error("expected error about unsupported protocol")
		}
	})

	t.Run("unsupported protocol async", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newAsyncContract(t, "consumer", "api")
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure for unsupported async protocol")
		}
		ir := report.Results[0].InteractionResults[0]
		found := false
		for _, e := range ir.Errors {
			if strings.Contains(e.Message, "unsupported protocol") {
				found = true
			}
		}
		if !found {
			t.Error("expected error about unsupported protocol")
		}
	})

	t.Run("interaction duration is recorded", func(t *testing.T) {
		body := map[string]any{"ok": true}
		srv := startProviderServer(t, map[string]route{
			"GET /dur": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "c", "api", "/dur", body))

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := report.Results[0]
		if r.DurationMS < 0 {
			t.Errorf("contract DurationMS = %d, should be >= 0", r.DurationMS)
		}
		ir := r.InteractionResults[0]
		if ir.DurationMS < 0 {
			t.Errorf("interaction DurationMS = %d, should be >= 0", ir.DurationMS)
		}
	})

	t.Run("contract metadata populated in result", func(t *testing.T) {
		body := map[string]any{"ok": true}
		srv := startProviderServer(t, map[string]route{
			"GET /meta": {status: 200, body: body},
		})
		v := NewVerifier("api", "2.5.0", srv.URL)
		c := newHTTPContract(t, "web-ui", "api", "/meta", body)
		c.Metadata.ID = "contract-abc-123"
		c.Metadata.Version = "3.0.0"
		c.Metadata.Consumer.Version = "1.2.3"
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := report.Results[0]
		if r.ContractID != "contract-abc-123" {
			t.Errorf("ContractID = %s, want contract-abc-123", r.ContractID)
		}
		if r.ContractVersion != "3.0.0" {
			t.Errorf("ContractVersion = %s, want 3.0.0", r.ContractVersion)
		}
		if r.Provider.Name != "api" || r.Provider.Version != "2.5.0" {
			t.Errorf("Provider = %+v, want {api 2.5.0}", r.Provider)
		}
		if r.Consumer.Name != "web-ui" || r.Consumer.Version != "1.2.3" {
			t.Errorf("Consumer = %+v, want {web-ui 1.2.3}", r.Consumer)
		}
	})

	t.Run("multiple interactions mixed pass and fail", func(t *testing.T) {
		srv := startProviderServer(t, map[string]route{
			"GET /ok":   {status: 200, body: map[string]any{"ok": true}},
			"GET /fail": {status: 500, body: nil},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := &contract.Contract{
			Metadata: contract.Metadata{
				ID:        "contract-multi",
				Version:   "1.0.0",
				Consumer:  contract.Participant{Name: "consumer", Version: "1.0.0"},
				Provider:  contract.Participant{Name: "api", Version: "1.0.0"},
				Status:    contract.StatusPublished,
				CreatedAt: time.Now().UTC(),
			},
			Interactions: []contract.Interaction{
				{
					ID:          "i-ok",
					Description: "GET /ok",
					Protocol:    contract.ProtocolHTTP,
					Payload: contract.Payload{
						HTTP: &contract.HTTPPayload{
							Request: contract.HTTPRequest{
								Method: "GET",
								Path:   "/ok",
							},
							Response: contract.HTTPResponse{
								Status: 200,
								Body:   map[string]any{"ok": true},
							},
						},
					},
				},
				{
					ID:          "i-fail",
					Description: "GET /fail",
					Protocol:    contract.ProtocolHTTP,
					Payload: contract.Payload{
						HTTP: &contract.HTTPPayload{
							Request: contract.HTTPRequest{
								Method: "GET",
								Path:   "/fail",
							},
							Response: contract.HTTPResponse{
								Status: 200,
								Body:   map[string]any{"result": "ok"},
							},
						},
					},
				},
			},
		}
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure with one failing interaction")
		}
		r := report.Results[0]
		if r.Summary.Total != 2 {
			t.Errorf("Total = %d, want 2", r.Summary.Total)
		}
		if r.Summary.Passed != 1 {
			t.Errorf("Passed = %d, want 1", r.Summary.Passed)
		}
		if r.Summary.Failed != 1 {
			t.Errorf("Failed = %d, want 1", r.Summary.Failed)
		}
	})

	t.Run("request to unreachable server fails gracefully", func(t *testing.T) {
		// Use an address that is unlikely to be listening.
		v := NewVerifier("api", "1.0.0", "http://127.0.0.1:1")
		c := newHTTPContract(t, "consumer", "api", "/test",
			map[string]any{"ok": true})
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure when provider is unreachable")
		}
		ir := report.Results[0].InteractionResults[0]
		if ir.Success {
			t.Fatal("expected interaction to fail")
		}
		found := false
		for _, e := range ir.Errors {
			if strings.Contains(e.Message, "request failed") {
				found = true
			}
		}
		if !found {
			t.Error("expected 'request failed' error")
		}
	})
}

//nolint:gocyclo // test function
func TestVerifyHTTPInteraction(t *testing.T) {
	t.Run("response body type mismatch reports error", func(t *testing.T) {
		// Contract expects {"count": 42} (number) but the provider returns {"count": "not-a-number"} (string).
		srv := startProviderServer(t, map[string]route{
			"GET /body": {status: 200, body: map[string]any{"count": "not-a-number"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContract(t, "consumer", "api", "/body",
			map[string]any{"count": 42})
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure due to body type mismatch (number vs string)")
		}
	})

	t.Run("response body missing key reports error", func(t *testing.T) {
		// Contract expects {"name":"value","extra":"field"} but provider only returns {"name":"value"}.
		srv := startProviderServer(t, map[string]route{
			"GET /body2": {status: 200, body: map[string]any{"name": "value"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContract(t, "consumer", "api", "/body2",
			map[string]any{"name": "value", "extra": "field"})
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected failure due to missing key in response body")
		}
	})

	t.Run("POST with request body", func(t *testing.T) {
		reqBody := map[string]any{"item": "widget"}
		respBody := map[string]any{"id": "new-1", "item": "widget"}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" || r.URL.Path != "/items" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			if err := json.NewEncoder(w).Encode(respBody); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		t.Cleanup(srv.Close)

		v := NewVerifier("api", "1.0.0", srv.URL)
		c := &contract.Contract{
			Metadata: contract.Metadata{
				ID:        "contract-post",
				Version:   "1.0.0",
				Consumer:  contract.Participant{Name: "consumer", Version: "1.0.0"},
				Provider:  contract.Participant{Name: "api", Version: "1.0.0"},
				Status:    contract.StatusPublished,
				CreatedAt: time.Now().UTC(),
			},
			Interactions: []contract.Interaction{
				{
					ID:          "i-post",
					Description: "POST /items",
					Protocol:    contract.ProtocolHTTP,
					Payload: contract.Payload{
						HTTP: &contract.HTTPPayload{
							Request: contract.HTTPRequest{
								Method:  "POST",
								Path:    "/items",
								Headers: map[string]string{"Content-Type": "application/json"},
								Body:    reqBody,
							},
							Response: contract.HTTPResponse{
								Status: 201,
								Body:   respBody,
							},
						},
					},
				},
			},
		}
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			for _, r := range report.Results {
				for _, ir := range r.InteractionResults {
					for _, e := range ir.Errors {
						t.Logf("  error: path=%s message=%s", e.Path, e.Message)
					}
				}
			}
			t.Fatal("expected POST interaction to pass")
		}
	})

	t.Run("matching rules are forwarded", func(t *testing.T) {
		// Contract expects body with type-matched "id" field.
		body := map[string]any{"id": "any-string-value"}
		srv := startProviderServer(t, map[string]route{
			"GET /typed": {status: 200, body: map[string]any{"id": "different-value"}},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)

		c := newHTTPContract(t, "consumer", "api", "/typed", body)
		c.MatchingRules = contract.MatchingRules{
			"$.response.body.id": {Match: contract.MatchTypeValue},
		}
		v.AddContract(c)

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// With type matching, the value "different-value" should match because
		// it is the same type (string) as "any-string-value".
		if !report.Success {
			for _, r := range report.Results {
				for _, ir := range r.InteractionResults {
					for _, e := range ir.Errors {
						t.Logf("  error: path=%s message=%s", e.Path, e.Message)
					}
				}
			}
			t.Fatal("expected success with type matching rule")
		}
	})
}

func TestVerifyWithTesting(t *testing.T) {
	t.Run("passes when all interactions match", func(t *testing.T) {
		body := map[string]any{"status": "ok"}
		srv := startProviderServer(t, map[string]route{
			"GET /health": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "monitor", "api", "/health", body))

		// Run inside a sub-test so we can inspect its result without
		// causing the outer test to fail.
		passed := t.Run("verify-pass", func(st *testing.T) {
			v.VerifyWithTesting(st)
		})
		if !passed {
			t.Fatal("expected VerifyWithTesting to pass")
		}
	})

	t.Run("fails t when an interaction does not match", func(t *testing.T) {
		// Provider returns 500 but contract expects 200.
		srv := startProviderServer(t, map[string]route{
			"GET /bad": {status: 500, body: nil},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		v.AddContract(newHTTPContract(t, "consumer", "api", "/bad",
			map[string]any{"ok": true}))

		// We need to capture test output to verify it fails without
		// actually failing this test. We cannot override testing.T,
		// but we can check the report directly.
		report, err := v.Verify(context.Background(), VerificationOptions{Timeout: 30 * time.Second})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if report.Success {
			t.Fatal("expected report to indicate failure")
		}
	})

	t.Run("fatal when no contracts", func(t *testing.T) {
		// VerifyWithTesting with no contracts should call t.Fatalf,
		// which we cannot test without causing a real fatal. Instead
		// verify via the underlying Verify method.
		v := NewVerifier("api", "1.0.0", "http://localhost:1")
		_, err := v.Verify(context.Background(), VerificationOptions{})
		if err == nil {
			t.Fatal("expected error for no contracts")
		}
	})

	t.Run("creates subtests per consumer and interaction", func(t *testing.T) {
		body := map[string]any{"id": "1"}
		srv := startProviderServer(t, map[string]route{
			"GET /items/1": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContract(t, "web-app", "api", "/items/1", body)
		c.Metadata.Version = "2.0.0"
		v.AddContract(c)

		passed := t.Run("verify-subtests", func(st *testing.T) {
			v.VerifyWithTesting(st)
		})
		if !passed {
			t.Fatal("expected VerifyWithTesting subtests to pass")
		}
	})
}

func TestOnProviderState(t *testing.T) {
	t.Run("returns the verifier for chaining", func(t *testing.T) {
		v := NewVerifier("api", "1.0.0", "http://localhost")
		result := v.OnProviderState("some state", func(_ contract.ProviderState) error {
			return nil
		})
		if result != v {
			t.Fatal("OnProviderState should return the same verifier for chaining")
		}
	})

	t.Run("successful state setup allows interaction to proceed", func(t *testing.T) {
		body := map[string]any{"id": "42", "name": "test-item"}
		srv := startProviderServer(t, map[string]route{
			"GET /items/42": {status: 200, body: body},
		})
		v := NewVerifier("api", "1.0.0", srv.URL)
		c := newHTTPContractWithState(t, "consumer", "/items/42",
			body, "item 42 exists", map[string]any{"id": "42"})
		v.AddContract(c)
		v.OnProviderState("item 42 exists", func(_ contract.ProviderState) error {
			// State setup succeeds.
			return nil
		})

		report, err := v.Verify(context.Background(), VerificationOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !report.Success {
			t.Fatal("expected success when state handler succeeds and response matches")
		}
	})
}

func TestNewVerifier(t *testing.T) {
	v := NewVerifier("test-provider", "3.2.1", "http://example.com")
	if v == nil {
		t.Fatal("NewVerifier returned nil")
	}
	if v.providerName != "test-provider" {
		t.Errorf("providerName = %s, want test-provider", v.providerName)
	}
	if v.providerVersion != "3.2.1" {
		t.Errorf("providerVersion = %s, want 3.2.1", v.providerVersion)
	}
	if v.baseURL != "http://example.com" {
		t.Errorf("baseURL = %s, want http://example.com", v.baseURL)
	}
	if v.stateHandlers == nil {
		t.Fatal("stateHandlers map should be initialized")
	}
	if v.httpValidator == nil {
		t.Fatal("httpValidator should be initialized")
	}
}

func TestAddContract(t *testing.T) {
	t.Run("returns verifier for chaining", func(t *testing.T) {
		v := NewVerifier("api", "1.0.0", "http://localhost")
		c := newHTTPContract(t, "c", "api", "/x", nil)
		result := v.AddContract(c)
		if result != v {
			t.Fatal("AddContract should return the same verifier for chaining")
		}
	})

	t.Run("appends to contracts slice", func(t *testing.T) {
		v := NewVerifier("api", "1.0.0", "http://localhost")
		c1 := newHTTPContract(t, "c1", "api", "/a", nil)
		c2 := newHTTPContract(t, "c2", "api", "/b", nil)
		v.AddContract(c1).AddContract(c2)

		if len(v.contracts) != 2 {
			t.Fatalf("expected 2 contracts, got %d", len(v.contracts))
		}
	})
}
