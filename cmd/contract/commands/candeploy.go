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
)

// CanDeploy checks if a service can be deployed safely.
func CanDeploy(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("can-deploy", flag.ExitOnError)
	brokerURL := fs.String("broker", getEnv("COVENANT_BROKER_URL", "http://localhost:8080"), "broker URL")
	service := fs.String("service", "", "service name (required)")
	version := fs.String("version", "", "service version (required)")
	env := fs.String("environment", "", "environment/tag to check")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant can-deploy [options]")
		fmt.Fprintln(os.Stderr, "\nChecks if a service version can be safely deployed.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *service == "" || *version == "" {
		return fmt.Errorf("--service and --version are required")
	}

	// Build query
	query := url.Values{}
	query.Set("service", *service)
	query.Set("version", *version)
	if *env != "" {
		query.Set("environment", *env)
	}

	checkURL := *brokerURL + "/can-deploy?" + query.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to check deployment: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to check deployment: %s", string(body))
	}

	var result struct {
		OK      bool   `json:"ok"`
		Service string `json:"service"`
		Version string `json:"version"`
		Env     string `json:"environment"`
		Reasons []struct {
			ContractID string `json:"contract_id"`
			Consumer   string `json:"consumer"`
			Provider   string `json:"provider"`
			Status     string `json:"status"`
			Message    string `json:"message"`
		} `json:"reasons"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Display results
	fmt.Printf("Service: %s v%s\n", result.Service, result.Version)
	if result.Env != "" {
		fmt.Printf("Environment: %s\n", result.Env)
	}
	fmt.Println()

	if result.OK {
		fmt.Println("✅ Safe to deploy")
	} else {
		fmt.Println("❌ NOT safe to deploy")
	}

	if len(result.Reasons) > 0 {
		fmt.Println("\nReasons:")
		for _, reason := range result.Reasons {
			status := "✓"
			if reason.Status != "verified" {
				status = "✗"
			}
			fmt.Printf("  %s [%s] %s <-> %s: %s\n", status, reason.Status, reason.Consumer, reason.Provider, reason.Message)
		}
	}

	if !result.OK {
		return fmt.Errorf("deployment blocked")
	}

	return nil
}
