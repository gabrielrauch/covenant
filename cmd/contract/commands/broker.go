package commands

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/gabrielrauch/covenant/pkg/broker"
)

// Broker starts the broker server.
func Broker(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("broker", flag.ExitOnError)
	addr := fs.String("addr", getEnv("COVENANT_BROKER_ADDR", ":8080"), "server address")
	storageDir := fs.String("storage", getEnv("COVENANT_STORAGE_DIR", "./data"), "storage directory")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: covenant broker [options]")
		fmt.Fprintln(os.Stderr, "\nStarts the contract broker server.")
		fmt.Fprintln(os.Stderr, "\nOptions:")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := broker.Config{
		StorageDir: *storageDir,
		Addr:       *addr,
	}

	return broker.Run(ctx, cfg)
}
