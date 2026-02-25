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

// --- Additional tests for uncovered branches ---

func TestVerificationService_RecordResult_ZeroVerifiedAt(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("zero VerifiedAt gets auto-populated with current time", func(t *testing.T) {
		result := createTestVerificationResult("contract-zero-time", "1.0.0", true)
		result.VerifiedAt = time.Time{} // zero value

		before := time.Now().UTC()
		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}
		after := time.Now().UTC()

		if result.VerifiedAt.IsZero() {
			t.Error("expected VerifiedAt to be auto-populated")
		}
		if result.VerifiedAt.Before(before) || result.VerifiedAt.After(after) {
			t.Errorf("VerifiedAt %v not in expected range [%v, %v]",
				result.VerifiedAt, before, after)
		}
	})

	t.Run("non-zero VerifiedAt is preserved", func(t *testing.T) {
		fixedTime := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
		result := createTestVerificationResult("contract-fixed-time", "1.0.0", true)
		result.VerifiedAt = fixedTime

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if !result.VerifiedAt.Equal(fixedTime) {
			t.Errorf("VerifiedAt = %v, want %v", result.VerifiedAt, fixedTime)
		}
	})
}

func TestVerificationService_RecordResult_ProviderVersionEdgeCases(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("missing provider version rejected", func(t *testing.T) {
		result := createTestVerificationResult("contract-no-pv", "", true)

		err := svc.RecordResult(ctx, result)
		if err == nil {
			t.Error("expected error for missing provider version")
		}
	})

	t.Run("missing both provider name and version rejected", func(t *testing.T) {
		result := createTestVerificationResult("contract-no-provider", "1.0.0", true)
		result.Provider = ServiceVersion{Name: "", Version: ""}

		err := svc.RecordResult(ctx, result)
		if err == nil {
			t.Error("expected error for missing provider name and version")
		}
	})
}

func TestVerificationService_RecordResult_EmptyInteractions(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("empty interaction results computes zero summary", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:         "contract-empty-ir",
			ContractVersion:    "1.0.0",
			Provider:           ServiceVersion{Name: "provider", Version: "1.0.0"},
			Consumer:           ServiceVersion{Name: "consumer", Version: "1.0.0"},
			VerifiedAt:         time.Now().UTC(),
			InteractionResults: []InteractionResult{},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Total != 0 {
			t.Errorf("Summary.Total = %d, want 0", result.Summary.Total)
		}
		if result.Summary.Passed != 0 {
			t.Errorf("Summary.Passed = %d, want 0", result.Summary.Passed)
		}
		if result.Summary.Failed != 0 {
			t.Errorf("Summary.Failed = %d, want 0", result.Summary.Failed)
		}
		if !result.Success {
			t.Error("expected Success to be true with no failures")
		}
	})

	t.Run("nil interaction results computes zero summary", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:         "contract-nil-ir",
			ContractVersion:    "1.0.0",
			Provider:           ServiceVersion{Name: "provider", Version: "1.0.0"},
			Consumer:           ServiceVersion{Name: "consumer", Version: "1.0.0"},
			VerifiedAt:         time.Now().UTC(),
			InteractionResults: nil,
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Total != 0 {
			t.Errorf("Summary.Total = %d, want 0", result.Summary.Total)
		}
		if !result.Success {
			t.Error("expected Success to be true with nil interactions")
		}
	})
}

func TestVerificationService_RecordResult_AllPassAllFail(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("all interactions pass", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:      "contract-all-pass",
			ContractVersion: "1.0.0",
			Provider:        ServiceVersion{Name: "provider", Version: "1.0.0"},
			Consumer:        ServiceVersion{Name: "consumer", Version: "1.0.0"},
			VerifiedAt:      time.Now().UTC(),
			InteractionResults: []InteractionResult{
				{ID: "1", Success: true},
				{ID: "2", Success: true},
				{ID: "3", Success: true},
			},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Total != 3 {
			t.Errorf("Summary.Total = %d, want 3", result.Summary.Total)
		}
		if result.Summary.Passed != 3 {
			t.Errorf("Summary.Passed = %d, want 3", result.Summary.Passed)
		}
		if result.Summary.Failed != 0 {
			t.Errorf("Summary.Failed = %d, want 0", result.Summary.Failed)
		}
		if !result.Success {
			t.Error("expected Success to be true when all pass")
		}
	})

	t.Run("all interactions fail", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:      "contract-all-fail",
			ContractVersion: "1.0.0",
			Provider:        ServiceVersion{Name: "provider", Version: "2.0.0"},
			Consumer:        ServiceVersion{Name: "consumer", Version: "1.0.0"},
			VerifiedAt:      time.Now().UTC(),
			InteractionResults: []InteractionResult{
				{ID: "1", Success: false, Errors: []InteractionError{{Message: "err1"}}},
				{ID: "2", Success: false, Errors: []InteractionError{{Message: "err2"}}},
			},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		if result.Summary.Passed != 0 {
			t.Errorf("Summary.Passed = %d, want 0", result.Summary.Passed)
		}
		if result.Summary.Failed != 2 {
			t.Errorf("Summary.Failed = %d, want 2", result.Summary.Failed)
		}
		if result.Success {
			t.Error("expected Success to be false when all fail")
		}
	})
}

