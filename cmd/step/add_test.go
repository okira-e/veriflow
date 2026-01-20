package step

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/spf13/cobra"
)

func TestStepAddCmd(t *testing.T) {
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
	cliopts.Silent = true
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
		if !assertions[0].Exists.IsSome() {
			t.Error("expected exists to be set")
		}
		if assertions[0].Exists.Unwrap() == false {
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

	// Test case 4: Add step with isNot assertion
	t.Run("add step with isNot assertion", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "GET",
			Path:   "/users/123",
			Status: 200,
			AssertExpressions: []string{
				"isNot $.status deleted",
				"isNot $.role admin",
			},
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"check-user-status"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with isNot assertions
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("check-user-status")
		if !ok {
			t.Fatal("step not found")
		}

		if !step.Assert.All.IsSome() {
			t.Fatal("expected assertions to be set")
		}

		assertions := step.Assert.All.Unwrap()
		if len(assertions) != 2 {
			t.Fatalf("expected 2 assertions, got %d", len(assertions))
		}

		// Check first isNot assertion
		if assertions[0].JsonPath != "$.status" {
			t.Errorf("expected jsonpath $.status, got %s", assertions[0].JsonPath)
		}
		if !assertions[0].IsNot.IsSome() {
			t.Error("expected isNot to be set")
		}
		if assertions[0].IsNot.Unwrap() != "deleted" {
			t.Errorf("expected isNot value deleted, got %s", assertions[0].IsNot.Unwrap())
		}

		// Check second isNot assertion
		if assertions[1].JsonPath != "$.role" {
			t.Errorf("expected jsonpath $.role, got %s", assertions[1].JsonPath)
		}
		if !assertions[1].IsNot.IsSome() {
			t.Error("expected isNot to be set")
		}
		if assertions[1].IsNot.Unwrap() != "admin" {
			t.Errorf("expected isNot value admin, got %s", assertions[1].IsNot.Unwrap())
		}
	})

	// Test case 5: Add step with XML body
	t.Run("add step with XML body", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "POST",
			Path:   "/api/users",
			Xml:    `<user><name>John</name><email>john@example.com</email></user>`,
			Status: 201,
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"create-user-xml"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with XML
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("create-user-xml")
		if !ok {
			t.Fatal("step not found")
		}

		if !step.Request.Xml.IsSome() {
			t.Error("expected XML to be set")
		}

		xmlData := step.Request.Xml.Unwrap()
		if !strings.Contains(xmlData, "<user>") {
			t.Errorf("expected XML data, got %s", xmlData)
		}
	})

	// Test case 6: Add step with XPath assertions
	t.Run("add step with XPath assertions", func(t *testing.T) {
		flags := addCmdFlags{
			Flow:   "test-flow",
			Method: "GET",
			Path:   "/api/users/123",
			Status: 200,
			AssertExpressions: []string{
				"exists /user/id",
				"equals /user/name John",
				"contains /user/email @",
			},
		}

		cmd := &cobra.Command{Use: "add"}

		err := runAddCmd(cmd, []string{"get-user-xml"}, flags)
		if err != nil {
			t.Fatalf("failed to add step: %v", err)
		}

		// Verify the step was added with XPath assertions
		loadedCfg, err := config.LoadConfig(configPath)
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}

		flow, ok := loadedCfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found")
		}

		step, ok := flow.GetStep("get-user-xml")
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
		if assertions[0].XPath != "/user/id" {
			t.Errorf("expected xpath /user/id, got %s", assertions[0].XPath)
		}
		if !assertions[0].Exists.IsSome() {
			t.Error("expected exists to be set")
		}

		// Check second assertion (equals)
		if assertions[1].XPath != "/user/name" {
			t.Errorf("expected xpath /user/name, got %s", assertions[1].XPath)
		}
		if !assertions[1].Equals.IsSome() {
			t.Error("expected equals to be set")
		}
		if assertions[1].Equals.Unwrap() != "John" {
			t.Errorf("expected equals value John, got %s", assertions[1].Equals.Unwrap())
		}

		// Check third assertion (contains)
		if assertions[2].XPath != "/user/email" {
			t.Errorf("expected xpath /user/email, got %s", assertions[2].XPath)
		}
		if !assertions[2].Contains.IsSome() {
			t.Error("expected contains to be set")
		}
	})

	// Test case 7: Add step with exports
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

