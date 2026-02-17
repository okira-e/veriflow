package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	. "github.com/okira-e/veriflow/app/opt"
)

type runnerTest struct {
	t      *testing.T
	server *httptest.Server
	runner *Runner
}

func newRunnerTest(t *testing.T, handler http.HandlerFunc) *runnerTest {
	t.Helper()

	server := httptest.NewServer(handler)

	cfg := &config.Cfg{
		BaseUrl: server.URL,
		Flows:   []*app.Flow{},
	}

	return &runnerTest{
		t:      t,
		server: server,
		runner: NewRunner(RunnerSettings{Cfg: cfg}),
	}
}

func (rt *runnerTest) Close() {
	rt.server.Close()
}

func (rt *runnerTest) MustExec(step *app.Step) {
	rt.t.Helper()
	if _, err := rt.runner.Execute(step); err != nil {
		rt.t.Fatalf("step failed: %v", err)
	}
}

func mustDecodeJSON(t *testing.T, r *http.Request, v any) {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed reading body: %v", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("invalid json body: %v", err)
	}
}

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

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any
		mustDecodeJSON(t, r, &data)

		if email, ok := data["email"].(string); ok {
			capturedEmail = email
		}

		if nested, ok := data["nested"].([]any); ok {
			if len(nested) > 0 {
				if obj, ok := nested[0].(map[string]any); ok {
					if val, ok := obj["key"].(string); ok {
						if strings.Contains(val, "{{RUN_ID}}") {
							t.Error("RUN_ID was not replaced in nested key")
						}
					} else {
						t.Error("key field missing in nested object")
					}
				} else {
					t.Error("nested array should contain objects")
				}
			} else {
				t.Error("nested array should not be empty")
			}
		} else {
			t.Error("nested field should be an array")
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("test",
		app.NewRequest("POST", "/test", map[string]any{
			"email": "test-{{RUN_ID}}@example.com",
			"nested": []any{
				map[string]string{
					"key": "value-{{RUN_ID}}",
				},
			},
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if strings.Contains(capturedEmail, "{{RUN_ID}}") {
		t.Fatal("RUN_ID was not replaced in email")
	}

	if !strings.HasPrefix(capturedEmail, "test-") ||
		!strings.HasSuffix(capturedEmail, "@example.com") {
		t.Fatalf("email format incorrect: %s", capturedEmail)
	}

	if capturedEmail != fmt.Sprintf("test-%s@example.com", rt.runner.RunId) {
		t.Fatal("RUN_ID mismatch")
	}
}

func TestRunner_ExportsAndBindings(t *testing.T) {
	var step2Token string

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/register":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user_id": "u123",
				"token":   "secret-token",
			})

		case "/profile":
			body, _ := io.ReadAll(r.Body)

			var data map[string]any
			_ = json.Unmarshal(body, &data)

			step2Token = data["token"].(string)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": "Test User",
			})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("register",
		app.NewRequest("POST", "/register", map[string]any{
			"email": "test@example.com",
		}),
		app.NewAssert(201, None[[]*app.Assertion]()),
		app.Exports{
			"user_token": "$.token",
		},
	))

	rt.MustExec(app.NewStep("get-profile",
		app.NewRequest("POST", "/profile", map[string]any{
			"token": "{{bind:user_token}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if step2Token != "secret-token" {
		t.Fatalf("expected token 'secret-token', got '%s'", step2Token)
	}
}

func TestRunner_DynamicPathBinding(t *testing.T) {
	var capturedPath string

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path

		switch {
		case r.URL.Path == "/users/register":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"user_id": "u456"})
		case strings.HasPrefix(r.URL.Path, "/users/"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "User"})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("register",
		app.NewRequest("POST", "/users/register", nil),
		app.NewAssert(201, None[[]*app.Assertion]()),
		app.Exports{"user_id": "$.user_id"},
	))

	rt.MustExec(app.NewStep("get-user",
		app.NewRequest("GET", "/users/{{bind:user_id}}", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if capturedPath != "/users/u456" {
		t.Fatalf("expected '/users/u456', got '%s'", capturedPath)
	}
}

func TestRunner_NestedExports(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/data":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"user": map[string]any{
						"profile": map[string]any{
							"id": "nested-123",
						},
					},
				},
			})

		case "/verify":
			w.WriteHeader(http.StatusOK)
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("test",
		app.NewRequest("GET", "/data", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"profile_id": "$.data.user.profile.id",
		},
	))

	rt.MustExec(app.NewStep("verify-export",
		app.NewRequest("POST", "/verify", map[string]any{
			"id": "{{bind:profile_id}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))
}

func TestRunner_AssertionFailure(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "error",
		})
	})
	defer rt.Close()

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

	_, err := rt.runner.Execute(step)
	if err == nil {
		t.Fatal("expected assertion failure, got nil")
	}

	var assertionFailure *AssertionFailure
	if !errors.As(err, &assertionFailure) {
		t.Fatalf("expected AssertionFailure, got %T", err)
	}

	if assertionFailure.Step != step {
		t.Fatal("assertion failure should reference the step")
	}

	if len(assertionFailure.Response) == 0 {
		t.Fatal("assertion failure should include response body")
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

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token": "abc123",
			})

		case "/step2":
			mustDecodeJSON(t, r, &receivedBody)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
			})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("get-token",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"auth_token": "$.token"},
	))

	rt.MustExec(app.NewStep("use-token",
		app.NewRequest("POST", "/step2", map[string]any{
			"auth": map[string]any{
				"token": "{{bind:auth_token}}",
				"type":  "bearer",
			},
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if auth, ok := receivedBody["auth"].(map[string]any); ok {
		if auth["token"] != "abc123" {
			t.Fatalf("expected token 'abc123', got '%v'", auth["token"])
		}
	} else {
		t.Fatal("auth object not found in request body")
	}
}

func TestRunner_ChainedBindingsInSameString(t *testing.T) {
	var receivedBody map[string]any

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"a": "foo",
				"b": "bar",
			})

		case "/step2":
			mustDecodeJSON(t, r, &receivedBody)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("export-values",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"a": "$.a",
			"b": "$.b",
		},
	))

	rt.MustExec(app.NewStep("use-chained",
		app.NewRequest("POST", "/step2", map[string]any{
			"value": "{{bind:a}}-{{bind:b}}-{{RUN_ID}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	val, ok := receivedBody["value"].(string)
	if !ok {
		t.Fatal("value field missing or not string")
	}

	expected := fmt.Sprintf("foo-bar-%s", rt.runner.RunId)
	if val != expected {
		t.Fatalf("expected '%s', got '%s'", expected, val)
	}
}

func TestRunner_ArrayBinding(t *testing.T) {
	var receivedBody map[string]any

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id": "item-1",
			})

		case "/step2":
			mustDecodeJSON(t, r, &receivedBody)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
			})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("get-id",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"item_id": "$.id"},
	))

	rt.MustExec(app.NewStep("use-id",
		app.NewRequest("POST", "/step2", map[string]any{
			"items": []any{
				"{{bind:item_id}}",
				"static-item",
			},
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	items, ok := receivedBody["items"].([]any)
	if !ok {
		t.Fatal("items array not found in request body")
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0] != "item-1" {
		t.Fatalf("expected 'item-1', got '%v'", items[0])
	}
}

func TestRunner_BindingInAssertion(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"expected_value": "dynamic-123",
			})

		case "/step2":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"actual_value": "dynamic-123",
			})
		}
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("get-expected",
		app.NewRequest("GET", "/step1", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"expected": "$.expected_value"},
	))

	rt.MustExec(app.NewStep("verify",
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
	))
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

	for i := 1; i <= 5; i += 1 {
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
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	})
	defer rt.Close()

	t.Run("DELETE with 204 No Content", func(t *testing.T) {
		rt.MustExec(app.NewStep("test",
			app.NewRequest("DELETE", "/resource", nil),
			app.NewAssert(204, None[[]*app.Assertion]()),
			app.Exports{},
		))
	})

	t.Run("Unexpected empty body with 200 OK", func(t *testing.T) {
		step := app.NewStep("test",
			app.NewRequest("GET", "/resource", nil),
			app.NewAssert(
				200,
				Some([]*app.Assertion{
					{
						JsonPath: "$.data",
						Exists:   Some(true),
					},
				}),
			),
			app.Exports{},
		)

		_, err := rt.runner.Execute(step)
		if err == nil {
			t.Fatal("expected assertion failure with empty body")
		}

		if _, ok := err.(*AssertionFailure); !ok {
			t.Fatalf("expected AssertionFailure, got %T", err)
		}
	})
}

