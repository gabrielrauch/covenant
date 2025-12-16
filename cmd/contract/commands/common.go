// Package commands provides command implementations for the contract tool.
package commands

import "os"

// getEnv returns the value of an environment variable or a default.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
