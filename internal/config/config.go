package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration for the fakepal server.
type Config struct {
	Port   string
	APIKey string
}

// Load reads configuration from environment variables.
// PORT defaults to 8080; API_KEY defaults to "test-api-key".
func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	// Accept bare numbers or already-prefixed values; normalize later in main.
	if _, err := strconv.Atoi(port); err == nil {
		// keep as-is; main will prepend ":"
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = "test-api-key"
	}

	return Config{
		Port:   port,
		APIKey: apiKey,
	}
}
