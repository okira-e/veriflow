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

func TestFlowDelete(t *testing.T) {
	t.Run("deletes existing flow", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "veriflow.json")

		cfg := &config.Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{app.NewFlow("to-delete"), app.NewFlow("to-keep")},
		}
		data, _ := json.MarshalIndent(cfg, "", "    ")
		os.WriteFile(configPath, data, 0644)

		originalConfigFile := cliopts.ConfigFile
		originalSilent := cliopts.Silent
		cliopts.ConfigFile = configPath
		cliopts.Silent = true
		defer func() {
			cliopts.ConfigFile = originalConfigFile
			cliopts.Silent = originalSilent
		}()

		err := runDeleteCmd(nil, []string{"to-delete"}, deleteCmdFlags{yes: true, NoSave: false})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		cfg, _ = config.LoadConfig(configPath)
		if _, ok := cfg.GetFlow("to-delete"); ok {
			t.Error("flow should be deleted")
		}
		if _, ok := cfg.GetFlow("to-keep"); !ok {
			t.Error("other flow should remain")
		}
	})

	t.Run("fails for non-existent flow", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "veriflow.json")

		cfg := &config.Cfg{BaseUrl: "http://localhost", Flows: []*app.Flow{}}
		data, _ := json.MarshalIndent(cfg, "", "    ")
		os.WriteFile(configPath, data, 0644)

		originalConfigFile := cliopts.ConfigFile
		cliopts.ConfigFile = configPath
		defer func() { cliopts.ConfigFile = originalConfigFile }()

		err := runDeleteCmd(nil, []string{"nonexistent"}, deleteCmdFlags{yes: true})
		if err == nil {
			t.Error("expected error for non-existent flow")
		}
	})
}
