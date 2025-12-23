// Package provider provides the provider-side API for contract verification.
package provider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/brokerapi"
	"github.com/gabrielrauch/covenant/pkg/contract"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

// Verifier verifies a provider against contracts.
type Verifier struct {
	providerName    string
	providerVersion string
	baseURL         string
	contracts       []*contract.Contract
	stateHandlers   map[string]StateHandler
	httpValidator   *httpvalidator.Validator
}

// StateHandler sets up provider state before an interaction is verified.
type StateHandler func(state contract.ProviderState) error

// NewVerifier creates a new provider verifier.
func NewVerifier(providerName, providerVersion, baseURL string) *Verifier {
	return &Verifier{
		providerName:    providerName,
		providerVersion: providerVersion,
		baseURL:         baseURL,
		stateHandlers:   make(map[string]StateHandler),
		httpValidator:   httpvalidator.NewValidator(),
	}
}

// AddContract adds a contract to verify against.
func (v *Verifier) AddContract(c *contract.Contract) *Verifier {
	v.contracts = append(v.contracts, c)
	return v
}

// AddContracts adds multiple contracts.
func (v *Verifier) AddContracts(contracts []*contract.Contract) *Verifier {
	v.contracts = append(v.contracts, contracts...)
	return v
}

// OnProviderState registers a handler for a provider state.
func (v *Verifier) OnProviderState(name string, handler StateHandler) *Verifier {
	v.stateHandlers[name] = handler
	return v
}

// VerificationOptions configures verification behavior.
type VerificationOptions struct {
	Timeout       time.Duration
	FailFast      bool
	StateSetupURL string // Optional URL to call for state setup
}

// VerificationReport contains the results of provider verification.
type VerificationReport struct {
	Provider        string
	ProviderVersion string
	VerifiedAt      time.Time
	Success         bool
	Results         []brokerapi.VerificationResult
}

// Verify verifies the provider against all loaded contracts.
func (v *Verifier) Verify(ctx context.Context, opts VerificationOptions) (*VerificationReport, error) {
	if len(v.contracts) == 0 {
		return nil, fmt.Errorf("no contracts to verify")
	}

	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	report := &VerificationReport{
		Provider:        v.providerName,
		ProviderVersion: v.providerVersion,
		VerifiedAt:      time.Now().UTC(),
		Success:         true,
	}

	for _, c := range v.contracts {
		// Only verify contracts where we are the provider
		if c.Metadata.Provider.Name != v.providerName {
			continue
		}

		result := v.verifyContract(ctx, c, opts)
		report.Results = append(report.Results, result)

		if !result.Success {
			report.Success = false
			if opts.FailFast {
				break
			}
		}
	}

	return report, nil
}

// verifyContract verifies a single contract.
func (v *Verifier) verifyContract(ctx context.Context, c *contract.Contract, opts VerificationOptions) brokerapi.VerificationResult {
	start := time.Now()

	result := brokerapi.VerificationResult{
		ContractID:      c.Metadata.ID,
		ContractVersion: c.Metadata.Version,
		Provider:        brokerapi.ServiceVersion{Name: v.providerName, Version: v.providerVersion},
		Consumer:        brokerapi.ServiceVersion{Name: c.Metadata.Consumer.Name, Version: c.Metadata.Consumer.Version},
		VerifiedAt:      time.Now().UTC(),
	}

	for i := range c.Interactions {
		interactionResult := v.verifyInteraction(ctx, &c.Interactions[i], c.MatchingRules, opts)
		result.InteractionResults = append(result.InteractionResults, interactionResult)
	}

	result.DurationMS = time.Since(start).Milliseconds()

	// Compute summary
	for _, ir := range result.InteractionResults {
		result.Summary.Total++
		if ir.Success {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}
	result.Success = result.Summary.Failed == 0

	return result
}

// verifyInteraction verifies a single interaction.
func (v *Verifier) verifyInteraction(ctx context.Context, interaction *contract.Interaction, contractRules contract.MatchingRules, _ VerificationOptions) brokerapi.InteractionResult {
	start := time.Now()

	result := brokerapi.InteractionResult{
		ID:          interaction.ID,
		Description: interaction.Description,
		Success:     true,
	}

	// Setup provider states
	for _, state := range interaction.ProviderStates {
		handler, ok := v.stateHandlers[state.Name]
		if !ok {
			result.Success = false
			result.Errors = append(result.Errors, brokerapi.InteractionError{
				Message: fmt.Sprintf("no handler for provider state: %s", state.Name),
			})
			result.DurationMS = time.Since(start).Milliseconds()
			return result
		}

		if err := handler(state); err != nil {
			result.Success = false
			result.Errors = append(result.Errors, brokerapi.InteractionError{
				Message: fmt.Sprintf("state setup failed for %s: %v", state.Name, err),
			})
			result.DurationMS = time.Since(start).Milliseconds()
			return result
		}
	}

	// Execute the interaction based on protocol
	switch interaction.Protocol {
	case contract.ProtocolHTTP:
		httpResult := v.verifyHTTPInteraction(ctx, interaction, contractRules)
		result.Success = httpResult.Success
		result.Errors = httpResult.Errors
	default:
		result.Success = false
		result.Errors = append(result.Errors, brokerapi.InteractionError{
			Message: fmt.Sprintf("unsupported protocol: %s", interaction.Protocol),
		})
	}

	result.DurationMS = time.Since(start).Milliseconds()
	return result
}

// verifyHTTPInteraction verifies an HTTP interaction.
func (v *Verifier) verifyHTTPInteraction(ctx context.Context, interaction *contract.Interaction, contractRules contract.MatchingRules) brokerapi.InteractionResult {
	result := brokerapi.InteractionResult{
		ID:          interaction.ID,
		Description: interaction.Description,
		Success:     true,
	}

	// Execute the request
	resp, err := v.httpValidator.ExecuteInteraction(ctx, v.baseURL, interaction)
	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, brokerapi.InteractionError{
			Message: fmt.Sprintf("request failed: %v", err),
		})
		return result
	}
	defer resp.Body.Close()

	// Validate the response
	validationResult := v.httpValidator.ValidateResponse(interaction, resp, contractRules)
	if !validationResult.Success {
		result.Success = false
		for _, err := range validationResult.Errors {
			result.Errors = append(result.Errors, brokerapi.InteractionError{
				Path:     err.Path,
				Expected: err.Expected,
				Actual:   err.Actual,
				Rule:     err.Rule,
				Message:  err.Message,
			})
		}
	}

	return result
}

// VerifyWithTesting runs verification as part of a Go test.
func (v *Verifier) VerifyWithTesting(t *testing.T) {
	t.Helper()

	ctx := context.Background()
	opts := VerificationOptions{
		Timeout: 30 * time.Second,
	}

	report, err := v.Verify(ctx, opts)
	if err != nil {
		t.Fatalf("Verification failed: %v", err)
	}

	for i := range report.Results {
		result := &report.Results[i]
		t.Run(fmt.Sprintf("%s/%s", result.Consumer.Name, result.ContractVersion), func(t *testing.T) {
			for j := range result.InteractionResults {
				ir := &result.InteractionResults[j]
				t.Run(ir.Description, func(t *testing.T) {
					if !ir.Success {
						for _, err := range ir.Errors {
							t.Errorf("%s: %s", err.Path, err.Message)
						}
					}
				})
			}
		})
	}

	if !report.Success {
		t.Fail()
	}
}
