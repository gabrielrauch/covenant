package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gabrielrauch/covenant/pkg/broker/brokerapi"
	"github.com/gabrielrauch/covenant/pkg/contract"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

type verifyConfig struct {
	brokerURL       string
	providerURL     string
	providerName    string
	providerVersion string
	contractDir     string
	publishResults  bool
}

// Verify verifies a provider against contracts.
func Verify(ctx context.Context, args []string) error {
	cfg, err := parseVerifyFlags(args)
	if err != nil {
		return err
	}

	files, err := findContractFiles(cfg.contractDir)
	if err != nil {
		return err
	}

	validator := httpvalidator.NewValidator()
	totalPassed, totalFailed := 0, 0

	for _, file := range files {
		passed, failed, err := verifyContractFile(ctx, cfg, validator, file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", file, err)
			continue
		}
		totalPassed += passed
		totalFailed += failed
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Passed: %d, Failed: %d\n", totalPassed, totalFailed)

	if totalFailed > 0 {
		return fmt.Errorf("%d interactions failed", totalFailed)
	}

	return nil
}

func parseVerifyFlags(args []string) (*verifyConfig, error) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	cfg := &verifyConfig{}

	fs.StringVar(&cfg.brokerURL, "broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	fs.StringVar(&cfg.providerURL, "provider-url", "", "provider base URL (required)")
	fs.StringVar(&cfg.providerName, "provider", "", "provider name (required)")
	fs.StringVar(&cfg.providerVersion, "provider-version", "", "provider version (required)")
	fs.StringVar(&cfg.contractDir, "contracts", "./contracts", "directory containing contracts")
	fs.BoolVar(&cfg.publishResults, "publish", true, "publish results to broker")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant verify [options]")
		fmt.Fprintln(os.Stderr, "\nVerifies a provider against contracts.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if cfg.providerURL == "" || cfg.providerName == "" || cfg.providerVersion == "" {
		return nil, fmt.Errorf("--provider-url, --provider, and --provider-version are required")
	}

	return cfg, nil
}

func findContractFiles(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to find contract files: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no contract files found in %s", dir)
	}
	return files, nil
}

func verifyContractFile(ctx context.Context, cfg *verifyConfig, validator *httpvalidator.Validator, file string) (passed, failed int, err error) {
	c, err := contract.LoadFromFile(file)
	if err != nil {
		return 0, 0, err
	}

	if c.Metadata.Provider.Name != cfg.providerName {
		return 0, 0, nil
	}

	fmt.Printf("\n=== Verifying contract: %s ===\n", filepath.Base(file))
	fmt.Printf("Consumer: %s, Provider: %s v%s\n", c.Metadata.Consumer.Name, c.Metadata.Provider.Name, c.Metadata.Version)

	result := brokerapi.VerificationResult{
		ContractID:      c.Metadata.ID,
		ContractVersion: c.Metadata.Version,
		Provider:        brokerapi.ServiceVersion{Name: cfg.providerName, Version: cfg.providerVersion},
		Consumer:        brokerapi.ServiceVersion{Name: c.Metadata.Consumer.Name, Version: c.Metadata.Consumer.Version},
		VerifiedAt:      time.Now().UTC(),
	}

	start := time.Now()
	passed, failed = verifyInteractions(ctx, cfg, validator, c, &result)
	result.DurationMS = time.Since(start).Milliseconds()

	if cfg.publishResults && cfg.brokerURL != "" {
		publishVerificationResults(ctx, cfg.brokerURL, &result)
	}

	return passed, failed, nil
}

func verifyInteractions(ctx context.Context, cfg *verifyConfig, validator *httpvalidator.Validator, c *contract.Contract, result *brokerapi.VerificationResult) (passed, failed int) {
	for i := range c.Interactions {
		interaction := &c.Interactions[i]
		if interaction.Protocol != contract.ProtocolHTTP || interaction.Payload.HTTP == nil {
			continue
		}

		interactionResult := verifyInteraction(ctx, cfg.providerURL, validator, interaction, c.MatchingRules)
		result.InteractionResults = append(result.InteractionResults, interactionResult)

		if interactionResult.Success {
			passed++
		} else {
			failed++
		}
	}
	return passed, failed
}

func verifyInteraction(ctx context.Context, providerURL string, validator *httpvalidator.Validator, interaction *contract.Interaction, rules contract.MatchingRules) brokerapi.InteractionResult {
	fmt.Printf("\n  Testing: %s...", interaction.Description)
	start := time.Now()

	resp, err := validator.ExecuteInteraction(ctx, providerURL, interaction)
	if err != nil {
		fmt.Printf(" ERROR: %v\n", err)
		return brokerapi.InteractionResult{
			ID:          interaction.ID,
			Description: interaction.Description,
			Success:     false,
			DurationMS:  time.Since(start).Milliseconds(),
			Errors:      []brokerapi.InteractionError{{Message: err.Error()}},
		}
	}
	defer resp.Body.Close()

	validationResult := validator.ValidateResponse(interaction, resp, rules)
	interactionResult := brokerapi.InteractionResult{
		ID:          interaction.ID,
		Description: interaction.Description,
		Success:     validationResult.Success,
		DurationMS:  time.Since(start).Milliseconds(),
	}

	if validationResult.Success {
		fmt.Printf(" PASSED (%dms)\n", interactionResult.DurationMS)
	} else {
		fmt.Printf(" FAILED\n")
		for _, e := range validationResult.Errors {
			fmt.Printf("    - %s: %s\n", e.Path, e.Message)
			interactionResult.Errors = append(interactionResult.Errors, brokerapi.InteractionError{
				Path:     e.Path,
				Expected: e.Expected,
				Actual:   e.Actual,
				Rule:     e.Rule,
				Message:  e.Message,
			})
		}
	}

	return interactionResult
}

func publishVerificationResults(ctx context.Context, brokerURL string, result *brokerapi.VerificationResult) {
	data, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to marshal results: %v\n", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", brokerURL+"/verifications", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create request: %v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req) //nolint:gosec // URL from user-configured broker
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to publish results: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("\nResults published to broker\n")
	}
}
