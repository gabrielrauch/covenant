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

	"github.com/gabrielrauch/covenant/pkg/broker/api"
	"github.com/gabrielrauch/covenant/pkg/contract"
	httpvalidator "github.com/gabrielrauch/covenant/pkg/validator/http"
)

// Verify verifies a provider against contracts.
func Verify(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	brokerURL := fs.String("broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	providerURL := fs.String("provider-url", "", "provider base URL (required)")
	providerName := fs.String("provider", "", "provider name (required)")
	providerVersion := fs.String("provider-version", "", "provider version (required)")
	contractDir := fs.String("contracts", "./contracts", "directory containing contracts")
	publishResults := fs.Bool("publish", true, "publish results to broker")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant verify [options]")
		fmt.Fprintln(os.Stderr, "\nVerifies a provider against contracts.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *providerURL == "" || *providerName == "" || *providerVersion == "" {
		return fmt.Errorf("--provider-url, --provider, and --provider-version are required")
	}

	// Find contract files
	files, err := filepath.Glob(filepath.Join(*contractDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find contract files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no contract files found in %s", *contractDir)
	}

	validator := httpvalidator.NewValidator()
	client := &http.Client{}

	totalPassed := 0
	totalFailed := 0

	for _, file := range files {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", file, err)
			continue
		}

		// Only verify contracts where we are the provider
		if c.Metadata.Provider.Name != *providerName {
			continue
		}

		fmt.Printf("\n=== Verifying contract: %s ===\n", filepath.Base(file))
		fmt.Printf("Consumer: %s, Provider: %s v%s\n", c.Metadata.Consumer.Name, c.Metadata.Provider.Name, c.Metadata.Version)

		result := api.VerificationResult{
			ContractID:      c.Metadata.ID,
			ContractVersion: c.Metadata.Version,
			Provider:        api.ServiceVersion{Name: *providerName, Version: *providerVersion},
			Consumer:        api.ServiceVersion{Name: c.Metadata.Consumer.Name, Version: c.Metadata.Consumer.Version},
			VerifiedAt:      time.Now().UTC(),
		}

		start := time.Now()

		for _, interaction := range c.Interactions {
			if interaction.Protocol != contract.ProtocolHTTP || interaction.Payload.HTTP == nil {
				continue
			}

			fmt.Printf("\n  Testing: %s...", interaction.Description)
			interactionStart := time.Now()

			// Execute the interaction
			resp, err := validator.ExecuteInteraction(ctx, *providerURL, interaction)
			if err != nil {
				fmt.Printf(" ERROR: %v\n", err)
				result.InteractionResults = append(result.InteractionResults, api.InteractionResult{
					ID:          interaction.ID,
					Description: interaction.Description,
					Success:     false,
					DurationMS:  time.Since(interactionStart).Milliseconds(),
					Errors:      []api.InteractionError{{Message: err.Error()}},
				})
				totalFailed++
				continue
			}

			// Validate the response
			validationResult := validator.ValidateResponse(interaction, resp, c.MatchingRules)
			resp.Body.Close()

			interactionResult := api.InteractionResult{
				ID:          interaction.ID,
				Description: interaction.Description,
				Success:     validationResult.Success,
				DurationMS:  time.Since(interactionStart).Milliseconds(),
			}

			if validationResult.Success {
				fmt.Printf(" PASSED (%dms)\n", interactionResult.DurationMS)
				totalPassed++
			} else {
				fmt.Printf(" FAILED\n")
				for _, err := range validationResult.Errors {
					fmt.Printf("    - %s: %s\n", err.Path, err.Message)
					interactionResult.Errors = append(interactionResult.Errors, api.InteractionError{
						Path:     err.Path,
						Expected: err.Expected,
						Actual:   err.Actual,
						Rule:     err.Rule,
						Message:  err.Message,
					})
				}
				totalFailed++
			}

			result.InteractionResults = append(result.InteractionResults, interactionResult)
		}

		result.DurationMS = time.Since(start).Milliseconds()

		// Publish results to broker
		if *publishResults && *brokerURL != "" {
			data, _ := json.Marshal(result)
			req, err := http.NewRequestWithContext(ctx, "POST", *brokerURL+"/verifications", bytes.NewReader(data))
			if err == nil {
				req.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(req)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusCreated {
						fmt.Printf("\nResults published to broker\n")
					}
				}
			}
		}
	}

	fmt.Printf("\n=== Summary ===\n")
	fmt.Printf("Passed: %d, Failed: %d\n", totalPassed, totalFailed)

	if totalFailed > 0 {
		return fmt.Errorf("%d interactions failed", totalFailed)
	}

	return nil
}
