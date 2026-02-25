package broker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/broker/brokerapi"
	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/log"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	backend, err := storage.NewFilesystemBackend(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	return NewServer(backend).WithLogger(log.Nop())
}

func validContract() *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:       "test-contract-1",
			Consumer: contract.Participant{Name: "consumer-a"},
			Provider: contract.Participant{Name: "provider-a"},
			Version:  "1.0.0",
			Status:   contract.StatusDraft,
		},
		Interactions: []contract.Interaction{
			{
				ID:          "int-1",
				Description: "get user",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request:  contract.HTTPRequest{Method: "GET", Path: "/users/1"},
						Response: contract.HTTPResponse{Status: 200, Body: map[string]any{"id": float64(1)}},
					},
				},
			},
		},
	}
}

func publishContract(t *testing.T, srv *Server, c *contract.Contract) {
	t.Helper()
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("failed to marshal contract: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/contracts", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("publish failed: %d %s", rr.Code, rr.Body.String())
	}
}

func recordVerification(t *testing.T, srv *Server, vr *brokerapi.VerificationResult) {
	t.Helper()
	body, err := json.Marshal(vr)
	if err != nil {
		t.Fatalf("failed to marshal verification: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/verifications", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("record verification failed: %d %s", rr.Code, rr.Body.String())
	}
}

// --- Health ---

func TestHandleHealth(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want status ok", rr.Body.String())
	}
}

// --- Publish ---

