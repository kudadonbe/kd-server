package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvPreservesProcessEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(envPath, []byte("EXISTING=value-from-file\nNEW_VALUE=loaded\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalDir)
	})

	t.Setenv("EXISTING", "process-value")
	originalNewValue, hadNewValue := os.LookupEnv("NEW_VALUE")
	if err := os.Unsetenv("NEW_VALUE"); err != nil {
		t.Fatalf("unset NEW_VALUE: %v", err)
	}
	t.Cleanup(func() {
		if hadNewValue {
			_ = os.Setenv("NEW_VALUE", originalNewValue)
			return
		}
		_ = os.Unsetenv("NEW_VALUE")
	})

	if err := LoadDotEnv(); err != nil {
		t.Fatalf("load .env: %v", err)
	}
	if got := os.Getenv("EXISTING"); got != "process-value" {
		t.Fatalf("existing value overridden: got %q", got)
	}
	if got := os.Getenv("NEW_VALUE"); got != "loaded" {
		t.Fatalf("new value not loaded: got %q", got)
	}
}
