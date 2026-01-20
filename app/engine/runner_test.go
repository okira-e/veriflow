package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	. "github.com/okira-e/veriflow/app/opt"
)

func TestRunner_BasicExecution(t *testing.T) {
	requestReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer server.Close()

	cfg := &config.Cfg{
		BaseUrl: server.URL,
		Flows:   []*app.Flow{},
	}

	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test-step",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !requestReceived {
		t.Error("expected request to be received by server")
	}

	if runner.StepsRan() != 1 {
		t.Errorf("expected 1 step ran, got %d", runner.StepsRan())
	}
}

func TestRunner_BaseUrlOverride(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not hit server1")
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"server": "2"})
	}))
	defer server2.Close()

	cfg := &config.Cfg{
		BaseUrl: server1.URL,
		Flows:   []*app.Flow{},
	}

	runner := NewRunner(RunnerSettings{
		Cfg:             cfg,
		BaseUrlOverride: server2.URL,
	})

	step := app.NewStep("test",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRunner_BuiltInBindings(t *testing.T) {
	var capturedEmail string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var data map[string]any
		json.Unmarshal(body, &data)

		if email, ok := data["email"].(string); ok {
			capturedEmail = email
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("POST", "/register", map[string]any{
			"email": "test-{{RUN_ID}}@example.com",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify RUN_ID was replaced
	if strings.Contains(capturedEmail, "{{RUN_ID}}") {
		t.Error("RUN_ID was not replaced in email")
	}

	expectedPrefix := "test-"
	expectedSuffix := "@example.com"
	if !strings.HasPrefix(capturedEmail, expectedPrefix) || !strings.HasSuffix(capturedEmail, expectedSuffix) {
		t.Errorf("email format incorrect: %s", capturedEmail)
	}

	// Verify RUN_ID is consistent
	if capturedEmail != fmt.Sprintf("test-%s@example.com", runner.RunId) {
		t.Error("RUN_ID mismatch")
	}
}

func TestRunner_ExportsAndBindings(t *testing.T) {
	var step2Token string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"user_id": "u123",
				"token":   "secret-token",
			})
		case "/profile":
			body, _ := io.ReadAll(r.Body)
			var data map[string]any
			json.Unmarshal(body, &data)
			step2Token = data["token"].(string)

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"name": "Test User"})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Step 1: Register and export token
	step1 := app.NewStep("register",
		app.NewRequest("POST", "/register", map[string]any{
			"email": "test@example.com",
		}),
		app.NewAssert(201, None[[]*app.Assertion]()),
		app.Exports{
			"user_token": "$.token",
		},
	)

	_, err := runner.Execute(step1)
	if err != nil {
		t.Fatalf("step1 failed: %v", err)
	}

	// Step 2: Use exported token
	step2 := app.NewStep("get-profile",
		app.NewRequest("POST", "/profile", map[string]any{
			"token": "{{bind:user_token}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err = runner.Execute(step2)
	if err != nil {
		t.Fatalf("step2 failed: %v", err)
	}

	// Verify token was passed correctly
	if step2Token != "secret-token" {
		t.Errorf("expected token 'secret-token', got '%s'", step2Token)
	}
}

func TestRunner_DynamicPathBinding(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		if r.URL.Path == "/users/register" {
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"user_id": "u456"})
		} else if strings.HasPrefix(r.URL.Path, "/users/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"name": "User"})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Step 1: Export user_id
	step1 := app.NewStep("register",
		app.NewRequest("POST", "/users/register", nil),
		app.NewAssert(201, None[[]*app.Assertion]()),
		app.Exports{"user_id": "$.user_id"},
	)

	runner.Execute(step1)

	// Step 2: Use user_id in path
	step2 := app.NewStep("get-user",
		app.NewRequest("GET", "/users/{{bind:user_id}}", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("step2 failed: %v", err)
	}

	if capturedPath != "/users/u456" {
		t.Errorf("expected path '/users/u456', got '%s'", capturedPath)
	}
}

func TestRunner_NestedExports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"user": map[string]any{
					"profile": map[string]any{
						"id": "nested-123",
					},
				},
			},
		})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("GET", "/data", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"profile_id": "$.data.user.profile.id",
		},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Export captured in runner's internal symtable
	// Test it by using it in a subsequent step
	step2 := app.NewStep("verify-export",
		app.NewRequest("POST", "/verify", map[string]any{
			"id": "{{bind:profile_id}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	// If the binding resolves correctly, the export worked
	_, err = runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected binding to resolve, got %v", err)
	}
}

func TestRunner_AssertionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
		})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{
				JsonPath: "$.status",
				Exists:   Some(true),
				Equals:   Some("success"),
				Contains: None[string](),
			},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected assertion failure, got nil")
	}

	var assertionFailure *AssertionFailure
	if !errors.As(err, &assertionFailure) {
		t.Errorf("expected AssertionFailure, got %T", err)
	}

	if assertionFailure.Step != step {
		t.Error("assertion failure should reference the step")
	}

	if len(assertionFailure.Response) == 0 {
		t.Error("assertion failure should include response body")
	}
}

