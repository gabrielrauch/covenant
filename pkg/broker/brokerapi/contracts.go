// Package brokerapi provides the broker API for contract management.
package brokerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/storage"
	"github.com/gabrielrauch/covenant/pkg/contract"
)

// ContractService manages contracts in the broker.
type ContractService struct {
	storage storage.Backend
	keys    *storage.KeyBuilder
}

// NewContractService creates a new contract service.
func NewContractService(backend storage.Backend) *ContractService {
	return &ContractService{
		storage: backend,
		keys:    storage.NewKeyBuilder(""),
	}
}

// ContractFilter defines criteria for filtering contracts.
type ContractFilter struct {
	Consumer string
	Provider string
	Tag      string
	Status   contract.Status
	Version  string
}

// ContractSummary contains summary information about a contract.
type ContractSummary struct {
	ID           string          `json:"id"`
	Version      string          `json:"version"`
	Consumer     string          `json:"consumer"`
	Provider     string          `json:"provider"`
	Status       contract.Status `json:"status"`
	Tags         []string        `json:"tags"`
	CreatedAt    time.Time       `json:"created_at"`
	Interactions int             `json:"interactions"`
}

// PublishResult contains the result of publishing a contract.
type PublishResult struct {
	ID       string `json:"id"`
	Version  string `json:"version"`
	IsNew    bool   `json:"is_new"`
	Checksum string `json:"checksum"`
}

// Publish stores a new contract or updates an existing draft.
func (s *ContractService) Publish(ctx context.Context, c *contract.Contract) (*PublishResult, error) {
	// Validate contract
	if err := contract.Validate(c); err != nil {
		return nil, fmt.Errorf("invalid contract: %w", err)
	}

	// Set created time if not set
	if c.Metadata.CreatedAt.IsZero() {
		c.Metadata.CreatedAt = time.Now().UTC()
	}

	// Compute checksum
	if err := contract.UpdateChecksum(c); err != nil {
		return nil, fmt.Errorf("failed to compute checksum: %w", err)
	}

	// Check if a contract with this ID already exists
	existingData, loadErr := s.storage.Load(ctx, s.keys.ContractByID(c.Metadata.ID))
	if loadErr == nil {
		// Contract exists, check if it can be updated
		var existing contract.Contract
		if unmarshalErr := json.Unmarshal(existingData, &existing); unmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse existing contract: %w", unmarshalErr)
		}

		// Only drafts can be updated
		if existing.Metadata.Status != contract.StatusDraft {
			return nil, fmt.Errorf("cannot update non-draft contract (status: %s)", existing.Metadata.Status)
		}
	} else if loadErr != storage.ErrNotFound {
		return nil, fmt.Errorf("failed to check existing contract: %w", loadErr)
	}

	// Serialize contract
	data, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize contract: %w", err)
	}

	// Store contract
	ops := []storage.Operation{
		{Type: storage.OpSave, Key: s.keys.ContractByID(c.Metadata.ID), Data: data},
		{Type: storage.OpSave, Key: s.keys.Contract(c.Metadata.Consumer.Name, c.Metadata.Provider.Name, c.Metadata.Version), Data: data},
		{Type: storage.OpSave, Key: s.keys.LatestContract(c.Metadata.Consumer.Name, c.Metadata.Provider.Name), Data: data},
	}

	// Update tags index
	for _, tag := range c.Metadata.Tags {
		tagKey := s.keys.TaggedContracts(tag) + c.Metadata.ID + ".json"
		ops = append(ops, storage.Operation{Type: storage.OpSave, Key: tagKey, Data: data})
	}

	if txErr := s.storage.Transaction(ctx, ops); txErr != nil {
		return nil, fmt.Errorf("failed to store contract: %w", txErr)
	}

	return &PublishResult{
		ID:       c.Metadata.ID,
		Version:  c.Metadata.Version,
		IsNew:    loadErr == storage.ErrNotFound,
		Checksum: c.Metadata.Checksum,
	}, nil
}

// Get retrieves a contract by consumer, provider, and optional version.
func (s *ContractService) Get(ctx context.Context, consumer, provider, version string) (*contract.Contract, error) {
	var key string
	if version == "" || version == "latest" {
		key = s.keys.LatestContract(consumer, provider)
	} else {
		key = s.keys.Contract(consumer, provider, version)
	}

	return s.loadContract(ctx, key)
}

// GetByID retrieves a contract by its ID.
func (s *ContractService) GetByID(ctx context.Context, id string) (*contract.Contract, error) {
	return s.loadContract(ctx, s.keys.ContractByID(id))
}

