package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gabrielrauch/covenant/pkg/contract"
)

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

	client := &http.Client{}
	published := 0

	for _, file := range files {
		c, err := contract.LoadFromFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Skipping %s: %v\n", file, err)
			continue
		}

		// Add tag if specified
		if *tag != "" {
			hasTag := false
			for _, t := range c.Metadata.Tags {
				if t == *tag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				c.Metadata.Tags = append(c.Metadata.Tags, *tag)
			}
		}

		// Publish to broker
		data, err := json.Marshal(c)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to serialize %s: %v\n", file, err)
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", *brokerURL+"/contracts", bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to publish %s: %v\n", file, err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			fmt.Fprintf(os.Stderr, "Failed to publish %s: %s\n", file, string(body))
			continue
		}

		var result struct {
			ID      string `json:"id"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			fmt.Printf("Published %s: %s v%s\n", filepath.Base(file), result.ID, result.Version)
		}
		published++
	}

	fmt.Printf("\nPublished %d/%d contracts to %s\n", published, len(files), *brokerURL)
	return nil
}