func TestVerificationService_RecordResult_OverwritesExisting(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("recording with same key overwrites previous result", func(t *testing.T) {
		// Record first result (failed)
		result1 := createTestVerificationResult("contract-overwrite", "1.0.0", false)
		if err := svc.RecordResult(ctx, result1); err != nil {
			t.Fatalf("first RecordResult failed: %v", err)
		}

		// Record second result with same contract+version (success)
		result2 := createTestVerificationResult("contract-overwrite", "1.0.0", true)
		if err := svc.RecordResult(ctx, result2); err != nil {
			t.Fatalf("second RecordResult failed: %v", err)
		}

		// Retrieve should get the latest
		got, err := svc.GetResult(ctx, "contract-overwrite", "1.0.0")
		if err != nil {
			t.Fatalf("GetResult failed: %v", err)
		}
		if !got.Success {
			t.Error("expected overwritten result to be successful")
		}
	})
}

func TestVerificationService_GetResult_CorruptedData(t *testing.T) {
	t.Run("corrupted JSON returns parse error", func(t *testing.T) {
		backend, err := storage.NewFilesystemBackend(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		contracts := NewContractService(backend)
		svc := NewVerificationService(backend, contracts)
		ctx := context.Background()

		// Write corrupted data directly to storage
		keys := storage.NewKeyBuilder("")
		key := keys.VerificationResult("contract-corrupt", "1.0.0")
		if saveErr := backend.Save(ctx, key, []byte("this is not valid json{{")); saveErr != nil {
			t.Fatalf("failed to save corrupted data: %v", saveErr)
		}

		_, err = svc.GetResult(ctx, "contract-corrupt", "1.0.0")
		if err == nil {
			t.Error("expected error for corrupted JSON")
		}
	})
}

func TestVerificationService_ListResults_CorruptedJSON(t *testing.T) {
	t.Run("corrupted entries are skipped without error", func(t *testing.T) {
		backend, err := storage.NewFilesystemBackend(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		contracts := NewContractService(backend)
		svc := NewVerificationService(backend, contracts)
		ctx := context.Background()

		contractID := "contract-mixed-data"

		// Record one valid result
		validResult := createTestVerificationResult(contractID, "1.0.0", true)
		if recordErr := svc.RecordResult(ctx, validResult); recordErr != nil {
			t.Fatalf("RecordResult failed: %v", recordErr)
		}

		// Write corrupted data directly for another "version"
		keys := storage.NewKeyBuilder("")
		corruptKey := keys.VerificationResult(contractID, "2.0.0")
		if saveErr := backend.Save(ctx, corruptKey, []byte("{invalid json")); saveErr != nil {
			t.Fatalf("failed to save corrupted data: %v", saveErr)
		}

		// ListResults should return only the valid result, skipping corrupted
		results, err := svc.ListResults(ctx, contractID)
		if err != nil {
			t.Fatalf("ListResults failed: %v", err)
		}

		if len(results) != 1 {
			t.Errorf("got %d results, want 1 (corrupted should be skipped)", len(results))
		}
		if len(results) > 0 && results[0].Provider.Version != "1.0.0" {
			t.Errorf("expected valid result version 1.0.0, got %s", results[0].Provider.Version)
		}
	})

	t.Run("all corrupted entries returns empty list", func(t *testing.T) {
		backend, err := storage.NewFilesystemBackend(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		contracts := NewContractService(backend)
		svc := NewVerificationService(backend, contracts)
		ctx := context.Background()

		contractID := "contract-all-corrupt"

		// Write multiple corrupted entries
		keys := storage.NewKeyBuilder("")
		for _, ver := range []string{"1.0.0", "2.0.0", "3.0.0"} {
			key := keys.VerificationResult(contractID, ver)
			if saveErr := backend.Save(ctx, key, []byte("not json "+ver)); saveErr != nil {
				t.Fatalf("failed to save corrupted data: %v", saveErr)
			}
		}

		results, err := svc.ListResults(ctx, contractID)
		if err != nil {
			t.Fatalf("ListResults failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("got %d results, want 0 (all corrupted)", len(results))
		}
	})
}

func TestVerificationService_ListResults_EmptyContract(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("empty contract ID returns empty list", func(t *testing.T) {
		results, err := svc.ListResults(ctx, "")
		if err != nil {
			t.Fatalf("ListResults failed: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("got %d results, want 0 for empty contract ID", len(results))
		}
	})
}

//nolint:gocyclo // test function
func TestVerificationService_GetResult_RoundTrip(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("recorded result can be retrieved with all fields intact", func(t *testing.T) {
		original := &VerificationResult{
			ContractID:      "contract-roundtrip",
			ContractVersion: "3.0.0",
			Provider:        ServiceVersion{Name: "my-provider", Version: "5.0.0"},
			Consumer:        ServiceVersion{Name: "my-consumer", Version: "2.0.0"},
			VerifiedAt:      time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			DurationMS:      250,
			InteractionResults: []InteractionResult{
				{
					ID:          "int-1",
					Description: "create order",
					Success:     true,
					DurationMS:  100,
				},
				{
					ID:          "int-2",
					Description: "get order",
					Success:     false,
					DurationMS:  150,
					Errors: []InteractionError{
						{
							Path:     "$.response.status",
							Rule:     "exact",
							Expected: "200",
							Actual:   "404",
							Message:  "status code mismatch",
						},
					},
				},
			},
		}

		if err := svc.RecordResult(ctx, original); err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		got, err := svc.GetResult(ctx, "contract-roundtrip", "5.0.0")
		if err != nil {
			t.Fatalf("GetResult failed: %v", err)
		}

		// Verify key fields
		if got.ContractID != original.ContractID {
			t.Errorf("ContractID = %q, want %q", got.ContractID, original.ContractID)
		}
		if got.ContractVersion != original.ContractVersion {
			t.Errorf("ContractVersion = %q, want %q", got.ContractVersion, original.ContractVersion)
		}
		if got.Provider.Name != original.Provider.Name {
			t.Errorf("Provider.Name = %q, want %q", got.Provider.Name, original.Provider.Name)
		}
		if got.Consumer.Name != original.Consumer.Name {
			t.Errorf("Consumer.Name = %q, want %q", got.Consumer.Name, original.Consumer.Name)
		}
		if got.DurationMS != original.DurationMS {
			t.Errorf("DurationMS = %d, want %d", got.DurationMS, original.DurationMS)
		}
		if !got.VerifiedAt.Equal(original.VerifiedAt) {
			t.Errorf("VerifiedAt = %v, want %v", got.VerifiedAt, original.VerifiedAt)
		}

		// Verify computed summary
		if got.Summary.Total != 2 {
			t.Errorf("Summary.Total = %d, want 2", got.Summary.Total)
		}
		if got.Summary.Passed != 1 {
			t.Errorf("Summary.Passed = %d, want 1", got.Summary.Passed)
		}
		if got.Summary.Failed != 1 {
			t.Errorf("Summary.Failed = %d, want 1", got.Summary.Failed)
		}
		if got.Success {
			t.Error("expected Success to be false (one failure)")
		}

		// Verify interaction results
		if len(got.InteractionResults) != 2 {
			t.Fatalf("InteractionResults length = %d, want 2", len(got.InteractionResults))
		}
		if got.InteractionResults[1].ID != "int-2" {
			t.Errorf("InteractionResults[1].ID = %q, want int-2", got.InteractionResults[1].ID)
		}
		if len(got.InteractionResults[1].Errors) != 1 {
			t.Fatalf("InteractionResults[1].Errors length = %d, want 1", len(got.InteractionResults[1].Errors))
		}
		if got.InteractionResults[1].Errors[0].Rule != "exact" {
			t.Errorf("Error rule = %q, want exact", got.InteractionResults[1].Errors[0].Rule)
		}
	})
}

func TestVerificationService_RecordResult_SummaryOverwritesPreviousValues(t *testing.T) {
	ctx := context.Background()
	svc := newTestVerificationService(t)

	t.Run("summary is recomputed even if pre-set", func(t *testing.T) {
		result := &VerificationResult{
			ContractID:      "contract-summary-overwrite",
			ContractVersion: "1.0.0",
			Provider:        ServiceVersion{Name: "provider", Version: "1.0.0"},
			Consumer:        ServiceVersion{Name: "consumer", Version: "1.0.0"},
			VerifiedAt:      time.Now().UTC(),
			// Pre-set incorrect summary values
			Summary: VerificationSummary{
				Total:  99,
				Passed: 50,
				Failed: 49,
			},
			Success: false, // Pre-set incorrect success
			InteractionResults: []InteractionResult{
				{ID: "1", Success: true},
			},
		}

		err := svc.RecordResult(ctx, result)
		if err != nil {
			t.Fatalf("RecordResult failed: %v", err)
		}

		// Summary should be recomputed
		if result.Summary.Total != 1 {
			t.Errorf("Summary.Total = %d, want 1 (should be recomputed)", result.Summary.Total)
		}
		if result.Summary.Passed != 1 {
			t.Errorf("Summary.Passed = %d, want 1 (should be recomputed)", result.Summary.Passed)
		}
		if result.Summary.Failed != 0 {
			t.Errorf("Summary.Failed = %d, want 0 (should be recomputed)", result.Summary.Failed)
		}
		if !result.Success {
			t.Error("expected Success to be recomputed to true")
		}
	})
}

func TestNewVerificationService(t *testing.T) {
	t.Run("creates service with correct dependencies", func(t *testing.T) {
		backend, err := storage.NewFilesystemBackend(t.TempDir())
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		contracts := NewContractService(backend)
		svc := NewVerificationService(backend, contracts)

		if svc == nil {
			t.Fatal("NewVerificationService returned nil")
		}
		if svc.storage == nil {
			t.Error("storage backend not set")
		}
		if svc.keys == nil {
			t.Error("key builder not set")
		}
		if svc.contracts == nil {
			t.Error("contracts service not set")
		}
	})
}