// List returns contracts matching the filter.
func (s *ContractService) List(ctx context.Context, filter *ContractFilter) ([]ContractSummary, error) {
	prefix := s.resolveListPrefix(filter)

	keys, err := s.storage.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}

	summaries := make([]ContractSummary, 0, len(keys))
	seen := make(map[string]bool)

	for _, key := range keys {
		if !strings.HasSuffix(key, ".json") {
			continue
		}

		c, err := s.loadContract(ctx, key)
		if err != nil {
			continue
		}

		if seen[c.Metadata.ID] {
			continue
		}
		seen[c.Metadata.ID] = true

		if !s.matchesFilter(c, filter) {
			continue
		}

		summaries = append(summaries, s.buildContractSummary(c))
	}

	return summaries, nil
}

// resolveListPrefix determines the storage prefix for listing contracts.
func (s *ContractService) resolveListPrefix(filter *ContractFilter) string {
	if filter.Tag != "" {
		return s.keys.TaggedContracts(filter.Tag)
	}
	if filter.Consumer != "" && filter.Provider != "" {
		return s.keys.Contract(filter.Consumer, filter.Provider, "")
	}
	if filter.Consumer != "" {
		return s.keys.AllContracts() + filter.Consumer + "/"
	}
	return s.keys.AllContracts()
}

// matchesFilter checks if a contract matches the given filter.
func (s *ContractService) matchesFilter(c *contract.Contract, filter *ContractFilter) bool {
	if filter.Consumer != "" && c.Metadata.Consumer.Name != filter.Consumer {
		return false
	}
	if filter.Provider != "" && c.Metadata.Provider.Name != filter.Provider {
		return false
	}
	if filter.Status != "" && c.Metadata.Status != filter.Status {
		return false
	}
	if filter.Version != "" && c.Metadata.Version != filter.Version {
		return false
	}
	return true
}

// buildContractSummary creates a ContractSummary from a contract.
func (s *ContractService) buildContractSummary(c *contract.Contract) ContractSummary {
	return ContractSummary{
		ID:           c.Metadata.ID,
		Version:      c.Metadata.Version,
		Consumer:     c.Metadata.Consumer.Name,
		Provider:     c.Metadata.Provider.Name,
		Status:       c.Metadata.Status,
		Tags:         c.Metadata.Tags,
		CreatedAt:    c.Metadata.CreatedAt,
		Interactions: len(c.Interactions),
	}
}

// Tag adds tags to a contract.
func (s *ContractService) Tag(ctx context.Context, contractID string, tags []string) error {
	c, err := s.GetByID(ctx, contractID)
	if err != nil {
		return err
	}

	// Add new tags
	existingTags := make(map[string]bool)
	for _, t := range c.Metadata.Tags {
		existingTags[t] = true
	}

	for _, tag := range tags {
		if !existingTags[tag] {
			c.Metadata.Tags = append(c.Metadata.Tags, tag)
			existingTags[tag] = true
		}
	}

	// Re-publish with updated tags
	_, err = s.Publish(ctx, c)
	return err
}

// Untag removes tags from a contract.
func (s *ContractService) Untag(ctx context.Context, contractID string, tags []string) error {
	c, err := s.GetByID(ctx, contractID)
	if err != nil {
		return err
	}

	// Build set of tags to remove
	removeTags := make(map[string]bool)
	for _, t := range tags {
		removeTags[t] = true
	}

	// Filter out removed tags with pre-allocated slice
	newTags := make([]string, 0, len(c.Metadata.Tags))
	for _, t := range c.Metadata.Tags {
		if !removeTags[t] {
			newTags = append(newTags, t)
		}
	}
	c.Metadata.Tags = newTags

	// Re-publish with updated tags
	_, err = s.Publish(ctx, c)
	return err
}

// Deprecate marks a contract as deprecated.
func (s *ContractService) Deprecate(ctx context.Context, contractID string) error {
	c, err := s.GetByID(ctx, contractID)
	if err != nil {
		return err
	}

	if c.Metadata.Status == contract.StatusDeprecated {
		return nil // Already deprecated
	}

	c.Metadata.Status = contract.StatusDeprecated

	// Re-serialize and store
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to serialize contract: %w", err)
	}

	return s.storage.Save(ctx, s.keys.ContractByID(c.Metadata.ID), data)
}

// loadContract loads and parses a contract from storage.
func (s *ContractService) loadContract(ctx context.Context, key string) (*contract.Contract, error) {
	data, err := s.storage.Load(ctx, key)
	if err != nil {
		return nil, err
	}

	var c contract.Contract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("failed to parse contract: %w", err)
	}

	return &c, nil
}
