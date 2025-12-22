package storage

import (
	"testing"
)

func TestKeyBuilder(t *testing.T) {
	kb := NewKeyBuilder("/v1")

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "Contract",
			method:   func() string { return kb.Contract("consumer", "provider", "1.0.0") },
			expected: "/v1/contracts/consumer/provider/1.0.0.json",
		},
		{
			name:     "ContractByID",
			method:   func() string { return kb.ContractByID("abc-123") },
			expected: "/v1/contracts/by-id/abc-123.json",
		},
		{
			name:     "VerificationResult",
			method:   func() string { return kb.VerificationResult("contract-id", "2.0.0") },
			expected: "/v1/verifications/contract-id/2.0.0.json",
		},
		{
			name:     "LatestContract",
			method:   func() string { return kb.LatestContract("consumer", "provider") },
			expected: "/v1/contracts/consumer/provider/latest.json",
		},
		{
			name:     "TaggedContracts",
			method:   func() string { return kb.TaggedContracts("production") },
			expected: "/v1/tags/production/",
		},
		{
			name:     "Webhook",
			method:   func() string { return kb.Webhook("webhook-id") },
			expected: "/v1/webhooks/webhook-id.json",
		},
		{
			name:     "AllContracts",
			method:   func() string { return kb.AllContracts() },
			expected: "/v1/contracts/",
		},
		{
			name:     "AllVerifications",
			method:   func() string { return kb.AllVerifications() },
			expected: "/v1/verifications/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestKeyBuilder_EmptyPrefix(t *testing.T) {
	kb := NewKeyBuilder("")

	result := kb.Contract("consumer", "provider", "1.0.0")
	expected := "/contracts/consumer/provider/1.0.0.json"

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
