package flow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
)

func setupTestConfig(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veriflow.json")

	cfg := &config.Cfg{
		ProjectName: "test",
		BaseUrl:     "http://localhost",
		Flows:       []*app.Flow{},
	}

	data, _ := json.MarshalIndent(cfg, "", "    ")
	os.WriteFile(configPath, data, 0644)

	originalConfigFile := cliopts.ConfigFile
	originalSilent := cliopts.Silent
	cliopts.ConfigFile = configPath
	cliopts.Silent = true

	cleanup := func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.Silent = originalSilent
	}

	return configPath, cleanup
}

func TestFlowAdd(t *testing.T) {
	t.Run("adds flow and rejects duplicate", func(t *testing.T) {
		configPath, cleanup := setupTestConfig(t)
		defer cleanup()

		// Add first flow
		err := runAddCmd(nil, []string{"user-onboarding"}, addCmdFlags{NoSave: false})
		if err != nil {
			t.Fatalf("first add failed: %v", err)
		}

		// Verify it exists
		cfg, _ := config.LoadConfig(configPath)
		if _, ok := cfg.GetFlow("user-onboarding"); !ok {
			t.Error("flow was not added")
		}

		// Duplicate should fail
		err = runAddCmd(nil, []string{"user-onboarding"}, addCmdFlags{NoSave: false})
		if err == nil {
			t.Error("expected error for duplicate flow")
		}
	})
}
