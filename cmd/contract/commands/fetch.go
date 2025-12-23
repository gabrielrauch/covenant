package commands

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// contractSummary represents a contract summary from the broker.
type contractSummary struct {
	ID       string `json:"id"`
	Consumer string `json:"consumer"`
	Provider string `json:"provider"`
	Version  string `json:"version"`
}

// Fetch fetches contracts from the broker.
func Fetch(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ExitOnError)
	brokerURL := fs.String("broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	provider := fs.String("provider", "", "filter by provider name")
	consumer := fs.String("consumer", "", "filter by consumer name")
	tag := fs.String("tag", "", "filter by tag")
	outDir := fs.String("out", "./contracts", "output directory")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant fetch [options]")
		fmt.Fprintln(os.Stderr, "\nFetches contracts from the broker to a local directory.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := NewHTTPClient(*brokerURL)

	// Build query
	query := url.Values{}
	if *provider != "" {
		query.Set("provider", *provider)
	}
	if *consumer != "" {
		query.Set("consumer", *consumer)
	}
	if *tag != "" {
		query.Set("tag", *tag)
	}

	// List contracts
	summaries, err := fetchContractList(ctx, client, query)
	if err != nil {
		return err
	}

	if len(summaries) == 0 {
		fmt.Println("No contracts found matching the criteria")
		return nil
	}

	// Create output directory
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Fetch each contract
	fetched := 0
	for _, summary := range summaries {
		if err := fetchAndSaveContract(ctx, client, *outDir, summary); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			continue
		}
		fetched++
	}

	fmt.Printf("\nFetched %d/%d contracts to %s\n", fetched, len(summaries), *outDir)
	return nil
}

// fetchContractList retrieves the list of contracts from the broker.
func fetchContractList(ctx context.Context, client *HTTPClient, query url.Values) ([]contractSummary, error) {
	var summaries []contractSummary
	if err := client.GetJSON(ctx, "/contracts", query, &summaries); err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}
	return summaries, nil
}

// fetchAndSaveContract fetches a single contract and saves it to disk.
func fetchAndSaveContract(ctx context.Context, client *HTTPClient, outDir string, summary contractSummary) error {
	var c contract.Contract
	path := fmt.Sprintf("/contracts/%s", summary.ID)
	if err := client.GetJSON(ctx, path, nil, &c); err != nil {
		return fmt.Errorf("failed to fetch %s: %w", summary.ID, err)
	}

	filename := fmt.Sprintf("%s-%s-%s.json", summary.Consumer, summary.Provider, summary.Version)
	outPath := filepath.Join(outDir, filename)

	if err := contract.SaveToFile(&c, outPath); err != nil {
		return fmt.Errorf("failed to save %s: %w", summary.ID, err)
	}

	fmt.Printf("Fetched %s -> %s\n", summary.ID, filename)
	return nil
}
