package step

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/spf13/cobra"
)

func TestRunAddCmd(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "veriflow.json")

	// Create initial config with a flow
	cfg := &config.Cfg{
		ProjectName: "test-project",
		BaseUrl:     "http://localhost:8080",
		Flows: []*app.Flow{
			app.NewFlow("test-flow"),
		},
	}

	// Save initial config
	configJson, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		t.Fatalf("failed to marshal initial config: %v", err)
	}
	if err := os.WriteFile(configPath, configJson, 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	// Set the config file path
	originalConfigFile := cliopts.ConfigFile
	cliopts.ConfigFile = configPath
	cliopts.NonInteractive = true
	defer func() {
		cliopts.ConfigFile = originalConfigFile
		cliopts.NonInteractive = false
	}()

	// Test case 1: Add step with minimal flags
	t.Run("add step with minimal flags", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "POST",
			Path:   "/users/register",
			Status: 201,
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"register"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("register")
		if !ok {
			t.Fatal("step not found")
		}

		if step.Request.Method != "POST" {
			t.Errorf("expected method POST, got %s", step.Request.Method)
		}
		if step.Request.Path != "/users/register" {
			t.Errorf("expected path /users/register, got %s", step.Request.Path)
		}
		if step.Assert.Status != 201 {
			t.Errorf("expected status 201, got %d", step.Assert.Status)
		}
	})

	// Test case 2: Add step with JSON body
	t.Run("add step with JSON body", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "PUT",
			Path:   "/users/123",
			Json:   `{"name":"John","email":"john@example.com"}`,
			Status: 200,
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"update-user"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with JSON
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("update-user")
		if !ok {
			t.Fatal("step not found")
		}

		if !step.Request.Json.IsSome() {
			t.Error("expected JSON to be set")
		}

		jsonData := step.Request.Json.Unwrap()
		if jsonData["name"] != "John" {
			t.Errorf("expected name John, got %v", jsonData["name"])
		}
		if jsonData["email"] != "john@example.com" {
			t.Errorf("expected email john@example.com, got %v", jsonData["email"])
		}
	})

	// Test case 3: Add step with assertions
	t.Run("add step with assertions", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "GET",
			Path:   "/users/123",
			Status: 200,
			AssertExpressions: []string{
				"exists $.id",
				"equals $.name John",
				"contains $.email @",
			},
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"get-user"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with assertions
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("get-user")
		if !ok {
			t.Fatal("step not found")
		}

		if !step.Assert.All.IsSome() {
			t.Fatal("expected assertions to be set")
		}

		assertions := step.Assert.All.Unwrap()
		if len(assertions) != 3 {
			t.Fatalf("expected 3 assertions, got %d", len(assertions))
		}

		// Check first assertion (exists)
		if assertions[0].JsonPath != "$.id" {
			t.Errorf("expected jsonpath $.id, got %s", assertions[0].JsonPath)
		}
		if !assertions[0].Exists {
			t.Error("expected exists to be true")
		}

		// Check second assertion (equals)
		if assertions[1].JsonPath != "$.name" {
			t.Errorf("expected jsonpath $.name, got %s", assertions[1].JsonPath)
		}
		if !assertions[1].Equals.IsSome() {
			t.Error("expected equals to be set")
		}
		if assertions[1].Equals.Unwrap() != "John" {
			t.Errorf("expected equals value John, got %s", assertions[1].Equals.Unwrap())
		}

		// Check third assertion (contains)
		if assertions[2].JsonPath != "$.email" {
			t.Errorf("expected jsonpath $.email, got %s", assertions[2].JsonPath)
		}
		if !assertions[2].Contains.IsSome() {
			t.Error("expected contains to be set")
		}
		if assertions[2].Contains.Unwrap() != "@" {
			t.Errorf("expected contains value @, got %s", assertions[2].Contains.Unwrap())
		}
	})

	// Test case 4: Add step with exports
	t.Run("add step with exports", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "POST",
			Path:   "/users/register",
			Status: 201,
			ExportExpressions: []string{
				"user_id $.data.user_id",
				"token $.data.token",
			},
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"register-with-exports"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with exports
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("register-with-exports")
		if !ok {
			t.Fatal("step not found")
		}

		if len(step.Exports) != 2 {
			t.Fatalf("expected 2 exports, got %d", len(step.Exports))
		}

		if step.Exports["user_id"] != "$.data.user_id" {
			t.Errorf("expected user_id export to be $.data.user_id, got %s", step.Exports["user_id"])
		}

		if step.Exports["token"] != "$.data.token" {
			t.Errorf("expected token export to be $.data.token, got %s", step.Exports["token"])
		}
	})
}