func TestRunner_ExportFailure(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": "value",
		})
	})
	defer rt.Close()

	step := app.NewStep("test",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"missing_field": "$.nonexistent.path",
		},
	)

	_, err := rt.runner.Execute(step)
	if err == nil {
		t.Fatal("expected error for failed export, got nil")
	}

	var assertionFailure *AssertionFailure
	if errors.As(err, &assertionFailure) {
		t.Fatal("export failure should not be an AssertionFailure")
	}
}

func TestRunner_UnresolvedBinding(t *testing.T) {
	var receivedBody map[string]any

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		mustDecodeJSON(t, r, &receivedBody)

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("test",
		app.NewRequest("POST", "/test", map[string]any{
			"value": "{{bind:nonexistent}}",
		}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if receivedBody["value"] != "{{bind:nonexistent}}" {
		t.Fatalf("expected unresolved binding to stay as-is, got '%v'",
			receivedBody["value"])
	}
}

func TestRunner_RunIdConsistency(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	defer rt.Close()

	runId := rt.runner.RunId
	if runId == "" {
		t.Fatal("RunId should not be empty")
	}

	for i := 0; i < 3; i += 1 {
		rt.MustExec(app.NewStep(fmt.Sprintf("step%d", i),
			app.NewRequest("GET", "/test", nil),
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		))

		if rt.runner.RunId != runId {
			t.Fatal("RunId should remain consistent across steps")
		}
	}
}

func TestRunner_XMLRequest(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")

		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<response><status>success</status></response>`))
	})
	defer rt.Close()

	req := app.Request{
		Method: "POST",
		Path:   "/test",
		Xml:    Some("<user><name>John</name></user>"),
	}

	rt.MustExec(app.NewStep("xml-request",
		req,
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if receivedContentType != "application/xml" {
		t.Fatalf("expected Content-Type application/xml, got %s", receivedContentType)
	}

	if receivedBody != "<user><name>John</name></user>" {
		t.Fatalf("expected XML body, got %s", receivedBody)
	}
}

func TestRunner_XMLResponseWithXPathAssertion(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`<user><id>123</id><name>John Doe</name><role>admin</role></user>`,
		))
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("xml-assertion",
		app.NewRequest("GET", "/user", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/user/id", Equals: Some("123")},
			{XPath: "/user/name", Contains: Some("John")},
			{XPath: "/user/role", IsNot: Some("guest")},
		})),
		app.Exports{},
	))
}

func TestRunner_XMLResponseExists(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`<response><data><token>abc123</token></data></response>`,
		))
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("xml-exists",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/data/token", Exists: Some(true)},
			{XPath: "/response/missing", Exists: Some(false)},
		})),
		app.Exports{},
	))
}

func TestRunner_XMLExports(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`<user><id>456</id><token>xyz789</token></user>`,
		))
	})
	defer rt.Close()

	step := app.NewStep("xml-export",
		app.NewRequest("GET", "/user", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{
			"user_id": "/user/id",
			"token":   "/user/token",
		},
	)

	_, err := rt.runner.Execute(step)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	symtable := rt.runner.symtable

	if symtable["user_id"] != "456" {
		t.Fatalf("expected user_id=456, got %v", symtable["user_id"])
	}

	if symtable["token"] != "xyz789" {
		t.Fatalf("expected token=xyz789, got %v", symtable["token"])
	}
}

func TestRunner_XMLRequestWithBinding(t *testing.T) {
	var receivedBody string

	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<response>ok</response>`))
	})
	defer rt.Close()

	// manually seed export
	rt.runner.symtable["user_name"] = "Alice"

	req := app.Request{
		Method: "POST",
		Path:   "/update",
		Xml: Some(
			"<user><name>{{bind:user_name}}</name><id>{{RUN_ID}}</id></user>",
		),
	}

	rt.MustExec(app.NewStep("step2",
		req,
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	))

	if !strings.Contains(receivedBody, "<name>Alice</name>") {
		t.Fatalf("expected XML with resolved binding, got %s", receivedBody)
	}

	if !strings.Contains(receivedBody, "<id>"+rt.runner.RunId+"</id>") {
		t.Fatalf("expected XML with RUN_ID, got %s", receivedBody)
	}
}

