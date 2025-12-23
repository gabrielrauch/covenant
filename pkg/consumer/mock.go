package consumer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gabrielrauch/covenant/pkg/contract"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

// Pact manages consumer contract testing with automatic mock server lifecycle.
// It provides a fluent API to define interactions and verify them against a mock server.
type Pact struct {
	consumer  string
	provider  string
	contracts []*contract.Contract
	mock      *httpvalidator.MockServer
	outDir    string
}

// NewPact creates a new pact for consumer testing.
func NewPact(consumer, provider string) *Pact {
	return &Pact{
		consumer: consumer,
		provider: provider,
		outDir:   "./contracts",
	}
}

// WithOutputDir sets the output directory for contract files.
func (p *Pact) WithOutputDir(dir string) *Pact {
	p.outDir = dir
	return p
}

// AddInteraction adds an interaction to the pact.
func (p *Pact) AddInteraction(interaction *Interaction) *Pact {
	if len(p.contracts) == 0 {
		// Create a new contract
		c := NewContract(p.consumer, p.provider).Build()
		p.contracts = append(p.contracts, c)
	}

	// Add interaction to the latest contract
	latest := p.contracts[len(p.contracts)-1]
	latest.Interactions = append(latest.Interactions, interaction.Build())

	return p
}

// Setup starts the mock server and returns its URL.
func (p *Pact) Setup() (string, error) {
	if len(p.contracts) == 0 || len(p.contracts[0].Interactions) == 0 {
		return "", fmt.Errorf("no interactions defined")
	}

	// Create mock server with all interactions
	var allInteractions []contract.Interaction
	for _, c := range p.contracts {
		allInteractions = append(allInteractions, c.Interactions...)
	}

	p.mock = httpvalidator.NewMockServer(allInteractions)
	if err := p.mock.Start(); err != nil {
		return "", fmt.Errorf("failed to start mock server: %w", err)
	}

	return p.mock.URL(), nil
}

// Verify verifies that all interactions were matched and saves the contract.
func (p *Pact) Verify() error {
	if p.mock == nil {
		return fmt.Errorf("mock server not started")
	}

	// Verify all interactions were matched
	result := p.mock.Verify()
	if !result.Success {
		var messages []string
		for _, err := range result.Errors {
			messages = append(messages, err.Message)
		}
		return fmt.Errorf("pact verification failed: %v", messages)
	}

	// Save contracts
	if err := os.MkdirAll(p.outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for _, c := range p.contracts {
		if err := contract.UpdateChecksum(c); err != nil {
			return fmt.Errorf("failed to update checksum: %w", err)
		}
		filename := fmt.Sprintf("%s-%s.json", p.consumer, p.provider)
		outPath := filepath.Join(p.outDir, filename)
		if err := contract.SaveToFile(c, outPath); err != nil {
			return fmt.Errorf("failed to save contract: %w", err)
		}
	}

	return nil
}

// Teardown stops the mock server.
func (p *Pact) Teardown() error {
	if p.mock != nil {
		return p.mock.Stop()
	}
	return nil
}

// MockURL returns the mock server URL.
func (p *Pact) MockURL() string {
	if p.mock != nil {
		return p.mock.URL()
	}
	return ""
}

// TestOption configures a pact test.
type TestOption func(*testConfig)

type testConfig struct {
	outDir string
}

// WithContractDir sets the output directory for contracts.
func WithContractDir(dir string) TestOption {
	return func(cfg *testConfig) {
		cfg.outDir = dir
	}
}

// RunTest runs a consumer test with automatic mock lifecycle management.
func RunTest(t *testing.T, consumer, provider string, interactions []*Interaction, testFunc func(mockURL string), opts ...TestOption) {
	t.Helper()

	cfg := &testConfig{outDir: "./contracts"}
	for _, opt := range opts {
		opt(cfg)
	}

	pact := NewPact(consumer, provider).WithOutputDir(cfg.outDir)
	for _, interaction := range interactions {
		pact.AddInteraction(interaction)
	}

	mockURL, err := pact.Setup()
	if err != nil {
		t.Fatalf("Failed to setup pact: %v", err)
	}
	defer func() {
		if err := pact.Teardown(); err != nil {
			t.Logf("Warning: failed to teardown pact: %v", err)
		}
	}()

	// Run the test
	testFunc(mockURL)

	// Verify and save
	if err := pact.Verify(); err != nil {
		t.Fatalf("Pact verification failed: %v", err)
	}
}
