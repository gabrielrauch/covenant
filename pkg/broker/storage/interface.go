// Package storage provides pluggable storage backends for the contract broker.
package storage

import (
	"context"
	"errors"
)

// Common errors returned by storage backends.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrConflict      = errors.New("conflict")
	ErrInvalidKey    = errors.New("invalid key")
)

// Backend defines the interface for storage backends.
type Backend interface {
	// Save stores data at the given key.
	Save(ctx context.Context, key string, data []byte) error

	// Load retrieves data from the given key.
	Load(ctx context.Context, key string) ([]byte, error)

	// List returns all keys matching the prefix.
	List(ctx context.Context, prefix string) ([]string, error)

	// Delete removes the data at the given key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Transaction executes multiple operations atomically.
	Transaction(ctx context.Context, ops []Operation) error
}

// Operation represents a single storage operation for transactions.
type Operation struct {
	Type OperationType
	Key  string
	Data []byte
}

// OperationType represents the type of operation.
type OperationType string

const (
	OpSave   OperationType = "save"
	OpDelete OperationType = "delete"
)

// KeyBuilder helps construct storage keys.
type KeyBuilder struct {
	prefix string
}

// NewKeyBuilder creates a new key builder with the given prefix.
func NewKeyBuilder(prefix string) *KeyBuilder {
	return &KeyBuilder{prefix: prefix}
}

// Contract returns a key for storing a contract.
func (kb *KeyBuilder) Contract(consumer, provider, version string) string {
	return kb.prefix + "/contracts/" + consumer + "/" + provider + "/" + version + ".json"
}

// ContractByID returns a key for storing a contract by ID.
func (kb *KeyBuilder) ContractByID(id string) string {
	return kb.prefix + "/contracts/by-id/" + id + ".json"
}

// VerificationResult returns a key for storing a verification result.
func (kb *KeyBuilder) VerificationResult(contractID, providerVersion string) string {
	return kb.prefix + "/verifications/" + contractID + "/" + providerVersion + ".json"
}

// LatestContract returns a key for the latest contract version.
func (kb *KeyBuilder) LatestContract(consumer, provider string) string {
	return kb.prefix + "/contracts/" + consumer + "/" + provider + "/latest.json"
}

// TaggedContracts returns a prefix for contracts with a specific tag.
func (kb *KeyBuilder) TaggedContracts(tag string) string {
	return kb.prefix + "/tags/" + tag + "/"
}

// Webhook returns a key for a webhook configuration.
func (kb *KeyBuilder) Webhook(id string) string {
	return kb.prefix + "/webhooks/" + id + ".json"
}

// AllContracts returns the prefix for all contracts.
func (kb *KeyBuilder) AllContracts() string {
	return kb.prefix + "/contracts/"
}

// AllVerifications returns the prefix for all verification results.
func (kb *KeyBuilder) AllVerifications() string {
	return kb.prefix + "/verifications/"
}
