package brokerapi

import (
	"context"
	"testing"
	"time"

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

// ---------------------------------------------------------------------------
// Cache tests
// ---------------------------------------------------------------------------

func TestCacheGetMiss(t *testing.T) {
	c := newMatrixCache()
	got, ok := c.get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
	if got != nil {
		t.Errorf("expected nil matrix on miss, got %v", got)
	}
}

func TestCacheSetAndHit(t *testing.T) {
	c := newMatrixCache()
	m := &Matrix{Services: []string{"a", "b"}}
	c.set("key1", m, 1*time.Minute)

	got, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit after set")
	}
	if got != m {
		t.Error("expected same matrix pointer from cache hit")
	}
}

func TestCacheExpiration(t *testing.T) {
	c := newMatrixCache()
	m := &Matrix{Services: []string{"x"}}
	// Use a TTL that is already expired.
	c.set("exp", m, -1*time.Second)

	got, ok := c.get("exp")
	if ok {
		t.Error("expected cache miss for expired entry")
	}
	if got != nil {
		t.Errorf("expected nil matrix for expired entry, got %v", got)
	}
}

func TestCacheDisabledWhenTTLZero(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)
	deploySvc = deploySvc.WithCacheTTL(0)

	// Publish a contract so GetMatrix has something to return.
	c := createTestContract("svc-a", "svc-b", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Call GetMatrix twice; with TTL=0, nothing should be cached.
	m1, err := deploySvc.GetMatrix(ctx, "", "")
	if err != nil {
		t.Fatalf("first GetMatrix failed: %v", err)
	}
	m2, err := deploySvc.GetMatrix(ctx, "", "")
	if err != nil {
		t.Fatalf("second GetMatrix failed: %v", err)
	}

	// Since caching is disabled, each call builds a fresh matrix.
	if m1 == m2 {
		t.Error("expected distinct matrix pointers when caching is disabled")
	}
}

func TestCacheHitReturnsSameMatrix(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)
	deploySvc = deploySvc.WithCacheTTL(5 * time.Minute)

	c := createTestContract("cache-c", "cache-p", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	m1, err := deploySvc.GetMatrix(ctx, "", "")
	if err != nil {
		t.Fatalf("first GetMatrix failed: %v", err)
	}
	m2, err := deploySvc.GetMatrix(ctx, "", "")
	if err != nil {
		t.Fatalf("second GetMatrix failed: %v", err)
	}

	if m1 != m2 {
		t.Error("expected same matrix pointer on cache hit")
	}
}

// ---------------------------------------------------------------------------
// buildMatrixCell tests
// ---------------------------------------------------------------------------

func newTestDeployServiceWithBackend(t *testing.T) (*DeployService, *ContractService, *VerificationService) {
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

func TestBuildMatrixCell_SelfDependency(t *testing.T) {
	deploySvc, _, _ := newTestDeployServiceWithBackend(t)
	pairs := map[pairKey][]*ContractSummary{}

	cell := deploySvc.buildMatrixCell(context.Background(), "svc-x", "svc-x", pairs)

	if cell.Consumer != "svc-x" || cell.Provider != "svc-x" {
		t.Errorf("expected consumer=provider=svc-x, got consumer=%q provider=%q", cell.Consumer, cell.Provider)
	}
	if cell.ContractCount != 0 {
		t.Errorf("ContractCount = %d, want 0 for self-dependency", cell.ContractCount)
	}
	if cell.Verified {
		t.Error("expected Verified=false for self-dependency")
	}
	if cell.LastVerified != "" {
		t.Errorf("expected empty LastVerified for self-dependency, got %q", cell.LastVerified)
	}
}

func TestBuildMatrixCell_NoContracts(t *testing.T) {
	deploySvc, _, _ := newTestDeployServiceWithBackend(t)
	pairs := map[pairKey][]*ContractSummary{}

	cell := deploySvc.buildMatrixCell(context.Background(), "a", "b", pairs)

	if cell.ContractCount != 0 {
		t.Errorf("ContractCount = %d, want 0", cell.ContractCount)
	}
	if cell.Verified {
		t.Error("expected Verified=false when no contracts exist for pair")
	}
}

func TestBuildMatrixCell_AllVerified(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployServiceWithBackend(t)

	c := createTestContract("cons", "prov", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "1.0.0", true)
	vr.Provider.Name = "prov"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "cons",
		Provider: "prov",
		Status:   contract.StatusPublished,
	}
	pairs := map[pairKey][]*ContractSummary{
		{consumer: "cons", provider: "prov"}: {summary},
	}

	cell := deploySvc.buildMatrixCell(ctx, "cons", "prov", pairs)

	if cell.ContractCount != 1 {
		t.Errorf("ContractCount = %d, want 1", cell.ContractCount)
	}
	if !cell.Verified {
		t.Error("expected Verified=true when all contracts verified")
	}
	if cell.LastVerified == "" {
		t.Error("expected LastVerified to be set")
	}
}