func TestRunner_MixedJSONRequestXMLResponse(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		// Accept JSON, return XML
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`<response><converted>true</converted></response>`,
		))
	})
	defer rt.Close()

	rt.MustExec(app.NewStep("mixed",
		app.NewRequest("POST", "/convert", map[string]any{
			"input": "data",
		}),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/converted", Equals: Some("true")},
		})),
		app.Exports{},
	))
}

func TestRunner_XMLAssertionFailure(t *testing.T) {
	rt := newRunnerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`<response><status>error</status></response>`,
		))
	})
	defer rt.Close()

	step := app.NewStep("xml-fail",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{XPath: "/response/status", Equals: Some("success")},
		})),
		app.Exports{},
	)

	_, err := rt.runner.Execute(step)
	if err == nil {
		t.Fatal("expected assertion failure")
	}

	var af *AssertionFailure
	if !errors.As(err, &af) {
		t.Fatalf("expected AssertionFailure, got %T", err)
	}
}

func TestRunner_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"success": true})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("custom-headers",
		app.Request{
			Method: "POST",
			Path:   "/api/test",
			Json:   Some(map[string]any{"data": "value"}),
			Headers: Some(map[string]string{
				"X-Custom-Header": "custom-value",
				"Authorization":   "Bearer token123",
				"X-Request-ID":    "req-456",
			}),
		},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("expected X-Custom-Header=custom-value, got %s", receivedHeaders.Get("X-Custom-Header"))
	}
	if receivedHeaders.Get("Authorization") != "Bearer token123" {
		t.Errorf("expected Authorization=Bearer token123, got %s", receivedHeaders.Get("Authorization"))
	}
	if receivedHeaders.Get("X-Request-ID") != "req-456" {
		t.Errorf("expected X-Request-ID=req-456, got %s", receivedHeaders.Get("X-Request-ID"))
	}
}

