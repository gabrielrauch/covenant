package http_example

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	"github.com/gabrielrauch/covenant/pkg/provider"
)

// OrdersAPI is a simple mock provider for testing.
type OrdersAPI struct {
	orders map[string]*Order
}

// NewOrdersAPI creates a new OrdersAPI.
func NewOrdersAPI() *OrdersAPI {
	return &OrdersAPI{
		orders: make(map[string]*Order),
	}
}

// ServeHTTP implements http.Handler.
func (api *OrdersAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && len(r.URL.Path) > len("/orders/"):
		id := r.URL.Path[len("/orders/"):]
		api.handleGetOrder(w, r, id)
	case r.Method == "POST" && r.URL.Path == "/orders":
		api.handleCreateOrder(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (api *OrdersAPI) handleGetOrder(w http.ResponseWriter, r *http.Request, id string) {
	order, ok := api.orders[id]
	if !ok {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func (api *OrdersAPI) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	// Parse request body to get items
	var req struct {
		Items []map[string]any `json:"items"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Build response with items echoed back
	response := map[string]any{
		"id":     "new-order-id",
		"status": "created",
		"items":  req.Items,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// AddOrder adds an order to the API (for state setup).
func (api *OrdersAPI) AddOrder(order *Order) {
	api.orders[order.ID] = order
}

// TestProviderVerification demonstrates provider verification.
func TestProviderVerification(t *testing.T) {
	// Create the provider API
	api := NewOrdersAPI()

	// Start test server
	server := httptest.NewServer(api)
	defer server.Close()

	// Load contracts from directory
	contractFiles, _ := filepath.Glob("./contracts/*.json")
	if len(contractFiles) == 0 {
		t.Skip("No contract files found - run consumer tests first")
	}

	var contracts []*contract.Contract
	for _, file := range contractFiles {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			t.Logf("Skipping %s: %v", file, err)
			continue
		}
		contracts = append(contracts, c)
	}

	if len(contracts) == 0 {
		t.Skip("No valid contracts found")
	}

	// Create verifier
	verifier := provider.NewVerifier("orders-api", "1.0.0", server.URL)

	// Register state handlers
	verifier.OnProviderState("order 123 exists", func(state contract.ProviderState) error {
		// Extract parameters from state
		id, _ := state.Params["id"].(string)
		status, _ := state.Params["status"].(string)
		total, _ := state.Params["total"].(float64)

		api.AddOrder(&Order{
			ID:     id,
			Status: status,
			Total:  total,
		})
		return nil
	})

	// Add contracts and verify
	verifier.AddContracts(contracts)
	verifier.VerifyWithTesting(t)
}
