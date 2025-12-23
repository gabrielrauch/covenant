// Command contract is the CLI tool for covenant contract testing.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gabrielrauch/covenant/cmd/contract/commands"
)

const version = "0.1.0"

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return printUsage()
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "publish":
		return commands.Publish(ctx, cmdArgs)
	case "fetch":
		return commands.Fetch(ctx, cmdArgs)
	case "verify":
		return commands.Verify(ctx, cmdArgs)
	case "can-deploy":
		return commands.CanDeploy(ctx, cmdArgs)
	case "tag":
		return commands.Tag(ctx, cmdArgs)
	case "broker":
		return commands.Broker(ctx, cmdArgs)
	case "version":
		fmt.Printf("covenant %s\n", version)
		return nil
	case "help", "-h", "--help":
		return printUsage()
	default:
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func printUsage() error {
	fmt.Print(`covenant - Consumer-driven contract testing for Go

Usage:
  covenant <command> [options]

Commands:
  publish     Publish contracts to the broker
  fetch       Fetch contracts from the broker
  verify      Verify a provider against contracts
  can-deploy  Check if a service can be safely deployed
  tag         Manage contract tags
  broker      Start the broker server
  version     Print version information

Run 'covenant <command> -h' for more information about a command.
`)
	return nil
}