func TestBuildAssertObjectFromExpressions_XPath(t *testing.T) {
	t.Run("XPath exists", func(t *testing.T) {
		assertions, err := BuildAssertObjectFromExpressions([]string{
			"exists /user/id",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(assertions) != 1 {
			t.Fatalf("expected 1 assertion, got %d", len(assertions))
		}

		if assertions[0].XPath != "/user/id" {
			t.Errorf("expected XPath /user/id, got %s", assertions[0].XPath)
		}
		if assertions[0].JsonPath != "" {
			t.Errorf("expected empty JsonPath, got %s", assertions[0].JsonPath)
		}
		if !assertions[0].Exists.IsSome() || !assertions[0].Exists.Unwrap() {
			t.Error("expected Exists to be true")
		}
	})

	t.Run("XPath equals", func(t *testing.T) {
		assertions, err := BuildAssertObjectFromExpressions([]string{
			"equals /response/status success",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(assertions) != 1 {
			t.Fatalf("expected 1 assertion, got %d", len(assertions))
		}

		if assertions[0].XPath != "/response/status" {
			t.Errorf("expected XPath /response/status, got %s", assertions[0].XPath)
		}
		if !assertions[0].Equals.IsSome() || assertions[0].Equals.Unwrap() != "success" {
			t.Errorf("expected Equals to be 'success', got %v", assertions[0].Equals)
		}
	})

	t.Run("XPath contains", func(t *testing.T) {
		assertions, err := BuildAssertObjectFromExpressions([]string{
			"contains /user/email @example.com",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if assertions[0].XPath != "/user/email" {
			t.Errorf("expected XPath /user/email, got %s", assertions[0].XPath)
		}
		if !assertions[0].Contains.IsSome() || assertions[0].Contains.Unwrap() != "@example.com" {
			t.Errorf("expected Contains to be '@example.com', got %v", assertions[0].Contains)
		}
	})

	t.Run("XPath isNot", func(t *testing.T) {
		assertions, err := BuildAssertObjectFromExpressions([]string{
			"isNot /user/role guest",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if assertions[0].XPath != "/user/role" {
			t.Errorf("expected XPath /user/role, got %s", assertions[0].XPath)
		}
		if !assertions[0].IsNot.IsSome() || assertions[0].IsNot.Unwrap() != "guest" {
			t.Errorf("expected IsNot to be 'guest', got %v", assertions[0].IsNot)
		}
	})

	t.Run("Mixed JSONPath and XPath", func(t *testing.T) {
		assertions, err := BuildAssertObjectFromExpressions([]string{
			"exists $.data.token",
			"equals /response/status ok",
			"contains $.user.name John",
			"isNot /error/code 500",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(assertions) != 4 {
			t.Fatalf("expected 4 assertions, got %d", len(assertions))
		}

		// First is JSONPath
		if assertions[0].JsonPath != "$.data.token" {
			t.Errorf("expected JSONPath $.data.token, got %s", assertions[0].JsonPath)
		}
		if assertions[0].XPath != "" {
			t.Errorf("expected empty XPath for JSONPath assertion")
		}

		// Second is XPath
		if assertions[1].XPath != "/response/status" {
			t.Errorf("expected XPath /response/status, got %s", assertions[1].XPath)
		}
		if assertions[1].JsonPath != "" {
			t.Errorf("expected empty JSONPath for XPath assertion")
		}

		// Third is JSONPath
		if assertions[2].JsonPath != "$.user.name" {
			t.Errorf("expected JSONPath $.user.name, got %s", assertions[2].JsonPath)
		}

		// Fourth is XPath
		if assertions[3].XPath != "/error/code" {
			t.Errorf("expected XPath /error/code, got %s", assertions[3].XPath)
		}
	})

	t.Run("Invalid path format", func(t *testing.T) {
		_, err := BuildAssertObjectFromExpressions([]string{
			"exists invalid.path",
		})
		if err == nil {
			t.Error("expected error for invalid path format")
		}
	})
}
