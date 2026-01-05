package brokerapi

import (
	"context"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
)

func newTestDeployService(t *testing.T) (*DeployService, *ContractService, *VerificationService) {
	t.Helper()
	backend, err := storage.NewFilesystemBackend(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}
	contracts := NewContractService(backend)
	verifications := NewVerificationService(backend, contracts)
	deploy := NewDeployService(contracts, verifications)
	return deploy, contracts, verifications
}

// hasReasonWithStatus checks if the deploy result contains a reason with the specified status.
func hasReasonWithStatus(result *CanDeployResult, status string) bool {
	for _, reason := range result.Reasons {
		if reason.Status == status {
			return true
		}
	}
	return false
}

// hasReasonForConsumer checks if the deploy result contains a reason for the specified consumer.
func hasReasonForConsumer(result *CanDeployResult, consumer string) bool {
	for _, reason := range result.Reasons {
		if reason.Consumer == consumer {
			return true
		}
	}
	return false
}

func TestDeployService_CanDeploy_Verified(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// Setup: Create a contract where "api-service" is the provider
	c := createTestContract("frontend", "api-service", "1.0.0")
	c.Metadata.Tags = []string{"production"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record a successful verification
	vr := createTestVerificationResult(c.Metadata.ID, "2.0.0", true)
	vr.Provider.Name = "api-service"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	result, err := deploySvc.CanDeploy(ctx, "api-service", "2.0.0", "production")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}

	if !result.OK {
		t.Errorf("expected OK=true, got reasons: %+v", result.Reasons)
	}
	if result.Service != "api-service" {
		t.Errorf("Service = %q, want api-service", result.Service)
	}
	if result.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", result.Version)
	}
}

func TestDeployService_CanDeploy_Missing(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	// Setup: Create a contract where "api-service" is the provider
	c := createTestContract("frontend", "api-service", "1.0.0")
	c.Metadata.Tags = []string{"production"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// No verification recorded - should report missing
	result, err := deploySvc.CanDeploy(ctx, "api-service", "3.0.0", "production")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}

	if result.OK {
		t.Error("expected OK=false when verification is missing")
	}
	if !hasReasonWithStatus(result, "missing") {
		t.Error("expected a 'missing' status reason")
	}
}

func TestDeployService_CanDeploy_Failed(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// Setup: Create a contract where "api-service" is the provider
	c := createTestContract("frontend", "api-service", "1.0.0")
	c.Metadata.Tags = []string{"production"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record a failed verification
	vr := createTestVerificationResult(c.Metadata.ID, "4.0.0", false)
	vr.Provider.Name = "api-service"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	result, err := deploySvc.CanDeploy(ctx, "api-service", "4.0.0", "production")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}

	if result.OK {
		t.Error("expected OK=false when verification failed")
	}
	if !hasReasonWithStatus(result, "failed") {
		t.Error("expected a 'failed' status reason")
	}
}

func TestDeployService_CanDeploy_NoContracts(t *testing.T) {
	ctx := context.Background()
	deploySvc, _, _ := newTestDeployService(t)

	// Service with no contracts should be deployable
	result, err := deploySvc.CanDeploy(ctx, "unrelated-service", "1.0.0", "staging")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}

	if !result.OK {
		t.Error("expected OK=true for service with no contracts")
	}
	if len(result.Reasons) != 0 {
		t.Errorf("expected no reasons, got %d", len(result.Reasons))
	}
}

func TestDeployService_CanDeploy_DeprecatedSkipped(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// Setup: Create a published contract
	c := createTestContract("frontend", "api-service", "1.0.0")
	c.Metadata.Tags = []string{"production"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record verification for the published contract
	vr := createTestVerificationResult(c.Metadata.ID, "2.0.0", true)
	vr.Provider.Name = "api-service"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	// Create a deprecated contract
	deprecatedContract := createTestContract("old-frontend", "api-service", "0.9.0")
	deprecatedContract.Metadata.Tags = []string{"production"}
	deprecatedContract.Metadata.Status = contract.StatusDeprecated
	if _, err := contractSvc.Publish(ctx, deprecatedContract); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Deploy should succeed and skip the deprecated contract
	result, err := deploySvc.CanDeploy(ctx, "api-service", "2.0.0", "production")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}

	// Should not have reason for the deprecated contract
	if hasReasonForConsumer(result, "old-frontend") {
		t.Error("deprecated contract should be skipped")
	}
}

func TestDeployService_CanDeploy_AsConsumer(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// Setup: Create a contract where "frontend" is the consumer
	c := createTestContract("frontend", "backend-api", "1.0.0")
	c.Metadata.Tags = []string{"production"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	t.Run("consumer deploy checks provider verification", func(t *testing.T) {
		// Record provider verification - the provider must verify against the contract
		vr := &VerificationResult{
			ContractID:      c.Metadata.ID,
			ContractVersion: c.Metadata.Version,
			Provider: ServiceVersion{
				Name:    "backend-api",
				Version: "1.0.0",
			},
			Consumer: ServiceVersion{
				Name:    "frontend",
				Version: "2.0.0",
			},
			Success: true,
			InteractionResults: []InteractionResult{
				{ID: "test", Success: true},
			},
		}
		if err := verificationSvc.RecordResult(ctx, vr); err != nil {
			t.Fatalf("setup RecordResult failed: %v", err)
		}

		result, err := deploySvc.CanDeploy(ctx, "frontend", "2.0.0", "production")
		if err != nil {
			t.Fatalf("CanDeploy failed: %v", err)
		}

		// Frontend as consumer should check that backend has verified
		if !result.OK {
			t.Errorf("expected OK=true, got reasons: %+v", result.Reasons)
		}
	})
}

func TestDeployReason(t *testing.T) {
	reason := DeployReason{
		ContractID: "contract-123",
		Status:     "verified",
	}

	if reason.ContractID != "contract-123" {
		t.Errorf("ContractID = %q, want contract-123", reason.ContractID)
	}
	if reason.Status != "verified" {
		t.Errorf("Status = %q, want verified", reason.Status)
	}
}

func TestCanDeployResult(t *testing.T) {
	result := CanDeployResult{
		OK: true,
		Reasons: []DeployReason{
			{Status: "verified"},
			{Status: "verified"},
		},
	}

	if !result.OK {
		t.Error("expected OK=true")
	}
	if len(result.Reasons) != 2 {
		t.Errorf("got %d reasons, want 2", len(result.Reasons))
	}
}
