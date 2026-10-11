// Package config reads server configuration from the environment.
package config

import (
	"cmp"
	"os"
)

// Config holds everything the server needs to start. All values come from
// environment variables so tests and deployments stay key-free by default.
type Config struct {
	// Addr is the listen address for the HTTP server (WIKIFYP_ADDR).
	Addr string
	// DatabaseURL is the Postgres connection string (WIKIFYP_DATABASE_URL).
	// Empty means the server runs without a database.
	DatabaseURL string
}

// FromEnv builds a Config from the environment, applying defaults.
func FromEnv() Config {
	return Config{
		Addr:        cmp.Or(os.Getenv("WIKIFYP_ADDR"), ":8080"),
		DatabaseURL: os.Getenv("WIKIFYP_DATABASE_URL"),
	}
}
