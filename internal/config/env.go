// Package config provides application configuration helpers.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads local development variables without overriding the process environment.
func LoadDotEnv() error {
	err := godotenv.Load()
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("config: load .env: %w", err)
}
