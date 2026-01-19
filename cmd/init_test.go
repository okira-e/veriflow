package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okira-e/veriflow/app/cliopts"
)

func TestInitCmd_CreatesConfigFile(t *testing.T) {
	// Setup: use a temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veriflow.json")

	// Save and restore global state
	originalConfigFile := cliopts.ConfigFile
	originalNonInteractive := cliopts.NonInteractive
	defer func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.NonInteractive = originalNonInteractive
	}()

	cliopts.ConfigFile = configPath
	cliopts.NonInteractive = true

	// Change to temp dir so config check works
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	flags := &initCmdFlags{
		BaseUrl: "http://localhost:8080",
	}

	err := runInitCmd(flags)
	if err != nil {
		t.Fatalf("runInitCmd failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("config file is empty")
	}
}

func TestInitCmd_FailsWithoutBaseUrlInNonInteractive(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veriflow.json")

	originalConfigFile := cliopts.ConfigFile
	originalNonInteractive := cliopts.NonInteractive
	defer func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.NonInteractive = originalNonInteractive
	}()

	cliopts.ConfigFile = configPath
	cliopts.NonInteractive = true

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	flags := &initCmdFlags{
		BaseUrl: "", // No base URL provided
	}

	err := runInitCmd(flags)
	if err == nil {
		t.Fatal("expected error when base-url missing in non-interactive mode")
	}
}

func TestInitCmd_FailsIfConfigExists(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veriflow.json")

	// Create existing config
	os.WriteFile(configPath, []byte(`{"baseUrl": "http://test.com"}`), 0644)

	originalConfigFile := cliopts.ConfigFile
	originalNonInteractive := cliopts.NonInteractive
	defer func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.NonInteractive = originalNonInteractive
	}()

	cliopts.ConfigFile = configPath
	cliopts.NonInteractive = true

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	flags := &initCmdFlags{
		BaseUrl: "http://localhost:8080",
	}

	err := runInitCmd(flags)
	if err == nil {
		t.Fatal("expected error when config already exists")
	}
}

func TestInitCmd_RespectsConfigFlag(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom-config.json")

	originalConfigFile := cliopts.ConfigFile
	originalNonInteractive := cliopts.NonInteractive
	defer func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.NonInteractive = originalNonInteractive
	}()

	cliopts.ConfigFile = customPath
	cliopts.NonInteractive = true

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	flags := &initCmdFlags{
		BaseUrl: "http://localhost:8080",
	}

	err := runInitCmd(flags)
	if err != nil {
		t.Fatalf("runInitCmd failed: %v", err)
	}

	// Verify custom path was used
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("config file was not created at custom path")
	}
}
