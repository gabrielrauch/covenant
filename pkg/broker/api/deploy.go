package api

import (
	"context"
	"fmt"

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
	OK      bool            `json:"ok"`
	Service string          `json:"service"`
	Version string          `json:"version"`
	Env     string          `json:"environment"`
	Reasons []DeployReason  `json:"reasons"`
}

// DeployReason explains why a deployment is or isn't safe.
type DeployReason struct {
	ContractID   string `json:"contract_id"`
	Consumer     string `json:"consumer"`
	Provider     string `json:"provider"`
	Status       string `json:"status"` // verified, missing, failed
	Message      string `json:"message"`
}

// CanDeploy checks if it's safe to deploy a service version to an environment.
func (s *DeployService) CanDeploy(ctx context.Context, service, version, environment string) (*CanDeployResult, error) {
	result := &CanDeployResult{
		OK:      true,
		Service: service,
		Version: version,
		Env:     environment,
	}

	// Find all contracts where this service is the provider
	providerContracts, err := s.contracts.List(ctx, ContractFilter{
		Provider: service,
		Tag:      environment,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list provider contracts: %w", err)
	}

	for _, summary := range providerContracts {
		// Skip deprecated contracts
		if summary.Status == contract.StatusDeprecated {
			continue
		}

		// Check if verification exists for this contract and version
		vr, err := s.verifications.GetResult(ctx, summary.ID, version)
		if err == storage.ErrNotFound {
			result.OK = false
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "missing",
				Message:    fmt.Sprintf("no verification found for contract %s (consumer: %s)", summary.ID, summary.Consumer),
			})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to get verification for %s: %w", summary.ID, err)
		}

		if !vr.Success {
			result.OK = false
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "failed",
				Message:    fmt.Sprintf("verification failed for contract %s (%d/%d interactions passed)", summary.ID, vr.Summary.Passed, vr.Summary.Total),
			})
		} else {
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "verified",
				Message:    fmt.Sprintf("verified against %s v%s", summary.Consumer, summary.Version),
			})
		}
	}

	// Find all contracts where this service is the consumer
	consumerContracts, err := s.contracts.List(ctx, ContractFilter{
		Consumer: service,
		Tag:      environment,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer contracts: %w", err)
	}

	for _, summary := range consumerContracts {
		if summary.Status == contract.StatusDeprecated {
			continue
		}

		// Check if the provider has verified against this contract
		// We need to find any verification where the provider verified against this contract
		results, err := s.verifications.ListResults(ctx, summary.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list verifications for %s: %w", summary.ID, err)
		}

		if len(results) == 0 {
			result.OK = false
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "missing",
				Message:    fmt.Sprintf("provider %s has not verified contract %s", summary.Provider, summary.ID),
			})
			continue
		}

		// Find the most recent successful verification
		var latestSuccess *VerificationResult
		for i := range results {
			if results[i].Success {
				if latestSuccess == nil || results[i].VerifiedAt.After(latestSuccess.VerifiedAt) {
					latestSuccess = &results[i]
				}
			}
		}

		if latestSuccess == nil {
			result.OK = false
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "failed",
				Message:    fmt.Sprintf("no successful verification from provider %s for contract %s", summary.Provider, summary.ID),
			})
		} else {
			result.Reasons = append(result.Reasons, DeployReason{
				ContractID: summary.ID,
				Consumer:   summary.Consumer,
				Provider:   summary.Provider,
				Status:     "verified",
				Message:    fmt.Sprintf("provider %s verified at %s", summary.Provider, latestSuccess.VerifiedAt.Format("2006-01-02 15:04:05")),
			})
		}
	}

	return result, nil
}

// Matrix represents a compatibility matrix between services.
type Matrix struct {
	Services []string         `json:"services"`
	Cells    [][]MatrixCell   `json:"cells"`
}

// MatrixCell represents a single cell in the compatibility matrix.
type MatrixCell struct {
	Consumer      string `json:"consumer"`
	Provider      string `json:"provider"`
	ContractCount int    `json:"contract_count"`
	Verified      bool   `json:"verified"`
	LastVerified  string `json:"last_verified,omitempty"`
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

	contracts, err := s.contracts.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}

	// Collect unique services
	servicesMap := make(map[string]bool)
	for _, c := range contracts {
		servicesMap[c.Consumer] = true
		servicesMap[c.Provider] = true
	}

	services := make([]string, 0, len(servicesMap))
	for svc := range servicesMap {
		services = append(services, svc)
	}

	// Build matrix
	matrix := &Matrix{
		Services: services,
	}

	// Group contracts by consumer-provider pair
	type pairKey struct{ consumer, provider string }
	pairs := make(map[pairKey][]ContractSummary)
	for _, c := range contracts {
		key := pairKey{c.Consumer, c.Provider}
		pairs[key] = append(pairs[key], c)
	}

	// Build cells
	for consumerSvc := range servicesMap {
		var row []MatrixCell
		for providerSvc := range servicesMap {
			if consumerSvc == providerSvc {
				row = append(row, MatrixCell{
					Consumer: consumerSvc,
					Provider: providerSvc,
				})
				continue
			}

			key := pairKey{consumerSvc, providerSvc}
			contractList := pairs[key]

			cell := MatrixCell{
				Consumer:      consumerSvc,
				Provider:      providerSvc,
				ContractCount: len(contractList),
			}

			// Check verification status
			if len(contractList) > 0 {
				allVerified := true
				for _, c := range contractList {
					results, _ := s.verifications.ListResults(ctx, c.ID)
					hasSuccess := false
					for _, r := range results {
						if r.Success {
							hasSuccess = true
							if cell.LastVerified == "" || r.VerifiedAt.Format("2006-01-02") > cell.LastVerified {
								cell.LastVerified = r.VerifiedAt.Format("2006-01-02")
							}
							break
						}
					}
					if !hasSuccess {
						allVerified = false
					}
				}
				cell.Verified = allVerified
			}

			row = append(row, cell)
		}
		matrix.Cells = append(matrix.Cells, row)
	}

	return matrix, nil
}