func TestRunner_StatusCodeMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": "not found"})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("GET", "/missing", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected error for status mismatch, got nil")
	}

	var assertionFailure *AssertionFailure
	if !errors.As(err, &assertionFailure) {
		t.Error("expected AssertionFailure")
	}
}

func TestRunner_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("GET", "/slow", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)
	step.Options.Timeout = Some("50ms")

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Timeout errors are request failures, check that error is present
	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("expected timeout/deadline error, got: %v", err)
	}
}

func TestRunner_NestedBodyBinding(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"token": "abc123"})
		case "/step2":
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step1 := app.NewStep("get-token",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"auth_token": "$.token"},
	)
	runner.Execute(step1)

	step2 := app.NewStep("use-token",
		app.NewRequest("POST", "/step2", map[string]any{
			"auth": map[string]any{
				"token": "{{bind:auth_token}}",
				"type":  "bearer",
			},
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify nested binding was resolved
	if auth, ok := receivedBody["auth"].(map[string]any); ok {
		if auth["token"] != "abc123" {
			t.Errorf("expected token 'abc123', got '%v'", auth["token"])
		}
	} else {
		t.Error("auth object not found in request body")
	}
}

func TestRunner_ArrayBinding(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"id": "item-1"})
		case "/step2":
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &receivedBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step1 := app.NewStep("get-id",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"item_id": "$.id"},
	)
	runner.Execute(step1)

	step2 := app.NewStep("use-id",
		app.NewRequest("POST", "/step2", map[string]any{
			"items": []any{
				"{{bind:item_id}}",
				"static-item",
			},
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify array binding was resolved
	if items, ok := receivedBody["items"].([]any); ok {
		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}
		if items[0] != "item-1" {
			t.Errorf("expected 'item-1', got '%v'", items[0])
		}
	} else {
		t.Error("items array not found in request body")
	}
}

func TestRunner_BindingInAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"expected_value": "dynamic-123"})
		case "/step2":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"actual_value": "dynamic-123"})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step1 := app.NewStep("get-expected",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"expected": "$.expected_value"},
	)
	runner.Execute(step1)

	step2 := app.NewStep("verify",
		app.NewRequest("GET", "/step2", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{
				JsonPath: "$.actual_value",
				Exists:   Some(true),
				Equals:   Some("{{bind:expected}}"),
				Contains: None[string](),
			},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error with matching assertion, got %v", err)
	}
}

func TestRunner_MultipleHTTPMethods(t *testing.T) {
	methodsCalled := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methodsCalled[r.Method] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"method": r.Method})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	methods := []string{"GET", "POST", "PUT", "PATCH", "DELETE"}
	for _, method := range methods {
		step := app.NewStep(fmt.Sprintf("test-%s", method),
			app.NewRequest(method, "/test", nil),
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)

		_, err := runner.Execute(step)
		if err != nil {
			t.Fatalf("method %s failed: %v", method, err)
		}
	}

	for _, method := range methods {
		if !methodsCalled[method] {
			t.Errorf("method %s was not called", method)
		}
	}
}

func TestRunner_StepsRanCounter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	if runner.StepsRan() != 0 {
		t.Errorf("expected 0 steps initially, got %d", runner.StepsRan())
	}

	for i := 1; i <= 5; i++ {
		step := app.NewStep(fmt.Sprintf("step%d", i),
			app.NewRequest("GET", "/test", nil),
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)
		runner.Execute(step)

		if runner.StepsRan() != i {
			t.Errorf("expected %d steps ran, got %d", i, runner.StepsRan())
		}
	}
}