func TestRunner_CustomHeadersWithBinding(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"token": "secret-token-abc"})
		case "/protected":
			receivedAuth = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"data": "protected"})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Step 1: Get token
	step1 := app.NewStep("login",
		app.NewRequest("POST", "/login", map[string]any{"username": "user"}),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"auth_token": "$.token"},
	)
	runner.Execute(step1)

	// Step 2: Use token in header
	step2 := app.NewStep("get-data",
		app.Request{
			Method: "GET",
			Path:   "/protected",
			Headers: Some(map[string]string{
				"Authorization": "Bearer {{bind:auth_token}}",
			}),
		},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedAuth != "Bearer secret-token-abc" {
		t.Errorf("expected Authorization='Bearer secret-token-abc', got %s", receivedAuth)
	}
}

func TestRunner_DisableHeaders(t *testing.T) {
	var cookieReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/set-cookie":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123"})
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true})
		case "/test":
			_, err := r.Cookie("session")
			cookieReceived = (err == nil)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"success": true})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Set a cookie
	step1 := app.NewStep("set-cookie",
		app.NewRequest("GET", "/set-cookie", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)
	runner.Execute(step1)

	// With DisableHeaders=true, cookie jar should be disabled
	step2 := app.NewStep("no-cookies",
		app.Request{
			Method:         "GET",
			Path:           "/test",
			DisableHeaders: true,
		},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cookieReceived {
		t.Error("cookie should NOT have been sent with DisableHeaders=true")
	}
}

func TestRunner_QueryParameters(t *testing.T) {
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"page":  r.URL.Query().Get("page"),
			"limit": r.URL.Query().Get("limit"),
		})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("query-params",
		app.NewRequest("GET", "/api/items?page=2&limit=50&sort=name", nil),
		app.NewAssert(200, Some([]*app.Assertion{
			{JsonPath: "$.page", Equals: Some("2")},
			{JsonPath: "$.limit", Equals: Some("50")},
		})),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(receivedQuery, "page=2") {
		t.Error("query should contain page=2")
	}
	if !strings.Contains(receivedQuery, "limit=50") {
		t.Error("query should contain limit=50")
	}
}