func TestBuildMatrixCell_MixedVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployServiceWithBackend(t)

	// Contract 1: verified
	c1 := createTestContract("cons", "prov", "1.0.0")
	c1.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c1); err != nil {
		t.Fatalf("setup Publish c1 failed: %v", err)
	}
	vr1 := createTestVerificationResult(c1.Metadata.ID, "1.0.0", true)
	vr1.Provider.Name = "prov"
	if err := verificationSvc.RecordResult(ctx, vr1); err != nil {
		t.Fatalf("setup RecordResult vr1 failed: %v", err)
	}

	// Contract 2: no verification recorded (will show as not verified)
	c2 := createTestContract("cons", "prov", "2.0.0")
	c2.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c2); err != nil {
		t.Fatalf("setup Publish c2 failed: %v", err)
	}

	pairs := map[pairKey][]*ContractSummary{
		{consumer: "cons", provider: "prov"}: {
			{ID: c1.Metadata.ID, Consumer: "cons", Provider: "prov"},
			{ID: c2.Metadata.ID, Consumer: "cons", Provider: "prov"},
		},
	}

	cell := deploySvc.buildMatrixCell(ctx, "cons", "prov", pairs)

	if cell.ContractCount != 2 {
		t.Errorf("ContractCount = %d, want 2", cell.ContractCount)
	}
	if cell.Verified {
		t.Error("expected Verified=false when not all contracts are verified")
	}
}

// ---------------------------------------------------------------------------
// findLatestSuccessDate tests
// ---------------------------------------------------------------------------

func TestFindLatestSuccessDate_NoResults(t *testing.T) {
	hasSuccess, date := findLatestSuccessDate(nil)
	if hasSuccess {
		t.Error("expected hasSuccess=false for nil results")
	}
	if date != "" {
		t.Errorf("expected empty date, got %q", date)
	}

	hasSuccess2, date2 := findLatestSuccessDate([]VerificationResult{})
	if hasSuccess2 {
		t.Error("expected hasSuccess=false for empty results")
	}
	if date2 != "" {
		t.Errorf("expected empty date, got %q", date2)
	}
}

