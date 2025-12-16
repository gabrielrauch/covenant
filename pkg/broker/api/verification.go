package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
)

// VerificationService manages verification results.
type VerificationService struct {
	storage   storage.Backend
	keys      *storage.KeyBuilder
	contracts *ContractService
}

// NewVerificationService creates a new verification service.
func NewVerificationService(backend storage.Backend, contracts *ContractService) *VerificationService {
	return &VerificationService{
		storage:   backend,
		keys:      storage.NewKeyBuilder(""),
		contracts: contracts,
	}
}

// VerificationResult represents the outcome of verifying a provider against a contract.
type VerificationResult struct {
	ContractID       string              `json:"contract_id"`
	ContractVersion  string              `json:"contract_version"`
	Provider         ServiceVersion      `json:"provider"`
	Consumer         ServiceVersion      `json:"consumer"`
	VerifiedAt       time.Time           `json:"verified_at"`
	DurationMS       int64               `json:"duration_ms"`
	Success          bool                `json:"success"`
	Summary          VerificationSummary `json:"summary"`
	InteractionResults []InteractionResult `json:"interactions"`
}

// ServiceVersion identifies a service and its version.
type ServiceVersion struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// VerificationSummary provides a high-level overview of verification results.
type VerificationSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// InteractionResult contains the verification result for a single interaction.
type InteractionResult struct {
	ID          string             `json:"id"`
	Description string             `json:"description"`
	Success     bool               `json:"success"`
	DurationMS  int64              `json:"duration_ms"`
	Errors      []InteractionError `json:"errors,omitempty"`
}

// InteractionError represents a single validation error in an interaction.
type InteractionError struct {
	Path     string `json:"path"`
	Rule     string `json:"rule,omitempty"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Message  string `json:"message"`
}

// RecordResult stores a verification result.
func (s *VerificationService) RecordResult(ctx context.Context, result *VerificationResult) error {
	if result.ContractID == "" {
		return fmt.Errorf("contract ID is required")
	}
	if result.Provider.Name == "" || result.Provider.Version == "" {
		return fmt.Errorf("provider name and version are required")
	}

	if result.VerifiedAt.IsZero() {
		result.VerifiedAt = time.Now().UTC()
	}

	// Compute summary
	result.Summary.Total = len(result.InteractionResults)
	result.Summary.Passed = 0
	result.Summary.Failed = 0
	for _, ir := range result.InteractionResults {
		if ir.Success {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}
	result.Success = result.Summary.Failed == 0

	data, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to serialize result: %w", err)
	}

	key := s.keys.VerificationResult(result.ContractID, result.Provider.Version)
	if err := s.storage.Save(ctx, key, data); err != nil {
		return fmt.Errorf("failed to store result: %w", err)
	}

	return nil
}

// GetResult retrieves a verification result.
func (s *VerificationService) GetResult(ctx context.Context, contractID, providerVersion string) (*VerificationResult, error) {
	key := s.keys.VerificationResult(contractID, providerVersion)
	data, err := s.storage.Load(ctx, key)
	if err != nil {
		return nil, err
	}

	var result VerificationResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to parse result: %w", err)
	}

	return &result, nil
}

// ListResults returns all verification results for a contract.
func (s *VerificationService) ListResults(ctx context.Context, contractID string) ([]VerificationResult, error) {
	prefix := s.keys.VerificationResult(contractID, "")
	keys, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list results: %w", err)
	}

	var results []VerificationResult
	for _, key := range keys {
		data, err := s.storage.Load(ctx, key)
		if err != nil {
			continue
		}

		var result VerificationResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}
