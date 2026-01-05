package brokerapi

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
)

func newTestContractService(t *testing.T) *ContractService {
	t.Helper()
	backend, err := storage.NewFilesystemBackend(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}
	return NewContractService(backend)
}

func createTestContract(consumer, provider, version string) *contract.Contract {
	return &contract.Contract{
		Metadata: contract.Metadata{
			ID:       consumer + "-" + provider + "-" + version,
			Version:  version,
			Consumer: contract.Participant{Name: consumer, Version: "1.0.0"},
			Provider: contract.Participant{Name: provider, Version: "1.0.0"},
			Status:   contract.StatusDraft,
		},
		Interactions: []contract.Interaction{
			{
				ID:          "test-interaction",
				Description: "Test interaction",
				Protocol:    contract.ProtocolHTTP,
				Payload: contract.Payload{
					HTTP: &contract.HTTPPayload{
						Request: contract.HTTPRequest{
							Method: "GET",
							Path:   "/test",
						},
						Response: contract.HTTPResponse{
							Status: 200,
						},
					},
				},
			},
		},
	}
}

func TestContractService_Publish(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	t.Run("publish new contract", func(t *testing.T) {
		c := createTestContract("consumer-a", "provider-a", "1.0.0")

		result, err := svc.Publish(ctx, c)
		if err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		if !result.IsNew {
			t.Error("expected IsNew to be true for new contract")
		}
		if result.ID != c.Metadata.ID {
			t.Errorf("ID = %q, want %q", result.ID, c.Metadata.ID)
		}
		if result.Checksum == "" {
			t.Error("expected checksum to be computed")
		}
	})

	t.Run("update draft contract", func(t *testing.T) {
		c := createTestContract("consumer-b", "provider-b", "1.0.0")
		_, err := svc.Publish(ctx, c)
		if err != nil {
			t.Fatalf("initial Publish failed: %v", err)
		}

		// Modify and re-publish
		c.Interactions[0].Description = "Updated description"
		result, err := svc.Publish(ctx, c)
		if err != nil {
			t.Fatalf("update Publish failed: %v", err)
		}

		if result.IsNew {
			t.Error("expected IsNew to be false for update")
		}
	})

	t.Run("reject update of published contract", func(t *testing.T) {
		c := createTestContract("consumer-c", "provider-c", "1.0.0")
		c.Metadata.Status = contract.StatusPublished

		_, err := svc.Publish(ctx, c)
		if err != nil {
			t.Fatalf("initial Publish failed: %v", err)
		}

		// Try to update - should fail
		c.Interactions[0].Description = "Attempted update"
		_, err = svc.Publish(ctx, c)
		if err == nil {
			t.Error("expected error when updating published contract")
		}
	})

	t.Run("invalid contract rejected", func(t *testing.T) {
		c := &contract.Contract{} // Empty contract

		_, err := svc.Publish(ctx, c)
		if err == nil {
			t.Error("expected error for invalid contract")
		}
	})
}

