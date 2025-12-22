package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

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
	listURL := *brokerURL + "/contracts"
	if len(query) > 0 {
		listURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to list contracts: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to list contracts: status %d (unable to read body: %w)", resp.StatusCode, err)
		}
		return fmt.Errorf("failed to list contracts: %s", string(body))
	}

	var summaries []struct {
		ID       string `json:"id"`
		Consumer string `json:"consumer"`
		Provider string `json:"provider"`
		Version  string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&summaries); err != nil {
		return fmt.Errorf("failed to parse contract list: %w", err)
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
		getURL := fmt.Sprintf("%s/contracts/%s", *brokerURL, summary.ID)
		req, err := http.NewRequestWithContext(ctx, "GET", getURL, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create request for %s: %v\n", summary.ID, err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch %s: %v\n", summary.ID, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to fetch %s: status %d (unable to read body)\n", summary.ID, resp.StatusCode)
			} else {
				fmt.Fprintf(os.Stderr, "Failed to fetch %s: %s\n", summary.ID, string(body))
			}
			continue
		}

		var c contract.Contract
		if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
			resp.Body.Close()
			fmt.Fprintf(os.Stderr, "Failed to parse %s: %v\n", summary.ID, err)
			continue
		}
		resp.Body.Close()

		// Save to file
		filename := fmt.Sprintf("%s-%s-%s.json", summary.Consumer, summary.Provider, summary.Version)
		outPath := filepath.Join(*outDir, filename)

		if err := contract.SaveToFile(&c, outPath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to save %s: %v\n", summary.ID, err)
			continue
		}

		fmt.Printf("Fetched %s -> %s\n", summary.ID, filename)
		fetched++
	}

	fmt.Printf("\nFetched %d/%d contracts to %s\n", fetched, len(summaries), *outDir)
	return nil
}
