package brokerapi

import (
	"context"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
)

func newTestVerificationService(t *testing.T) *VerificationService {
	t.Helper()
	backend, err := storage.NewFilesystemBackend(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test backend: %v", err)
	}
	contracts := NewContractService(backend)
	return NewVerificationService(backend, contracts)
}

func createTestVerificationResult(contractID, providerVersion string, success bool) *VerificationResult {
	return &VerificationResult{
		ContractID:      contractID,
		ContractVersion: "1.0.0",
		Provider: ServiceVersion{
			Name:    "test-provider",
			Version: providerVersion,
		},
		Consumer: ServiceVersion{
			Name:    "test-consumer",
			Version: "1.0.0",
		},
		VerifiedAt: time.Now().UTC(),
		DurationMS: 100,
		InteractionResults: []InteractionResult{
			{
				ID:          "interaction-1",
				Description: "Test interaction",
				Success:     success,
				DurationMS:  50,
			},
		},
	}
}

func TestVerificationService_RecordResult(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("record successful verification", func(t *testing.T) {
		result := createTestVerificationResult("contract-1", "1.0.0", true)

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		// Verify summary was computed
		if result.Summary.Total != 1 {
			t.Errorf("Summary.Total = %d, want 1", result.Summary.Total)
		}
		if result.Summary.Passed != 1 {
			t.Errorf("Summary.Passed = %d, want 1", result.Summary.Passed)
		}
		if !result.Success {
			t.Error("expected Success to be true")
		}
	})

	t.Run("record failed verification", func(t *testing.T) {
		result := createTestVerificationResult("contract-2", "1.0.0", false)
		result.InteractionResults[0].Errors = []InteractionError{
			{
				Path:     "$.response.status",
				Expected: "200",
				Actual:   "500",
				Message:  "status mismatch",
			},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Failed != 1 {
			t.Errorf("Summary.Failed = %d, want 1", result.Summary.Failed)
		}
		if result.Success {
			t.Error("expected Success to be false")
		}
	})

	t.Run("missing contract ID rejected", func(t *testing.T) {
		result := createTestVerificationResult("", "1.0.0", true)

		err := svc.RecordResult(ctx, result)
		if err == nil {
			t.Error("expected error for missing contract ID")
		}
	})

	t.Run("missing provider info rejected", func(t *testing.T) {
		result := createTestVerificationResult("contract-3", "1.0.0", true)
		result.Provider.Name = ""

		err := svc.RecordResult(ctx, result)
		if err == nil {
			t.Error("expected error for missing provider name")
		}
	})

	t.Run("auto-sets verified_at if zero", func(t *testing.T) {
		result := createTestVerificationResult("contract-4", "1.0.0", true)
		result.VerifiedAt = time.Time{}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.VerifiedAt.IsZero() {
			t.Error("expected VerifiedAt to be set")
		}
	})
}

func TestVerificationService_GetResult(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	// Setup: record a result
	result := createTestVerificationResult("contract-get", "2.0.0", true)
	if err := svc.RecordResult(ctx, result); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	t.Run("get existing result", func(t *testing.T) {
		got, err := svc.GetResult(ctx, "contract-get", "2.0.0")
		if err != nil {
			t.Fatalf("GetResult failed: %v", err)
		}

		if got.ContractID != "contract-get" {
			t.Errorf("ContractID = %q, want contract-get", got.ContractID)
		}
		if got.Provider.Version != "2.0.0" {
			t.Errorf("Provider.Version = %q, want 2.0.0", got.Provider.Version)
		}
	})

	t.Run("get non-existent result", func(t *testing.T) {
		_, err := svc.GetResult(ctx, "nonexistent", "1.0.0")
		if err != storage.ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("get with wrong version", func(t *testing.T) {
		_, err := svc.GetResult(ctx, "contract-get", "9.9.9")
		if err != storage.ErrNotFound {
			t.Errorf("expected ErrNotFound for wrong version, got %v", err)
		}
	})
}

func TestVerificationService_ListResults(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	// Setup: record multiple results for the same contract
	contractID := "contract-list"
	versions := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, v := range versions {
		result := createTestVerificationResult(contractID, v, v != "1.1.0") // 1.1.0 fails
		if err := svc.RecordResult(ctx, result); err != nil {
			t.Fatalf("setup RecordResult failed: %v", err)
		}
	}

	// Also record for a different contract
	otherResult := createTestVerificationResult("other-contract", "1.0.0", true)
	if err := svc.RecordResult(ctx, otherResult); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	t.Run("list results for contract", func(t *testing.T) {
		results, err := svc.ListResults(ctx, contractID)
		if err != nil {
			t.Fatalf("ListResults failed: %v", err)
		}

		if len(results) != 3 {
			t.Errorf("got %d results, want 3", len(results))
		}
	})

	t.Run("list results for non-existent contract", func(t *testing.T) {
		results, err := svc.ListResults(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("ListResults failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("got %d results, want 0", len(results))
		}
	})
}

func TestVerificationResult_Summary(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("mixed results compute correctly", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:      "contract-summary",
			ContractVersion: "1.0.0",
			Provider:        ServiceVersion{Name: "provider", Version: "1.0.0"},
			Consumer:        ServiceVersion{Name: "consumer", Version: "1.0.0"},
			InteractionResults: []InteractionResult{
				{ID: "1", Success: true},
				{ID: "2", Success: true},
				{ID: "3", Success: false},
				{ID: "4", Success: true},
				{ID: "5", Success: false},
			},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Total != 5 {
			t.Errorf("Summary.Total = %d, want 5", result.Summary.Total)
		}
		if result.Summary.Passed != 3 {
			t.Errorf("Summary.Passed = %d, want 3", result.Summary.Passed)
		}
		if result.Summary.Failed != 2 {
			t.Errorf("Summary.Failed = %d, want 2", result.Summary.Failed)
		}
		if result.Success {
			t.Error("expected Success to be false when any interaction fails")
		}
	})
}
