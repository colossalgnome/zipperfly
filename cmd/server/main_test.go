package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvFile_WithExplicitFlag(t *testing.T) {
	// Create a temporary env file
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "test.env")
	if err := os.WriteFile(envPath, []byte("FLAG_FOO=bar\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	// Ensure variable not set beforehand
	_ = os.Unsetenv("FLAG_FOO")

	loadEnvFile(envPath)

	if got := os.Getenv("FLAG_FOO"); got != "bar" {
		t.Fatalf("expected FLAG_FOO=bar from explicit config file, got %q", got)
	}
}

func TestLoadEnvFile_WithConfigEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env-from-config.env")
	if err := os.WriteFile(envPath, []byte("CONFIG_FOO=baz\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp env file: %v", err)
	}

	_ = os.Unsetenv("CONFIG_FOO")
	_ = os.Setenv("CONFIG_FILE", envPath)
	defer os.Unsetenv("CONFIG_FILE")

	loadEnvFile("")

	if got := os.Getenv("CONFIG_FOO"); got != "baz" {
		t.Fatalf("expected CONFIG_FOO=baz from CONFIG_FILE, got %q", got)
	}
}

func TestLoadEnvFile_DefaultDotEnv(t *testing.T) {
	// Work in an isolated temp dir so we don't collide with any real .env
	tmpDir := t.TempDir()
	origWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	defer os.Chdir(origWD)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir failed: %v", err)
	}

	if err := os.WriteFile(".env", []byte("DOTENV_FOO=qux\n"), 0o600); err != nil {
		t.Fatalf("failed to write .env file: %v", err)
	}

	_ = os.Unsetenv("DOTENV_FOO")
	_ = os.Unsetenv("CONFIG_FILE")

	loadEnvFile("")

	if got := os.Getenv("DOTENV_FOO"); got != "qux" {
		t.Fatalf("expected DOTENV_FOO=qux from .env, got %q", got)
	}
}
