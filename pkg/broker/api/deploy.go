package api

import (
	"context"
	"fmt"
	"sort"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
)

// DeployService handles deployment safety checks.
type DeployService struct {
	contracts     *ContractService
	verifications *VerificationService
}

// NewDeployService creates a new deploy service.
func NewDeployService(contracts *ContractService, verifications *VerificationService) *DeployService {
	return &DeployService{
		contracts:     contracts,
		verifications: verifications,
	}
}

// CanDeployResult contains the result of a can-deploy check.
type CanDeployResult struct {
	OK      bool           `json:"ok"`
	Service string         `json:"service"`
	Version string         `json:"version"`
	Env     string         `json:"environment"`
	Reasons []DeployReason `json:"reasons"`
}

// DeployReason explains why a deployment is or isn't safe.
type DeployReason struct {
	ContractID string `json:"contract_id"`
	Consumer   string `json:"consumer"`
	Provider   string `json:"provider"`
	Status     string `json:"status"` // verified, missing, failed
	Message    string `json:"message"`
}

// CanDeploy checks if it's safe to deploy a service version to an environment.
func (s *DeployService) CanDeploy(ctx context.Context, service, version, environment string) (*CanDeployResult, error) {
	result := &CanDeployResult{
		OK:      true,
		Service: service,
		Version: version,
		Env:     environment,
	}

	// Check contracts where this service is the provider
	providerReasons, err := s.checkProviderContracts(ctx, service, version, environment)
	if err != nil {
		return nil, err
	}
	result.Reasons = append(result.Reasons, providerReasons...)

	// Check contracts where this service is the consumer
	consumerReasons, err := s.checkConsumerContracts(ctx, service, environment)
	if err != nil {
		return nil, err
	}
	result.Reasons = append(result.Reasons, consumerReasons...)

	// Set OK = false if any reason indicates a problem
	for _, reason := range result.Reasons {
		if reason.Status != "verified" {
			result.OK = false
			break
		}
	}

	return result, nil
}