func TestRunner_QueryParametersWithBinding(t *testing.T) {
	var receivedPage string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"default_page": "5"})
		case "/items":
			receivedPage = r.URL.Query().Get("page")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"items": []string{"a", "b"}})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	// Get page number from config
	step1 := app.NewStep("get-config",
		app.NewRequest("GET", "/config", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"page_num": "$.default_page"},
	)
	runner.Execute(step1)

	// Use it in query params
	step2 := app.NewStep("get-items",
		app.NewRequest("GET", "/items?page={{bind:page_num}}&limit=10", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step2)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedPage != "5" {
		t.Errorf("expected page=5 in query, got page=%s", receivedPage)
	}
}

func TestRunner_HTMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html><html><body><h1>Hello World</h1></body></html>`))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("html-response",
		app.NewRequest("GET", "/page", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	response, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(string(response), "Hello World") {
		t.Error("response should contain HTML content")
	}
}

func TestRunner_PlainTextResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Plain text response"))
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("text-response",
		app.NewRequest("GET", "/text", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	response, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if string(response) != "Plain text response" {
		t.Errorf("expected 'Plain text response', got %s", string(response))
	}
}

func TestRunner_NetworkConnectionRefused(t *testing.T) {
	// Use a port that's unlikely to be in use
	cfg := &config.Cfg{BaseUrl: "http://localhost:59999", Flows: []*app.Flow{}}
	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("connection-refused",
		app.NewRequest("GET", "/test", nil),
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected error for connection refused, got nil")
	}

	// Should be a request execution error, not an assertion failure
	var assertionFailure *AssertionFailure
	if errors.As(err, &assertionFailure) {
		t.Error("connection error should NOT be an AssertionFailure")
	}

	if !strings.Contains(err.Error(), "connection refused") && !strings.Contains(err.Error(), "connect") {
		t.Errorf("error should mention connection issue, got: %v", err)
	}
}

func TestRunner_MultipleFileUpload(t *testing.T) {
	var receivedFileCount int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedFileCount = len(r.MultipartForm.File)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"count": receivedFileCount})
	}))
	defer server.Close()

	// Create temporary test files
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("content1"), 0644)
	os.WriteFile(file2, []byte("content2"), 0644)

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	cfg.ConfigFilePath = filepath.Join(tmpDir, "veriflow.json")

	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("upload-multiple",
		app.Request{
			Method: "POST",
			Path:   "/upload",
			Files: Some(map[string]string{
				"document1": "file1.txt",
				"document2": "file2.txt",
			}),
		},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if receivedFileCount != 2 {
		t.Errorf("expected 2 files uploaded, got %d", receivedFileCount)
	}
}

func TestRunner_FileUploadNonExistent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{}}
	cfg.ConfigFilePath = filepath.Join(tmpDir, "veriflow.json")

	runner := NewRunner(RunnerSettings{Cfg: cfg})

	step := app.NewStep("upload-missing",
		app.Request{
			Method: "POST",
			Path:   "/upload",
			Files:  Some(map[string]string{"document": "nonexistent.txt"}),
		},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{},
	)

	_, err := runner.Execute(step)
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention file not found, got: %v", err)
	}
}