func TestFindLatestSuccessDate_NoSuccesses(t *testing.T) {
	results := []VerificationResult{
		{Success: false, VerifiedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Success: false, VerifiedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
	}

	hasSuccess, date := findLatestSuccessDate(results)
	if hasSuccess {
		t.Error("expected hasSuccess=false when all results failed")
	}
	if date != "" {
		t.Errorf("expected empty date, got %q", date)
	}
}

func TestFindLatestSuccessDate_MultipleResults(t *testing.T) {
	tests := []struct {
		name        string
		results     []VerificationResult
		wantSuccess bool
		wantDate    string
	}{
		{
			name: "single success",
			results: []VerificationResult{
				{Success: true, VerifiedAt: time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC)},
			},
			wantSuccess: true,
			wantDate:    "2025-03-15",
		},
		{
			name: "success among failures returns first success",
			results: []VerificationResult{
				{Success: false, VerifiedAt: time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)},
				{Success: true, VerifiedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)},
				{Success: true, VerifiedAt: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)},
			},
			wantSuccess: true,
			wantDate:    "2025-03-01",
		},
		{
			name: "first result is success",
			results: []VerificationResult{
				{Success: true, VerifiedAt: time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)},
				{Success: false, VerifiedAt: time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)},
			},
			wantSuccess: true,
			wantDate:    "2025-06-10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasSuccess, date := findLatestSuccessDate(tt.results)
			if hasSuccess != tt.wantSuccess {
				t.Errorf("hasSuccess = %v, want %v", hasSuccess, tt.wantSuccess)
			}
			if date != tt.wantDate {
				t.Errorf("date = %q, want %q", date, tt.wantDate)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractServices tests
// ---------------------------------------------------------------------------

func TestExtractServices(t *testing.T) {
	tests := []struct {
		name      string
		contracts []ContractSummary
		want      []string
	}{
		{
			name:      "empty contracts",
			contracts: nil,
			want:      []string{},
		},
		{
			name: "single contract",
			contracts: []ContractSummary{
				{Consumer: "frontend", Provider: "backend"},
			},
			want: []string{"backend", "frontend"},
		},
		{
			name: "deduplicates services",
			contracts: []ContractSummary{
				{Consumer: "frontend", Provider: "backend"},
				{Consumer: "frontend", Provider: "auth"},
			},
			want: []string{"auth", "backend", "frontend"},
		},
		{
			name: "sorted output",
			contracts: []ContractSummary{
				{Consumer: "zebra", Provider: "alpha"},
				{Consumer: "middle", Provider: "alpha"},
			},
			want: []string{"alpha", "middle", "zebra"},
		},
		{
			name: "consumer equals provider in different contracts",
			contracts: []ContractSummary{
				{Consumer: "a", Provider: "b"},
				{Consumer: "b", Provider: "c"},
			},
			want: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServices(tt.contracts)
			if len(got) != len(tt.want) {
				t.Fatalf("extractServices returned %d services, want %d: got=%v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("service[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// findLatestSuccessfulVerification tests
// ---------------------------------------------------------------------------

func TestFindLatestSuccessfulVerification(t *testing.T) {
	tests := []struct {
		name    string
		results []VerificationResult
		wantNil bool
		wantVer string
	}{
		{
			name:    "nil results returns nil",
			results: nil,
			wantNil: true,
		},
		{
			name:    "empty results returns nil",
			results: []VerificationResult{},
			wantNil: true,
		},
		{
			name: "all failed returns nil",
			results: []VerificationResult{
				{Success: false, Provider: ServiceVersion{Version: "1.0"}, VerifiedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
				{Success: false, Provider: ServiceVersion{Version: "2.0"}, VerifiedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
			},
			wantNil: true,
		},
		{
			name: "single success",
			results: []VerificationResult{
				{Success: true, Provider: ServiceVersion{Version: "1.0"}, VerifiedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
			},
			wantVer: "1.0",
		},
		{
			name: "latest success among multiple",
			results: []VerificationResult{
				{Success: true, Provider: ServiceVersion{Version: "1.0"}, VerifiedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
				{Success: false, Provider: ServiceVersion{Version: "2.0"}, VerifiedAt: time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)},
				{Success: true, Provider: ServiceVersion{Version: "3.0"}, VerifiedAt: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)},
				{Success: true, Provider: ServiceVersion{Version: "4.0"}, VerifiedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
			},
			wantVer: "3.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findLatestSuccessfulVerification(tt.results)
			if tt.wantNil {
				if got != nil {
					t.Errorf("expected nil, got result with version %q", got.Provider.Version)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil result")
			}
			if got.Provider.Version != tt.wantVer {
				t.Errorf("got version %q, want %q", got.Provider.Version, tt.wantVer)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkConsumerVerification — direct tests
// ---------------------------------------------------------------------------

func TestCheckConsumerVerification_NoResults(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c := createTestContract("web-ui", "data-api", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "web-ui",
		Provider: "data-api",
		Status:   contract.StatusPublished,
	}
	reason, err := deploySvc.checkConsumerVerification(ctx, summary)
	if err != nil {
		t.Fatalf("checkConsumerVerification failed: %v", err)
	}
	if reason.Status != "missing" {
		t.Errorf("Status = %q, want missing", reason.Status)
	}
	if reason.Consumer != "web-ui" {
		t.Errorf("Consumer = %q, want web-ui", reason.Consumer)
	}
	if reason.Provider != "data-api" {
		t.Errorf("Provider = %q, want data-api", reason.Provider)
	}
}

func TestCheckConsumerVerification_AllFailed(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("web-ui", "data-api", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record only failed verifications
	vr := createTestVerificationResult(c.Metadata.ID, "1.0.0", false)
	vr.Provider.Name = "data-api"
	vr.InteractionResults = []InteractionResult{{ID: "i1", Success: false}}
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "web-ui",
		Provider: "data-api",
		Status:   contract.StatusPublished,
	}
	reason, err := deploySvc.checkConsumerVerification(ctx, summary)
	if err != nil {
		t.Fatalf("checkConsumerVerification failed: %v", err)
	}
	if reason.Status != "failed" {
		t.Errorf("Status = %q, want failed", reason.Status)
	}
}

func TestCheckConsumerVerification_HasSuccess(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("web-ui", "data-api", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record a successful verification
	vr := createTestVerificationResult(c.Metadata.ID, "2.0.0", true)
	vr.Provider.Name = "data-api"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "web-ui",
		Provider: "data-api",
		Status:   contract.StatusPublished,
	}
	reason, err := deploySvc.checkConsumerVerification(ctx, summary)
	if err != nil {
		t.Fatalf("checkConsumerVerification failed: %v", err)
	}
	if reason.Status != "verified" {
		t.Errorf("Status = %q, want verified", reason.Status)
	}
}

// ---------------------------------------------------------------------------
// checkConsumerContracts — direct tests
// ---------------------------------------------------------------------------

func TestCheckConsumerContracts_SkipsDeprecated(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	// Create a deprecated contract where "mobile" is the consumer
	c := createTestContract("mobile", "payments-api", "1.0.0")
	c.Metadata.Tags = []string{"prod"}
	c.Metadata.Status = contract.StatusDeprecated
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	reasons, err := deploySvc.checkConsumerContracts(ctx, "mobile", "prod")
	if err != nil {
		t.Fatalf("checkConsumerContracts failed: %v", err)
	}
	if len(reasons) != 0 {
		t.Errorf("expected 0 reasons (deprecated skipped), got %d", len(reasons))
	}
}

func TestCheckConsumerContracts_ReturnsReasonsForActiveContracts(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	// Active contract where "mobile" is the consumer — no verification recorded
	c := createTestContract("mobile", "orders-api", "1.0.0")
	c.Metadata.Tags = []string{"prod"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	reasons, err := deploySvc.checkConsumerContracts(ctx, "mobile", "prod")
	if err != nil {
		t.Fatalf("checkConsumerContracts failed: %v", err)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(reasons))
	}
	if reasons[0].Status != "missing" {
		t.Errorf("Status = %q, want missing", reasons[0].Status)
	}
}

// ---------------------------------------------------------------------------
// checkProviderContracts — direct tests
// ---------------------------------------------------------------------------

func TestCheckProviderContracts_SkipsDeprecated(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c := createTestContract("frontend", "api-svc", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusDeprecated
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	reasons, err := deploySvc.checkProviderContracts(ctx, "api-svc", "1.0.0", "staging")
	if err != nil {
		t.Fatalf("checkProviderContracts failed: %v", err)
	}
	if len(reasons) != 0 {
		t.Errorf("expected 0 reasons (deprecated skipped), got %d", len(reasons))
	}
}

func TestCheckProviderContracts_MissingVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c := createTestContract("frontend", "api-svc", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	reasons, err := deploySvc.checkProviderContracts(ctx, "api-svc", "9.9.9", "staging")
	if err != nil {
		t.Fatalf("checkProviderContracts failed: %v", err)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(reasons))
	}
	if reasons[0].Status != "missing" {
		t.Errorf("Status = %q, want missing", reasons[0].Status)
	}
}

func TestCheckProviderContracts_FailedVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("frontend", "api-svc", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "5.0.0", false)
	vr.Provider.Name = "api-svc"
	vr.InteractionResults = []InteractionResult{{ID: "i1", Success: false}}
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	reasons, err := deploySvc.checkProviderContracts(ctx, "api-svc", "5.0.0", "staging")
	if err != nil {
		t.Fatalf("checkProviderContracts failed: %v", err)
	}
	if len(reasons) != 1 {
		t.Fatalf("expected 1 reason, got %d", len(reasons))
	}
	if reasons[0].Status != "failed" {
		t.Errorf("Status = %q, want failed", reasons[0].Status)
	}
}

// ---------------------------------------------------------------------------
// checkProviderVerification — direct tests
// ---------------------------------------------------------------------------

func TestCheckProviderVerification_Missing(t *testing.T) {
	ctx := context.Background()
	deploySvc, _, _ := newTestDeployService(t)

	summary := &ContractSummary{
		ID:       "nonexistent-contract",
		Consumer: "consumer-x",
		Provider: "provider-x",
	}
	reason, err := deploySvc.checkProviderVerification(ctx, summary, "1.0.0")
	if err != nil {
		t.Fatalf("checkProviderVerification failed: %v", err)
	}
	if reason.Status != "missing" {
		t.Errorf("Status = %q, want missing", reason.Status)
	}
}

func TestCheckProviderVerification_Failed(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("cons", "prov", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "3.0.0", false)
	vr.Provider.Name = "prov"
	vr.InteractionResults = []InteractionResult{{ID: "i1", Success: false}}
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "cons",
		Provider: "prov",
		Version:  "1.0.0",
	}
	reason, err := deploySvc.checkProviderVerification(ctx, summary, "3.0.0")
	if err != nil {
		t.Fatalf("checkProviderVerification failed: %v", err)
	}
	if reason.Status != "failed" {
		t.Errorf("Status = %q, want failed", reason.Status)
	}
}

func TestCheckProviderVerification_Verified(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("cons", "prov", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "3.0.0", true)
	vr.Provider.Name = "prov"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	summary := &ContractSummary{
		ID:       c.Metadata.ID,
		Consumer: "cons",
		Provider: "prov",
		Version:  "1.0.0",
	}
	reason, err := deploySvc.checkProviderVerification(ctx, summary, "3.0.0")
	if err != nil {
		t.Fatalf("checkProviderVerification failed: %v", err)
	}
	if reason.Status != "verified" {
		t.Errorf("Status = %q, want verified", reason.Status)
	}
}

// ---------------------------------------------------------------------------
// checkCellVerification — direct tests
// ---------------------------------------------------------------------------

func TestCheckCellVerification_AllVerified(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("cell-cons", "cell-prov", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "1.0.0", true)
	vr.Provider.Name = "cell-prov"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	contractList := []*ContractSummary{
		{ID: c.Metadata.ID, Consumer: "cell-cons", Provider: "cell-prov"},
	}
	allVerified, lastVerified := deploySvc.checkCellVerification(ctx, contractList)
	if !allVerified {
		t.Error("expected allVerified=true")
	}
	if lastVerified == "" {
		t.Error("expected lastVerified to be set")
	}
}

func TestCheckCellVerification_NoSuccessfulVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("cell-cons2", "cell-prov2", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record a failed verification only
	vr := createTestVerificationResult(c.Metadata.ID, "1.0.0", false)
	vr.Provider.Name = "cell-prov2"
	vr.InteractionResults = []InteractionResult{{ID: "i1", Success: false}}
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	contractList := []*ContractSummary{
		{ID: c.Metadata.ID, Consumer: "cell-cons2", Provider: "cell-prov2"},
	}
	allVerified, lastVerified := deploySvc.checkCellVerification(ctx, contractList)
	if allVerified {
		t.Error("expected allVerified=false when only failed verifications exist")
	}
	if lastVerified != "" {
		t.Errorf("expected empty lastVerified, got %q", lastVerified)
	}
}

func TestCheckCellVerification_MixedContracts(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// First contract: verified
	c1 := createTestContract("cell-mix-cons", "cell-mix-prov", "1.0.0")
	c1.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c1); err != nil {
		t.Fatalf("setup Publish c1 failed: %v", err)
	}
	vr1 := createTestVerificationResult(c1.Metadata.ID, "1.0.0", true)
	vr1.Provider.Name = "cell-mix-prov"
	if err := verificationSvc.RecordResult(ctx, vr1); err != nil {
		t.Fatalf("setup RecordResult vr1 failed: %v", err)
	}

	// Second contract: no verifications at all (will have empty results from ListResults)
	c2 := createTestContract("cell-mix-cons", "cell-mix-prov", "2.0.0")
	c2.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c2); err != nil {
		t.Fatalf("setup Publish c2 failed: %v", err)
	}

	contractList := []*ContractSummary{
		{ID: c1.Metadata.ID, Consumer: "cell-mix-cons", Provider: "cell-mix-prov"},
		{ID: c2.Metadata.ID, Consumer: "cell-mix-cons", Provider: "cell-mix-prov"},
	}
	allVerified, lastVerified := deploySvc.checkCellVerification(ctx, contractList)
	if allVerified {
		t.Error("expected allVerified=false when one contract has no verifications")
	}
	// lastVerified should still have the date from the first verified contract
	if lastVerified == "" {
		t.Error("expected lastVerified to be set from the verified contract")
	}
}

// ---------------------------------------------------------------------------
// CanDeploy — combined consumer+provider scenarios
// ---------------------------------------------------------------------------

func TestCanDeploy_AsConsumerWithNoProviderVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c := createTestContract("my-app", "remote-api", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// my-app is consumer, remote-api hasn't verified anything
	result, err := deploySvc.CanDeploy(ctx, "my-app", "1.0.0", "staging")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}
	if result.OK {
		t.Error("expected OK=false when provider hasn't verified")
	}
	if !hasReasonWithStatus(result, "missing") {
		t.Error("expected a missing status reason")
	}
}

func TestCanDeploy_AsConsumerWithFailedProviderVerification(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	c := createTestContract("my-app", "remote-api", "1.0.0")
	c.Metadata.Tags = []string{"staging"}
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	// Record a failed provider verification
	vr := createTestVerificationResult(c.Metadata.ID, "2.0.0", false)
	vr.Provider.Name = "remote-api"
	vr.InteractionResults = []InteractionResult{{ID: "i1", Success: false}}
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	result, err := deploySvc.CanDeploy(ctx, "my-app", "1.0.0", "staging")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}
	if result.OK {
		t.Error("expected OK=false when provider's verification failed")
	}
	if !hasReasonWithStatus(result, "failed") {
		t.Error("expected a failed status reason")
	}
}

func TestCanDeploy_MultipleProviderContracts(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)

	// Service "big-api" is provider for two different consumers
	c1 := createTestContract("app-a", "big-api", "1.0.0")
	c1.Metadata.Tags = []string{"prod"}
	c1.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c1); err != nil {
		t.Fatalf("setup Publish c1 failed: %v", err)
	}

	c2 := createTestContract("app-b", "big-api", "1.0.0")
	c2.Metadata.Tags = []string{"prod"}
	c2.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c2); err != nil {
		t.Fatalf("setup Publish c2 failed: %v", err)
	}

	// Verify only one contract
	vr := createTestVerificationResult(c1.Metadata.ID, "3.0.0", true)
	vr.Provider.Name = "big-api"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	result, err := deploySvc.CanDeploy(ctx, "big-api", "3.0.0", "prod")
	if err != nil {
		t.Fatalf("CanDeploy failed: %v", err)
	}
	if result.OK {
		t.Error("expected OK=false when one contract is unverified")
	}
	// Should have both a verified and a missing reason
	if !hasReasonWithStatus(result, "verified") {
		t.Error("expected a verified status reason")
	}
	if !hasReasonWithStatus(result, "missing") {
		t.Error("expected a missing status reason")
	}
}

// ---------------------------------------------------------------------------
// GetMatrix — additional tests
// ---------------------------------------------------------------------------

func TestGetMatrix_WithVerifiedContracts(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, verificationSvc := newTestDeployService(t)
	deploySvc = deploySvc.WithCacheTTL(0) // disable caching

	c := createTestContract("matrix-cons", "matrix-prov", "1.0.0")
	c.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c); err != nil {
		t.Fatalf("setup Publish failed: %v", err)
	}

	vr := createTestVerificationResult(c.Metadata.ID, "1.0.0", true)
	vr.Provider.Name = "matrix-prov"
	if err := verificationSvc.RecordResult(ctx, vr); err != nil {
		t.Fatalf("setup RecordResult failed: %v", err)
	}

	matrix, err := deploySvc.GetMatrix(ctx, "", "")
	if err != nil {
		t.Fatalf("GetMatrix failed: %v", err)
	}

	if len(matrix.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(matrix.Services))
	}
	// Find the cell for cons->prov
	found := false
	for _, row := range matrix.Cells {
		for _, cell := range row {
			if cell.Consumer == "matrix-cons" && cell.Provider == "matrix-prov" {
				found = true
				if cell.ContractCount != 1 {
					t.Errorf("ContractCount = %d, want 1", cell.ContractCount)
				}
				if !cell.Verified {
					t.Error("expected Verified=true")
				}
				if cell.LastVerified == "" {
					t.Error("expected LastVerified to be set")
				}
			}
		}
	}
	if !found {
		t.Error("cell for matrix-cons->matrix-prov not found")
	}
}

func TestGetMatrix_FilterByConsumer(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c1 := createTestContract("alpha", "beta", "1.0.0")
	c1.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c1); err != nil {
		t.Fatalf("setup Publish c1 failed: %v", err)
	}

	c2 := createTestContract("gamma", "delta", "1.0.0")
	c2.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c2); err != nil {
		t.Fatalf("setup Publish c2 failed: %v", err)
	}

	matrix, err := deploySvc.GetMatrix(ctx, "alpha", "")
	if err != nil {
		t.Fatalf("GetMatrix failed: %v", err)
	}
	// Should only contain alpha and beta
	for _, svc := range matrix.Services {
		if svc == "gamma" || svc == "delta" {
			t.Errorf("unexpected service %q in filtered matrix", svc)
		}
	}
}

func TestGetMatrix_FilterByProvider(t *testing.T) {
	ctx := context.Background()
	deploySvc, contractSvc, _ := newTestDeployService(t)

	c1 := createTestContract("alpha", "beta", "1.0.0")
	c1.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c1); err != nil {
		t.Fatalf("setup Publish c1 failed: %v", err)
	}

	c2 := createTestContract("gamma", "delta", "1.0.0")
	c2.Metadata.Status = contract.StatusPublished
	if _, err := contractSvc.Publish(ctx, c2); err != nil {
		t.Fatalf("setup Publish c2 failed: %v", err)
	}

	matrix, err := deploySvc.GetMatrix(ctx, "", "beta")
	if err != nil {
		t.Fatalf("GetMatrix failed: %v", err)
	}
	for _, svc := range matrix.Services {
		if svc == "gamma" || svc == "delta" {
			t.Errorf("unexpected service %q in filtered matrix", svc)
		}
	}
}

// ---------------------------------------------------------------------------
// groupContractsByPair tests
// ---------------------------------------------------------------------------

func TestGroupContractsByPair(t *testing.T) {
	tests := []struct {
		name      string
		contracts []ContractSummary
		wantPairs int
		checkKey  *pairKey
		wantCount int
	}{
		{
			name:      "empty contracts",
			contracts: nil,
			wantPairs: 0,
		},
		{
			name: "single contract",
			contracts: []ContractSummary{
				{ID: "c1", Consumer: "a", Provider: "b"},
			},
			wantPairs: 1,
			checkKey:  &pairKey{consumer: "a", provider: "b"},
			wantCount: 1,
		},
		{
			name: "multiple contracts same pair",
			contracts: []ContractSummary{
				{ID: "c1", Consumer: "a", Provider: "b"},
				{ID: "c2", Consumer: "a", Provider: "b"},
				{ID: "c3", Consumer: "a", Provider: "b"},
			},
			wantPairs: 1,
			checkKey:  &pairKey{consumer: "a", provider: "b"},
			wantCount: 3,
		},
		{
			name: "different pairs",
			contracts: []ContractSummary{
				{ID: "c1", Consumer: "a", Provider: "b"},
				{ID: "c2", Consumer: "x", Provider: "y"},
			},
			wantPairs: 2,
		},
		{
			name: "reverse pair is separate",
			contracts: []ContractSummary{
				{ID: "c1", Consumer: "a", Provider: "b"},
				{ID: "c2", Consumer: "b", Provider: "a"},
			},
			wantPairs: 2,
			checkKey:  &pairKey{consumer: "a", provider: "b"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupContractsByPair(tt.contracts)
			if len(got) != tt.wantPairs {
				t.Errorf("got %d pairs, want %d", len(got), tt.wantPairs)
			}
			if tt.checkKey != nil {
				group := got[*tt.checkKey]
				if len(group) != tt.wantCount {
					t.Errorf("pair %v has %d contracts, want %d", *tt.checkKey, len(group), tt.wantCount)
				}
			}
		})
	}
}
