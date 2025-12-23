package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

// publishResult represents the response from publishing a contract.
type publishResult struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Publish publishes contracts to the broker.
func Publish(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	brokerURL := fs.String("broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	tag := fs.String("tag", "", "tag to apply to published contracts")
	dir := fs.String("dir", "./contracts", "directory containing contract files")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant publish [options]")
		fmt.Fprintln(os.Stderr, "\nPublishes contracts from a directory to the broker.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Find contract files
	files, err := filepath.Glob(filepath.Join(*dir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to find contract files: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no contract files found in %s", *dir)
	}

	client := NewHTTPClient(*brokerURL)
	published := 0

	for _, file := range files {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", file, err)
			continue
		}

		ensureTagInContract(c, *tag)

		result, err := publishContract(ctx, client, c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to publish %s: %v\n", file, err)
			continue
		}

		fmt.Printf("Published %s: %s v%s\n", filepath.Base(file), result.ID, result.Version)
		published++
	}

	fmt.Printf("\nPublished %d/%d contracts to %s\n", published, len(files), *brokerURL)
	return nil
}

// ensureTagInContract adds a tag to the contract if not already present.
func ensureTagInContract(c *contract.Contract, tag string) {
	if tag == "" {
		return
	}

	for _, t := range c.Metadata.Tags {
		if t == tag {
			return
		}
	}
	c.Metadata.Tags = append(c.Metadata.Tags, tag)
}

// publishContract sends a contract to the broker and returns the result.
func publishContract(ctx context.Context, client *HTTPClient, c *contract.Contract) (*publishResult, error) {
	respBody, err := client.PostJSON(ctx, "/contracts", c, http.StatusCreated)
	if err != nil {
		return nil, err
	}

	var result publishResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
