// Package httpexample demonstrates consumer-driven contract testing with covenant.
package httpexample

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/consumer"
)

// OrderClient is the consumer's HTTP client for the Orders API.
type OrderClient struct {
	baseURL string
	client  *http.Client
}

// Order represents an order.
type Order struct {
	ID     string  `json:"id"`
	Status string  `json:"status"`
	Total  float64 `json:"total"`
}

// GetOrder fetches an order by ID.
func (c *OrderClient) GetOrder(ctx context.Context, id string) (*Order, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/orders/%s", c.baseURL, id), http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("unexpected status %d (failed to read body: %v)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, err
	}

	return &order, nil
}

// TestGetOrder_Consumer demonstrates a consumer test.
func TestGetOrder_Consumer(t *testing.T) {
	// Define the interaction
	getOrderInteraction := consumer.NewInteraction("get order by id").
		WithID("get-order-by-id").
		Given("order 123 exists", map[string]any{
			"id":     "123",
			"status": "pending",
			"total":  99.99,
		}).
		WithHTTPRequest("GET", "/orders/123").
		WillRespondWith(200).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{
			"id":     "123",
			"status": "pending",
			"total":  99.99,
		}).
		Build()

	// Run the test with automatic mock management
	consumer.RunTest(t, "checkout-ui", "orders-api",
		[]*consumer.Interaction{getOrderInteraction},
		func(mockURL string) {
			// Create client pointing to mock
			client := &OrderClient{
				baseURL: mockURL,
				client:  &http.Client{},
			}

			// Execute the actual consumer code
			order, err := client.GetOrder(context.Background(), "123")
			if err != nil {
				t.Fatalf("Failed to get order: %v", err)
			}

			// Verify the response
			if order.ID != "123" {
				t.Errorf("Expected ID 123, got %s", order.ID)
			}
			if order.Status != "pending" {
				t.Errorf("Expected status pending, got %s", order.Status)
			}
		},
		consumer.WithContractDir("./contracts"),
	)
}

// TestCreateOrder_Consumer demonstrates creating an order.
func TestCreateOrder_Consumer(t *testing.T) {
	createOrderInteraction := consumer.NewInteraction("create new order").
		WithID("create-order").
		WithHTTPRequest("POST", "/orders").
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{
			"items": []map[string]any{
				{"sku": "WIDGET-1", "quantity": 2},
			},
		}).
		WillRespondWith(201).
		WithHeader("Content-Type", "application/json").
		WithJSONBody(map[string]any{
			"id":     "new-order-id",
			"status": "created",
			"items": []map[string]any{
				{"sku": "WIDGET-1", "quantity": 2},
			},
		}).
		Build()

	// Add matching rules for dynamic values
	createOrderInteraction.WithMatchingRule("$.response.body.id", consumer.TypeMatcher())

	consumer.RunTest(t, "checkout-ui", "orders-api",
		[]*consumer.Interaction{createOrderInteraction},
		func(mockURL string) {
			// Build request body
			body := `{"items": [{"sku": "WIDGET-1", "quantity": 2}]}`

			// Test the actual HTTP call
			req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, mockURL+"/orders", strings.NewReader(body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to create order: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusCreated {
				t.Errorf("Expected status 201, got %d", resp.StatusCode)
			}
		},
	)
}