func TestHandlePublishContract(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newTestServer(t)
		c := validContract()
		body, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/contracts", strings.NewReader("{bad"))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("empty contract rejected", func(t *testing.T) {
		srv := newTestServer(t)
		body, err := json.Marshal(&contract.Contract{})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing consumer name", func(t *testing.T) {
		srv := newTestServer(t)
		c := validContract()
		c.Metadata.Consumer.Name = ""
		body, err := json.Marshal(c)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

// --- List ---

func TestHandleListContracts(t *testing.T) {
	srv := newTestServer(t)

	// Publish some contracts
	c1 := validContract()
	c1.Metadata.ID = "c-1"
	c1.Metadata.Consumer.Name = "consumer-a"
	publishContract(t, srv, c1)

	c2 := validContract()
	c2.Metadata.ID = "c-2"
	c2.Metadata.Consumer.Name = "consumer-b"
	publishContract(t, srv, c2)

	t.Run("list all", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/contracts", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var summaries []brokerapi.ContractSummary
		if err := json.NewDecoder(rr.Body).Decode(&summaries); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(summaries) < 2 {
			t.Errorf("got %d contracts, want >= 2", len(summaries))
		}
	})

	t.Run("filter by consumer", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/contracts?consumer=consumer-b", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
		var summaries []brokerapi.ContractSummary
		if err := json.NewDecoder(rr.Body).Decode(&summaries); err != nil {
			t.Fatalf("decode: %v", err)
		}
		for _, s := range summaries {
			if s.Consumer != "consumer-b" {
				t.Errorf("got consumer %q, want consumer-b", s.Consumer)
			}
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/contracts?status=invalid", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

// --- Get ---

func TestHandleGetContract(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newTestServer(t)
		c := validContract()
		publishContract(t, srv, c)

		req := httptest.NewRequest(http.MethodGet, "/contracts/"+c.Metadata.ID, http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/contracts/nonexistent", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

// --- Tag ---

func TestHandleTagContract(t *testing.T) {
	srv := newTestServer(t)
	c := validContract()
	publishContract(t, srv, c)

	t.Run("success", func(t *testing.T) {
		tagReq := map[string][]string{"tags": {"env:prod"}}
		body, err := json.Marshal(tagReq)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/tags", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/tags", strings.NewReader("{bad"))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("too many tags", func(t *testing.T) {
		tags := make([]string, maxTags+1)
		for i := range tags {
			tags[i] = "tag"
		}
		tagReq := map[string][]string{"tags": tags}
		body, err := json.Marshal(tagReq)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/tags", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("contract not found", func(t *testing.T) {
		tagReq := map[string][]string{"tags": {"env"}}
		body, err := json.Marshal(tagReq)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/contracts/nonexistent/tags", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

// --- Untag ---

func TestHandleUntagContract(t *testing.T) {
	srv := newTestServer(t)
	c := validContract()
	publishContract(t, srv, c)

	// First add a tag
	tagReq := map[string][]string{"tags": {"env:prod"}}
	body, err := json.Marshal(tagReq)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	tagReqHTTP := httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/tags", bytes.NewReader(body))
	tagRR := httptest.NewRecorder()
	srv.ServeHTTP(tagRR, tagReqHTTP)

	t.Run("success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/contracts/"+c.Metadata.ID+"/tags?tags=env:prod", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("missing tags parameter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/contracts/"+c.Metadata.ID+"/tags", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

// --- Deprecate ---

func TestHandleDeprecateContract(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newTestServer(t)
		c := validContract()
		publishContract(t, srv, c)

		req := httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/deprecate", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", rr.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/contracts/nonexistent/deprecate", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

// --- Record Verification ---

func TestHandleRecordVerification(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		srv := newTestServer(t)
		c := validContract()
		publishContract(t, srv, c)

		vr := &brokerapi.VerificationResult{
			ContractID:      c.Metadata.ID,
			ContractVersion: c.Metadata.Version,
			Provider:        brokerapi.ServiceVersion{Name: "provider-a", Version: "2.0.0"},
			Consumer:        brokerapi.ServiceVersion{Name: "consumer-a", Version: "1.0.0"},
		}
		body, err := json.Marshal(vr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/verifications", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodPost, "/verifications", strings.NewReader("{bad"))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing contract ID", func(t *testing.T) {
		srv := newTestServer(t)
		vr := &brokerapi.VerificationResult{
			Provider: brokerapi.ServiceVersion{Name: "provider-a", Version: "2.0.0"},
		}
		body, err := json.Marshal(vr)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/verifications", bytes.NewReader(body))
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})
}

// --- List Verifications ---

func TestHandleListVerifications(t *testing.T) {
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/verifications/unknown-contract", http.NoBody)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

// --- Get Verification ---

func TestHandleGetVerification(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/verifications/unknown/1.0.0", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", rr.Code)
		}
	})
}

// --- Can Deploy ---

func TestHandleCanDeploy(t *testing.T) {
	t.Run("missing service", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/can-deploy?version=1.0.0", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("missing version", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/can-deploy?service=api", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rr.Code)
		}
	})

	t.Run("no contracts returns ok", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/can-deploy?service=api&version=1.0.0", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("with environment", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/can-deploy?service=api&version=1.0.0&environment=prod", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
	})
}

// --- Matrix ---

func TestHandleMatrix(t *testing.T) {
	t.Run("empty matrix", func(t *testing.T) {
		srv := newTestServer(t)
		req := httptest.NewRequest(http.MethodGet, "/matrix", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("with contracts", func(t *testing.T) {
		srv := newTestServer(t)
		publishContract(t, srv, validContract())

		req := httptest.NewRequest(http.MethodGet, "/matrix", http.NoBody)
		rr := httptest.NewRecorder()
		srv.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rr.Code)
		}
	})
}

// --- Validate Status ---

func TestValidateStatus(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"draft", false},
		{"published", false},
		{"deprecated", false},
		{"", true},
		{"unknown", true},
		{"DRAFT", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := validateStatus(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStatus(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// --- WithLogger ---

func TestWithLogger(t *testing.T) {
	srv := newTestServer(t)
	result := srv.WithLogger(log.Nop())
	if result != srv {
		t.Error("WithLogger should return same server")
	}
}

// --- End-to-end ---

func TestEndToEndFlow(t *testing.T) {
	srv := newTestServer(t)

	// 1. Publish
	c := validContract()
	publishContract(t, srv, c)

	// 2. Get
	req := httptest.NewRequest(http.MethodGet, "/contracts/"+c.Metadata.ID, http.NoBody)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get: status = %d", rr.Code)
	}

	// 3. Tag
	tagBody, err := json.Marshal(map[string][]string{"tags": {"env:prod"}})
	if err != nil {
		t.Fatalf("marshal tags: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/contracts/"+c.Metadata.ID+"/tags", bytes.NewReader(tagBody))
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("tag: status = %d", rr.Code)
	}

	// 4. Record verification
	vr := &brokerapi.VerificationResult{
		ContractID:      c.Metadata.ID,
		ContractVersion: c.Metadata.Version,
		Provider:        brokerapi.ServiceVersion{Name: "provider-a", Version: "2.0.0"},
		Consumer:        brokerapi.ServiceVersion{Name: "consumer-a", Version: "1.0.0"},
	}
	recordVerification(t, srv, vr)

	// 5. Can-deploy
	req = httptest.NewRequest(http.MethodGet, "/can-deploy?service=provider-a&version=2.0.0", http.NoBody)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("can-deploy: status = %d: %s", rr.Code, rr.Body.String())
	}

	// 6. Matrix
	req = httptest.NewRequest(http.MethodGet, "/matrix", http.NoBody)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("matrix: status = %d", rr.Code)
	}

	// 7. Health
	req = httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("health: status = %d", rr.Code)
	}
}