func TestContractService_Get(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	// Setup: publish a contract
	c := createTestContract("consumer-get", "provider-get", "1.0.0")
	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("get by consumer/provider/version", func(t *testing.T) {
		got, err := svc.Get(ctx, "consumer-get", "provider-get", "1.0.0")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.Metadata.ID != c.Metadata.ID {
			t.Errorf("ID = %q, want %q", got.Metadata.ID, c.Metadata.ID)
		}
	})

	t.Run("get latest", func(t *testing.T) {
		got, err := svc.Get(ctx, "consumer-get", "provider-get", "latest")
		if err != nil {
			t.Fatalf("Get latest failed: %v", err)
		}
		if got.Metadata.ID != c.Metadata.ID {
			t.Errorf("ID = %q, want %q", got.Metadata.ID, c.Metadata.ID)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		_, err := svc.Get(ctx, "nonexistent", "nonexistent", "1.0.0")
		if err != storage.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestContractService_GetByID(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	c := createTestContract("consumer-id", "provider-id", "1.0.0")
	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("get by ID", func(t *testing.T) {
		got, err := svc.GetByID(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.Metadata.Version != c.Metadata.Version {
			t.Errorf("Version = %q, want %q", got.Metadata.Version, c.Metadata.Version)
		}
	})

	t.Run("get by ID not found", func(t *testing.T) {
		_, err := svc.GetByID(ctx, "nonexistent-id")
		if err != storage.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestContractService_List(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	// Setup: publish multiple contracts
	contracts := []*contract.Contract{
		createTestContract("consumer-list", "provider-a", "1.0.0"),
		createTestContract("consumer-list", "provider-b", "1.0.0"),
		createTestContract("consumer-other", "provider-a", "1.0.0"),
	}
	contracts[0].Metadata.Tags = []string{"production"}
	contracts[2].Metadata.Status = contract.StatusPublished

	for _, c := range contracts {
		if _, err := svc.Publish(ctx, c); err != nil {
			t.Fatalf("setup Publish failed: %v", err)
		}
	}

	t.Run("list all", func(t *testing.T) {
		summaries, err := svc.List(ctx, &ContractFilter{})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(summaries) != 3 {
			t.Errorf("got %d contracts, want 3", len(summaries))
		}
	})

	t.Run("filter by consumer", func(t *testing.T) {
		summaries, err := svc.List(ctx, &ContractFilter{Consumer: "consumer-list"})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(summaries) != 2 {
			t.Errorf("got %d contracts, want 2", len(summaries))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		summaries, err := svc.List(ctx, &ContractFilter{Status: contract.StatusPublished})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(summaries) != 1 {
			t.Errorf("got %d contracts, want 1", len(summaries))
		}
	})

	t.Run("filter by tag", func(t *testing.T) {
		summaries, err := svc.List(ctx, &ContractFilter{Tag: "production"})
		if err != nil {
			t.Fatalf("List failed: %v", err)
		}
		if len(summaries) != 1 {
			t.Errorf("got %d contracts, want 1", len(summaries))
		}
	})
}

func TestContractService_Tag(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	c := createTestContract("consumer-tag", "provider-tag", "1.0.0")
	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("add tags", func(t *testing.T) {
		err := svc.Tag(ctx, c.Metadata.ID, []string{"staging", "test"})
		if err != nil {
			t.Fatalf("Tag failed: %v", err)
		}

		got, err := svc.GetByID(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if len(got.Metadata.Tags) != 2 {
			t.Errorf("got %d tags, want 2", len(got.Metadata.Tags))
		}
	})

	t.Run("add duplicate tag is idempotent", func(t *testing.T) {
		err := svc.Tag(ctx, c.Metadata.ID, []string{"staging"})
		if err != nil {
			t.Fatalf("Tag failed: %v", err)
		}

		got, err := svc.GetByID(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if len(got.Metadata.Tags) != 2 {
			t.Errorf("got %d tags, want 2 (duplicate not added)", len(got.Metadata.Tags))
		}
	})

	t.Run("tag non-existent contract", func(t *testing.T) {
		err := svc.Tag(ctx, "nonexistent", []string{"tag"})
		if err == nil {
			t.Error("expected error for non-existent contract")
		}
	})
}

func TestContractService_Untag(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	c := createTestContract("consumer-untag", "provider-untag", "1.0.0")
	c.Metadata.Tags = []string{"production", "staging", "test"}
	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("remove tags", func(t *testing.T) {
		err := svc.Untag(ctx, c.Metadata.ID, []string{"staging", "test"})
		if err != nil {
			t.Fatalf("Untag failed: %v", err)
		}

		got, err := svc.GetByID(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if len(got.Metadata.Tags) != 1 {
			t.Errorf("got %d tags, want 1", len(got.Metadata.Tags))
		}
		if got.Metadata.Tags[0] != "production" {
			t.Errorf("remaining tag = %q, want production", got.Metadata.Tags[0])
		}
	})
}

func TestContractService_Deprecate(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	c := createTestContract("consumer-dep", "provider-dep", "1.0.0")
	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("deprecate contract", func(t *testing.T) {
		err := svc.Deprecate(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("Deprecate failed: %v", err)
		}

		got, err := svc.GetByID(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("GetByID failed: %v", err)
		}
		if got.Metadata.Status != contract.StatusDeprecated {
			t.Errorf("status = %q, want deprecated", got.Metadata.Status)
		}
	})

	t.Run("deprecate already deprecated is idempotent", func(t *testing.T) {
		err := svc.Deprecate(ctx, c.Metadata.ID)
		if err != nil {
			t.Fatalf("Deprecate failed: %v", err)
		}
	})

	t.Run("deprecate non-existent contract", func(t *testing.T) {
		err := svc.Deprecate(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for non-existent contract")
		}
	})
}

func TestContractSummary(t *testing.T) {
	ctx := context.Background()
	svc := newTestContractService(t)

	c := createTestContract("consumer-sum", "provider-sum", "2.0.0")
	c.Metadata.Tags = []string{"prod"}
	c.Metadata.CreatedAt = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := svc.Publish(ctx, c)
	if err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	summaries, err := svc.List(ctx, &ContractFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("got %d summaries, want 1", len(summaries))
	}

	sum := summaries[0]
	if sum.Consumer != "consumer-sum" {
		t.Errorf("Consumer = %q, want consumer-sum", sum.Consumer)
	}
	if sum.Provider != "provider-sum" {
		t.Errorf("Provider = %q, want provider-sum", sum.Provider)
	}
	if sum.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", sum.Version)
	}
	if sum.Interactions != 1 {
		t.Errorf("Interactions = %d, want 1", sum.Interactions)
	}
	if len(sum.Tags) != 1 || sum.Tags[0] != "prod" {
		t.Errorf("Tags = %v, want [prod]", sum.Tags)
	}
}
