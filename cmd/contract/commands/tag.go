package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Tag manages contract tags.
func Tag(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("tag", flag.ExitOnError)
	brokerURL := fs.String("broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	contractID := fs.String("contract", "", "contract ID (required)")
	addTags := fs.String("add", "", "comma-separated tags to add")
	removeTags := fs.String("remove", "", "comma-separated tags to remove")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant tag [options]")
		fmt.Fprintln(os.Stderr, "\nAdd or remove tags from contracts.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *contractID == "" {
		return fmt.Errorf("--contract is required")
	}

	if *addTags == "" && *removeTags == "" {
		return fmt.Errorf("either --add or --remove is required")
	}

	client := &http.Client{}

	// Add tags
	if *addTags != "" {
		tags := strings.Split(*addTags, ",")
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}

		data, err := json.Marshal(map[string][]string{"tags": tags})
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/contracts/%s/tags", *brokerURL, *contractID), bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to add tags: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("failed to add tags: status %d", resp.StatusCode)
		}

		fmt.Printf("Added tags: %s\n", *addTags)
	}

	// Remove tags
	if *removeTags != "" {
		tags := strings.TrimSpace(*removeTags)
		req, err := http.NewRequestWithContext(ctx, "DELETE", fmt.Sprintf("%s/contracts/%s/tags?tags=%s", *brokerURL, *contractID, tags), http.NoBody)
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to remove tags: %w", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("failed to remove tags: status %d", resp.StatusCode)
		}

		fmt.Printf("Removed tags: %s\n", *removeTags)
	}

	return nil
}
