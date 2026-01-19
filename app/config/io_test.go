package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Run("loads valid config with flows and steps", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "veriflow.json")

		configData := `{
			"projectName": "test-project",
			"baseUrl": "http://localhost:8080",
			"beforeRun": ["echo before"],
			"afterRun": ["echo after"],
			"flows": [
				{
					"name": "user-flow",
					"steps": [
						{
							"name": "register",
							"request": {"method": "POST", "path": "/register"},
							"assert": {"status": 201},
							"exports": {"user_id": "$.data.id"}
						}
					]
				}
			]
		}`
		os.WriteFile(configPath, []byte(configData), 0644)

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.ProjectName != "test-project" {
			t.Errorf("expected project name 'test-project', got '%s'", cfg.ProjectName)
		}
		if cfg.BaseUrl != "http://localhost:8080" {
			t.Errorf("expected base URL, got '%s'", cfg.BaseUrl)
		}
		if len(cfg.BeforeRun) != 1 || len(cfg.AfterRun) != 1 {
			t.Error("hooks not loaded")
		}

		// Verify indexing works
		flow, ok := cfg.GetFlow("user-flow")
		if !ok {
			t.Fatal("flow not indexed")
		}
		step, ok := flow.GetStep("register")
		if !ok {
			t.Fatal("step not indexed")
		}
		if step.Exports["user_id"] != "$.data.id" {
			t.Error("exports not loaded")
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/veriflow.json")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "veriflow.json")
		os.WriteFile(configPath, []byte(`not valid json`), 0644)

		_, err := LoadConfig(configPath)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}