// checkProviderContracts verifies all contracts where the service is the provider.
func (s *DeployService) checkProviderContracts(ctx context.Context, service, version, environment string) ([]DeployReason, error) {
	contracts, err := s.contracts.List(ctx, &ContractFilter{
		Provider: service,
		Tag:      environment,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list provider contracts: %w", err)
	}

	reasons := make([]DeployReason, 0, len(contracts))
	for i := range contracts {
		summary := &contracts[i]
		if summary.Status == contract.StatusDeprecated {
			continue
		}

		reason, err := s.checkProviderVerification(ctx, summary, version)
		if err != nil {
			return nil, err
		}
		reasons = append(reasons, reason)
	}

	return reasons, nil
}

// checkProviderVerification checks a single provider contract verification.
func (s *DeployService) checkProviderVerification(ctx context.Context, summary *ContractSummary, version string) (DeployReason, error) {
	vr, err := s.verifications.GetResult(ctx, summary.ID, version)
	if err == storage.ErrNotFound {
		return DeployReason{
			ContractID: summary.ID,
			Consumer:   summary.Consumer,
			Provider:   summary.Provider,
			Status:     "missing",
			Message:    fmt.Sprintf("no verification found for contract %s (consumer: %s)", summary.ID, summary.Consumer),
		}, nil
	}
	if err != nil {
		return DeployReason{}, fmt.Errorf("failed to get verification for %s: %w", summary.ID, err)
	}

	if !vr.Success {
		return DeployReason{
			ContractID: summary.ID,
			Consumer:   summary.Consumer,
			Provider:   summary.Provider,
			Status:     "failed",
			Message:    fmt.Sprintf("verification failed for contract %s (%d/%d interactions passed)", summary.ID, vr.Summary.Passed, vr.Summary.Total),
		}, nil
	}

	return DeployReason{
		ContractID: summary.ID,
		Consumer:   summary.Consumer,
		Provider:   summary.Provider,
		Status:     "verified",
		Message:    fmt.Sprintf("verified against %s v%s", summary.Consumer, summary.Version),
	}, nil
}

// checkConsumerContracts verifies all contracts where the service is the consumer.
func (s *DeployService) checkConsumerContracts(ctx context.Context, service, environment string) ([]DeployReason, error) {
	contracts, err := s.contracts.List(ctx, &ContractFilter{
		Consumer: service,
		Tag:      environment,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer contracts: %w", err)
	}

	reasons := make([]DeployReason, 0, len(contracts))
	for i := range contracts {
		summary := &contracts[i]
		if summary.Status == contract.StatusDeprecated {
			continue
		}

		reason, err := s.checkConsumerVerification(ctx, summary)
		if err != nil {
			return nil, err
		}
		reasons = append(reasons, reason)
	}

	return reasons, nil
}

// checkConsumerVerification checks if a provider has verified a consumer contract.
func (s *DeployService) checkConsumerVerification(ctx context.Context, summary *ContractSummary) (DeployReason, error) {
	results, err := s.verifications.ListResults(ctx, summary.ID)
	if err != nil {
		return DeployReason{}, fmt.Errorf("failed to list verifications for %s: %w", summary.ID, err)
	}

	if len(results) == 0 {
		return DeployReason{
			ContractID: summary.ID,
			Consumer:   summary.Consumer,
			Provider:   summary.Provider,
			Status:     "missing",
			Message:    fmt.Sprintf("provider %s has not verified contract %s", summary.Provider, summary.ID),
		}, nil
	}

	latestSuccess := findLatestSuccessfulVerification(results)
	if latestSuccess == nil {
		return DeployReason{
			ContractID: summary.ID,
			Consumer:   summary.Consumer,
			Provider:   summary.Provider,
			Status:     "failed",
			Message:    fmt.Sprintf("no successful verification from provider %s for contract %s", summary.Provider, summary.ID),
		}, nil
	}

	return DeployReason{
		ContractID: summary.ID,
		Consumer:   summary.Consumer,
		Provider:   summary.Provider,
		Status:     "verified",
		Message:    fmt.Sprintf("provider %s verified at %s", summary.Provider, latestSuccess.VerifiedAt.Format("2006-01-02 15:04:05")),
	}, nil
}

// findLatestSuccessfulVerification finds the most recent successful verification from results.
func findLatestSuccessfulVerification(results []VerificationResult) *VerificationResult {
	var latest *VerificationResult
	for i := range results {
		if results[i].Success {
			if latest == nil || results[i].VerifiedAt.After(latest.VerifiedAt) {
				latest = &results[i]
			}
		}
	}
	return latest
}

// Matrix represents a compatibility matrix between services.
type Matrix struct {
	Services []string       `json:"services"`
	Cells    [][]MatrixCell `json:"cells"`
}

// MatrixCell represents a single cell in the compatibility matrix.
type MatrixCell struct {
	Consumer      string `json:"consumer"`
	Provider      string `json:"provider"`
	ContractCount int    `json:"contract_count"`
	Verified      bool   `json:"verified"`
	LastVerified  string `json:"last_verified,omitempty"`
}

// pairKey identifies a consumer-provider pair.
type pairKey struct {
	consumer, provider string
}

// GetMatrix returns a compatibility matrix for the given services.
func (s *DeployService) GetMatrix(ctx context.Context, consumer, provider string) (*Matrix, error) {
	filter := ContractFilter{}
	if consumer != "" {
		filter.Consumer = consumer
	}
	if provider != "" {
		filter.Provider = provider
	}

	contracts, err := s.contracts.List(ctx, &filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}

	services := extractServices(contracts)
	pairs := groupContractsByPair(contracts)

	matrix := &Matrix{
		Services: services,
	}

	for _, consumerSvc := range services {
		var row []MatrixCell
		for _, providerSvc := range services {
			cell := s.buildMatrixCell(ctx, consumerSvc, providerSvc, pairs)
			row = append(row, cell)
		}
		matrix.Cells = append(matrix.Cells, row)
	}

	return matrix, nil
}

// extractServices builds a sorted list of unique service names from contracts.
func extractServices(contracts []ContractSummary) []string {
	servicesMap := make(map[string]bool)
	for i := range contracts {
		servicesMap[contracts[i].Consumer] = true
		servicesMap[contracts[i].Provider] = true
	}

	services := make([]string, 0, len(servicesMap))
	for svc := range servicesMap {
		services = append(services, svc)
	}
	sort.Strings(services)

	return services
}

// groupContractsByPair groups contracts by their consumer-provider pair.
func groupContractsByPair(contracts []ContractSummary) map[pairKey][]*ContractSummary {
	pairs := make(map[pairKey][]*ContractSummary)
	for i := range contracts {
		c := &contracts[i]
		key := pairKey{c.Consumer, c.Provider}
		pairs[key] = append(pairs[key], c)
	}
	return pairs
}

// buildMatrixCell creates a matrix cell for a consumer-provider pair.
func (s *DeployService) buildMatrixCell(ctx context.Context, consumerSvc, providerSvc string, pairs map[pairKey][]*ContractSummary) MatrixCell {
	if consumerSvc == providerSvc {
		return MatrixCell{
			Consumer: consumerSvc,
			Provider: providerSvc,
		}
	}

	key := pairKey{consumerSvc, providerSvc}
	contractList := pairs[key]

	cell := MatrixCell{
		Consumer:      consumerSvc,
		Provider:      providerSvc,
		ContractCount: len(contractList),
	}

	if len(contractList) > 0 {
		cell.Verified, cell.LastVerified = s.checkCellVerification(ctx, contractList)
	}

	return cell
}

// checkCellVerification checks verification status for all contracts in a cell.
func (s *DeployService) checkCellVerification(ctx context.Context, contractList []*ContractSummary) (allVerified bool, lastVerified string) {
	allVerified = true

	for _, c := range contractList {
		results, err := s.verifications.ListResults(ctx, c.ID)
		if err != nil {
			allVerified = false
			continue
		}

		hasSuccess, latestDate := findLatestSuccessDate(results)
		if !hasSuccess {
			allVerified = false
		} else if lastVerified == "" || latestDate > lastVerified {
			lastVerified = latestDate
		}
	}

	return allVerified, lastVerified
}

// findLatestSuccessDate finds the latest successful verification date.
func findLatestSuccessDate(results []VerificationResult) (hasSuccess bool, latestDate string) {
	for i := range results {
		if results[i].Success {
			return true, results[i].VerifiedAt.Format("2006-01-02")
		}
	}
	return false, ""
}