func TestRunner_EmptyResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		// No body
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("DELETE", "/resource", nil),
		app.NewAssert(204, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error with empty body, got %v", err)
	}
}

func TestRunner_ExportFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"data": "value"})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"missing_field": "$.nonexistent.path",
		},
	)

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected error for failed export, got nil")
	}

	// Should NOT be an assertion failure
	var assertionFailure *AssertionFailure
	if errors.As(err, &assertionFailure) {
		t.Error("export failure should not be an AssertionFailure")
	}
}

func TestRunner_UnresolvedBinding(t *testing.T) {
	var receivedBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("test",
		app.NewRequest("POST", "/test", map[string]any{
			"value": "{{bind:nonexistent}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error (unresolved stays as-is), got %v", err)
	}

	// Unresolved bindings stay as-is
	if receivedBody["value"] != "{{bind:nonexistent}}" {
		t.Errorf("expected unresolved binding to stay as-is, got '%v'", receivedBody["value"])
	}
}

func TestRunner_RunIdConsistency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	runId := runner.RunId
	if runId == "" {
		t.Error("RunId should not be empty")
	}

	// Execute multiple steps
	for i := 0; i < 3; i++ {
		step := app.NewStep(fmt.Sprintf("step%d", i),
			app.NewRequest("GET", "/test", nil),
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)
		runner.Execute(step)

		if runner.RunId != runId {
			t.Error("RunId should remain consistent across steps")
		}
	}
}

func TestRunner_XMLRequest(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response><status>success</status></response>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	req := app.Request{
		Method: "POST",
		Path:   "/test",
		Xml:    Some("<user><name>John</name></user>"),
	}

	step := app.NewStep("xml-request",
		req,
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedContentType != "application/xml" {
		t.Errorf("expected Content-Type application/xml, got %s", receivedContentType)
	}

	if receivedBody != "<user><name>John</name></user>" {
		t.Errorf("expected XML body, got %s", receivedBody)
	}
}

func TestRunner_XMLResponseWithXPathAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<user><id>123</id><name>John Doe</name><role>admin</role></user>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("xml-assertion",
		app.NewRequest("GET", "/user", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/user/id", Equals: Some("123")},
			{XPath: "/user/name", Contains: Some("John")},
			{XPath: "/user/role", IsNot: Some("guest")},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_XMLResponseExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response><data><token>abc123</token></data></response>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("xml-exists",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/data/token", Exists: Some(true)},
			{XPath: "/response/missing", Exists: Some(false)},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_XMLExports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<user><id>456</id><token>xyz789</token></user>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("xml-export",
		app.NewRequest("GET", "/user", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"user_id": "/user/id",
			"token":   "/user/token",
		})

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	symtable := runner.symtable
	if symtable["user_id"] != "456" {
		t.Errorf("expected user_id=456, got %v", symtable["user_id"])
	}
	if symtable["token"] != "xyz789" {
		t.Errorf("expected token=xyz789, got %v", symtable["token"])
	}
}

func TestRunner_XMLRequestWithBinding(t *testing.T) {
	var receivedBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response>ok</response>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Manually set an export for testing
	runner.symtable["user_name"] = "Alice"

	// Second step uses binding in XML
	req := app.Request{
		Method: "POST",
		Path:   "/update",
		Xml:    Some("<user><name>{{bind:user_name}}</name><id>{{RUN_ID}}</id></user>"),
	}

	step2 := app.NewStep("step2", req,
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{})

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(receivedBody, "<name>Alice</name>") {
		t.Errorf("expected XML with resolved binding, got %s", receivedBody)
	}
	if !strings.Contains(receivedBody, "<id>"+runner.RunId+"</id>") {
		t.Errorf("expected XML with RUN_ID, got %s", receivedBody)
	}
}

func TestRunner_MixedJSONRequestXMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server accepts JSON, returns XML
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response><converted>true</converted></response>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("mixed",
		app.NewRequest("POST", "/convert", map[string]any{"input": "data"}),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/converted", Equals: Some("true")},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunner_XMLAssertionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<response><status>error</status></response>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("xml-fail",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/status", Equals: Some("success")}, // Will fail
		})),
		app.Exports{})

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected assertion failure")
	}

	var af *AssertionFailure
	if !errors.As(err, &af) {
		t.Error("expected AssertionFailure")
	}
}
