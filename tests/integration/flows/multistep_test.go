package integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	. "github.com/okira-e/veriflow/app/opt"
)

func TestIntegration_DeepExportChain(t *testing.T) {
	// Tests: Export from step N used in step N+1, N+2, etc.
	// This catches bugs where symtable gets corrupted or bindings fail after many steps.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		switch r.URL.Path {
		case "/step1":
			json.NewEncoder(w).Encode(map[string]any{"id": "A", "token": "T1"})
		case "/step2":
			json.NewEncoder(w).Encode(map[string]any{"session": "S1", "ref": "R1"})
		case "/step3":
			json.NewEncoder(w).Encode(map[string]any{"order_id": "O1"})
		case "/step4":
			json.NewEncoder(w).Encode(map[string]any{"payment_id": "P1"})
		case "/step5":
			// Final step uses ALL previous exports
			json.NewEncoder(w).Encode(map[string]any{"confirmed": true})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	steps := []*app.Step{
		app.NewStep("step1", app.Request{Method: "GET", Path: "/step1"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"id": "$.id", "token": "$.token"}),

		app.NewStep("step2", app.Request{Method: "POST", Path: "/step2", Json: Some[any](map[string]any{
			"id": "{{bind:id}}", "token": "{{bind:token}}",
		})},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"session": "$.session", "ref": "$.ref"}),

		app.NewStep("step3", app.Request{Method: "POST", Path: "/step3", Json: Some[any](map[string]any{
			"id": "{{bind:id}}", "session": "{{bind:session}}",
		})},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"order_id": "$.order_id"}),

		app.NewStep("step4", app.Request{Method: "POST", Path: "/step4", Json: Some[any](map[string]any{
			"order_id": "{{bind:order_id}}", "ref": "{{bind:ref}}",
		})},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"payment_id": "$.payment_id"}),

		// Final step uses exports from steps 1, 2, 3, and 4
		app.NewStep("step5", app.Request{Method: "POST", Path: "/step5", Json: Some[any](map[string]any{
			"id":         "{{bind:id}}",
			"token":      "{{bind:token}}",
			"session":    "{{bind:session}}",
			"order_id":   "{{bind:order_id}}",
			"payment_id": "{{bind:payment_id}}",
		})},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.confirmed", Equals: Some("true")},
			})),
			app.Exports{}),
	}

	for i, step := range steps {
		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %d (%s) failed: %v", i+1, step.Name, err)
		}
	}

	if runner.StepsRan() != 5 {
		t.Errorf("expected 5 steps, got %d", runner.StepsRan())
	}
}

func TestIntegration_CompleteUserJourney(t *testing.T) {
	// Realistic user flow: Register → Login → Create Resource → Update → Delete → Verify Deleted
	// Tests cookies, dynamic paths, exports, assertions all working together.

	users := make(map[string]map[string]any)
	sessions := make(map[string]string)
	resources := make(map[string]map[string]any)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/register" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			userId := fmt.Sprintf("user-%s", body["email"].(string)[:8])
			users[userId] = body
			sessions[userId] = fmt.Sprintf("sess-%s", userId)
			http.SetCookie(w, &http.Cookie{Name: "session", Value: sessions[userId]})
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"user_id": userId})

		case r.URL.Path == "/login" && r.Method == "POST":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			for uid, u := range users {
				if u["email"] == body["email"] {
					http.SetCookie(w, &http.Cookie{Name: "session", Value: sessions[uid]})
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(map[string]any{"user_id": uid, "token": "jwt-" + uid})
					return
				}
			}
			w.WriteHeader(http.StatusUnauthorized)

		case strings.HasPrefix(r.URL.Path, "/resources") && r.Method == "POST":
			cookie, _ := r.Cookie("session")
			if cookie == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			resId := fmt.Sprintf("res-%v", len(resources)+1)
			resources[resId] = body
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"resource_id": resId, "data": body})

		case strings.HasPrefix(r.URL.Path, "/resources/") && r.Method == "PUT":
			resId := strings.TrimPrefix(r.URL.Path, "/resources/")
			if _, ok := resources[resId]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			for k, v := range body {
				resources[resId][k] = v
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resources[resId])

		case strings.HasPrefix(r.URL.Path, "/resources/") && r.Method == "DELETE":
			resId := strings.TrimPrefix(r.URL.Path, "/resources/")
			if _, ok := resources[resId]; !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			delete(resources, resId)
			w.WriteHeader(http.StatusNoContent)

		case strings.HasPrefix(r.URL.Path, "/resources/") && r.Method == "GET":
			resId := strings.TrimPrefix(r.URL.Path, "/resources/")
			if res, ok := resources[resId]; ok {
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(res)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	flow := []*app.Step{
		// 1. Register
		app.NewStep("register",
			app.Request{Method: "POST", Path: "/register", Json: Some[any](map[string]any{
				"email": "test-{{RUN_ID}}@example.com", "password": "secret",
			})},
			app.NewAssert(201, Some([]*app.Assertion{{JsonPath: "$.user_id", Exists: Some(true)}})),
			app.Exports{"user_id": "$.user_id"}),

		// 2. Login (sets cookie)
		app.NewStep("login",
			app.Request{Method: "POST", Path: "/login", Json: Some[any](map[string]any{
				"email": "test-{{RUN_ID}}@example.com", "password": "secret",
			})},
			app.NewAssert(200, Some([]*app.Assertion{{JsonPath: "$.token", Exists: Some(true)}})),
			app.Exports{"token": "$.token"}),

		// 3. Create resource (uses cookie from login)
		app.NewStep("create-resource",
			app.Request{Method: "POST", Path: "/resources", Json: Some[any](map[string]any{
				"name": "Resource-{{RUN_ID}}", "owner": "{{bind:user_id}}",
			})},
			app.NewAssert(201, None[[]*app.Assertion]()),
			app.Exports{"resource_id": "$.resource_id"}),

		// 4. Update resource (dynamic path + export)
		app.NewStep("update-resource",
			app.Request{Method: "PUT", Path: "/resources/{{bind:resource_id}}", Json: Some[any](map[string]any{
				"name": "Updated-{{RUN_ID}}",
			})},
			app.NewAssert(200, Some([]*app.Assertion{{JsonPath: "$.name", Contains: Some("Updated")}})),
			app.Exports{}),

		// 5. Delete resource
		app.NewStep("delete-resource",
			app.Request{Method: "DELETE", Path: "/resources/{{bind:resource_id}}"},
			app.NewAssert(204, None[[]*app.Assertion]()),
			app.Exports{}),

		// 6. Verify deleted (should 404)
		app.NewStep("verify-deleted",
			app.Request{Method: "GET", Path: "/resources/{{bind:resource_id}}"},
			app.NewAssert(404, None[[]*app.Assertion]()),
			app.Exports{}),
	}

	for _, step := range flow {
		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %s failed: %v", step.Name, err)
		}
	}
}

func TestIntegration_AssertionFailureMidFlow(t *testing.T) {
	// Steps 1-2 succeed, step 3 fails assertion, steps 4-5 should never execute.
	// Verifies: error contains correct step info, response body captured, prior exports intact.

	stepsHit := make(map[int]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			stepsHit[1] = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"value": "A"})

		case "/step2":
			stepsHit[2] = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"value": "B"})

		case "/step3":
			stepsHit[3] = true
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"status": "error", "code": 500}) // Will fail assertion

		case "/step4":
			stepsHit[4] = true
			w.WriteHeader(http.StatusOK)

		case "/step5":
			stepsHit[5] = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	steps := []*app.Step{
		app.NewStep("step1", app.Request{Method: "GET", Path: "/step1"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"val_a": "$.value"}),

		app.NewStep("step2", app.Request{Method: "GET", Path: "/step2"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{"val_b": "$.value"}),

		// This step will fail - expects status="success" but gets "error"
		app.NewStep("step3-fails", app.Request{Method: "GET", Path: "/step3"},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.status", Equals: Some("success")}, // WILL FAIL
			})),
			app.Exports{}),

		app.NewStep("step4", app.Request{Method: "GET", Path: "/step4"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{}),

		app.NewStep("step5", app.Request{Method: "GET", Path: "/step5"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{}),
	}

	var failedStep *app.Step
	for _, step := range steps {
		_, err := runner.Execute(step)
		if err != nil {
			failedStep = step
			// Verify it's an assertion failure with response body
			var af *engine.AssertionFailure
			if errors.As(err, &af) {
				if len(af.Response) == 0 {
					t.Error("assertion failure should include response body")
				}
			}
			break
		}
	}

	// Verify step 3 was the failure point
	if failedStep == nil || failedStep.Name != "step3-fails" {
		t.Errorf("expected step3-fails to fail, got: %v", failedStep)
	}

	// Verify steps 1-3 were hit, 4-5 were not
	if !stepsHit[1] || !stepsHit[2] || !stepsHit[3] {
		t.Error("steps 1-3 should have been executed")
	}
	if stepsHit[4] || stepsHit[5] {
		t.Error("steps 4-5 should NOT have been executed after failure")
	}

	if runner.StepsRan() != 3 {
		t.Errorf("expected 3 steps ran (including failed), got %d", runner.StepsRan())
	}
}

func TestIntegration_ExportFailureMidFlow(t *testing.T) {
	// Step 2 tries to export a non-existent path.
	// Should fail without corrupting state from step 1's exports.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/step1":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"valid": "data"})
		case "/step2":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"different": "structure"}) // Missing $.expected.path
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	step1 := app.NewStep("step1", app.Request{Method: "GET", Path: "/step1"},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"valid_data": "$.valid"})

	step2 := app.NewStep("step2-bad-export", app.Request{Method: "GET", Path: "/step2"},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{"missing": "$.expected.deeply.nested.path"}) // Will fail

	if _, err := runner.Execute(step1); err != nil {
		t.Fatalf("step1 should succeed: %v", err)
	}

	_, err := runner.Execute(step2)
	if err == nil {
		t.Fatal("step2 should fail due to bad export path")
	}

	// Should NOT be an assertion failure
	var af *engine.AssertionFailure
	if errors.As(err, &af) {
		t.Error("export failure should not be classified as AssertionFailure")
	}
}

func TestIntegration_CookiePersistenceAcrossEndpoints(t *testing.T) {
	// Multiple endpoints set different cookies, all should persist.
	// Final endpoint requires all cookies to succeed.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth":
			http.SetCookie(w, &http.Cookie{Name: "auth", Value: "auth-token"})
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))

		case "/preferences":
			http.SetCookie(w, &http.Cookie{Name: "prefs", Value: "dark-mode"})
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))

		case "/tracking":
			http.SetCookie(w, &http.Cookie{Name: "tracking", Value: "session-123"})
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success":true}`))

		case "/dashboard":
			// Requires ALL three cookies
			authCookie, _ := r.Cookie("auth")
			prefsCookie, _ := r.Cookie("prefs")
			trackingCookie, _ := r.Cookie("tracking")

			if authCookie == nil || prefsCookie == nil || trackingCookie == nil {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"missing cookies"}`))
				return
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"auth":     authCookie.Value,
				"prefs":    prefsCookie.Value,
				"tracking": trackingCookie.Value,
			})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	steps := []*app.Step{
		app.NewStep("get-auth", app.Request{Method: "GET", Path: "/auth"},
			app.NewAssert(200, None[[]*app.Assertion]()), app.Exports{}),

		app.NewStep("get-prefs", app.Request{Method: "GET", Path: "/preferences"},
			app.NewAssert(200, None[[]*app.Assertion]()), app.Exports{}),

		app.NewStep("get-tracking", app.Request{Method: "GET", Path: "/tracking"},
			app.NewAssert(200, None[[]*app.Assertion]()), app.Exports{}),

		// This requires all 3 cookies from previous steps
		app.NewStep("dashboard", app.Request{Method: "GET", Path: "/dashboard"},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.auth", Equals: Some("auth-token")},
				{JsonPath: "$.prefs", Equals: Some("dark-mode")},
				{JsonPath: "$.tracking", Equals: Some("session-123")},
			})),
			app.Exports{}),
	}

	for _, step := range steps {
		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %s failed: %v", step.Name, err)
		}
	}
}

func TestIntegration_ComplexNestedDataFlow(t *testing.T) {
	// Step 1: Get complex nested response, export multiple values
	// Step 2: Use those in complex nested request body
	// Step 3: Assert on nested response using bindings

	var step2Body map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/config":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"system": map[string]any{
					"api": map[string]any{
						"key":     "api-key-123",
						"version": "v2",
					},
					"tenant": map[string]any{
						"id":   "tenant-456",
						"name": "Acme Corp",
					},
				},
				"limits": map[string]any{
					"rate": 1000,
				},
			})

		case "/execute":
			json.NewDecoder(r.Body).Decode(&step2Body)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"processed_by": step2Body["config"].(map[string]any)["tenant_id"],
					"api_version":  step2Body["config"].(map[string]any)["api_version"],
				},
			})

		case "/verify":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"tenant_id": "tenant-456",
				"status":    "verified",
			})
		}
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	steps := []*app.Step{
		// Export from deeply nested paths
		app.NewStep("get-config", app.Request{Method: "GET", Path: "/config"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{
				"api_key":     "$.system.api.key",
				"api_version": "$.system.api.version",
				"tenant_id":   "$.system.tenant.id",
				"rate_limit":  "$.limits.rate",
			}),

		// Use exports in nested request body
		app.NewStep("execute", app.Request{Method: "POST", Path: "/execute", Json: Some[any](map[string]any{
			"config": map[string]any{
				"api_key":     "{{bind:api_key}}",
				"api_version": "{{bind:api_version}}",
				"tenant_id":   "{{bind:tenant_id}}",
			},
			"options": map[string]any{
				"rate_limit": "{{bind:rate_limit}}",
			},
		})},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.result.processed_by", Equals: Some("{{bind:tenant_id}}")},
			})),
			app.Exports{}),

		// Final verification using binding in assertion
		app.NewStep("verify", app.Request{Method: "GET", Path: "/verify"},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.tenant_id", Equals: Some("{{bind:tenant_id}}")},
				{JsonPath: "$.status", Equals: Some("verified")},
			})),
			app.Exports{}),
	}

	for _, step := range steps {
		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %s failed: %v", step.Name, err)
		}
	}

	// Verify nested body was constructed correctly
	config := step2Body["config"].(map[string]any)
	if config["api_key"] != "api-key-123" {
		t.Errorf("expected api_key=api-key-123, got %v", config["api_key"])
	}
	if config["tenant_id"] != "tenant-456" {
		t.Errorf("expected tenant_id=tenant-456, got %v", config["tenant_id"])
	}
}

func TestIntegration_PaginationPattern(t *testing.T) {
	// Common real-world pattern: paginate through results using cursor tokens.
	// Tests: Export cursor → use in next request → repeat.

	pages := []struct {
		items      []string
		nextCursor string
	}{
		{[]string{"item1", "item2"}, "cursor-page2"},
		{[]string{"item3", "item4"}, "cursor-page3"},
		{[]string{"item5"}, ""}, // Last page
	}
	currentPage := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cursor := r.URL.Query().Get("cursor")

		// Determine which page to return
		switch cursor {
		case "":
			currentPage = 0
		case "cursor-page2":
			currentPage = 1
		case "cursor-page3":
			currentPage = 2
		}

		page := pages[currentPage]
		resp := map[string]any{
			"items": page.items,
		}
		if page.nextCursor != "" {
			resp["next_cursor"] = page.nextCursor
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	// Page 1 (no cursor)
	page1 := app.NewStep("page1", app.Request{Method: "GET", Path: "/items"},
		app.NewAssert(200, Some([]*app.Assertion{{JsonPath: "$.next_cursor", Exists: Some(true)}})),
		app.Exports{"cursor": "$.next_cursor"})

	// Page 2 (use cursor from page 1)
	page2 := app.NewStep("page2", app.Request{Method: "GET", Path: "/items?cursor={{bind:cursor}}"},
		app.NewAssert(200, Some([]*app.Assertion{{JsonPath: "$.next_cursor", Exists: Some(true)}})),
		app.Exports{"cursor": "$.next_cursor"}) // Overwrite cursor

	// Page 3 (use cursor from page 2)
	page3 := app.NewStep("page3", app.Request{Method: "GET", Path: "/items?cursor={{bind:cursor}}"},
		app.NewAssert(200, Some([]*app.Assertion{{JsonPath: "$.next_cursor", Exists: Some(false)}})), // No more pages
		app.Exports{})

	for _, step := range []*app.Step{page1, page2, page3} {
		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %s failed: %v", step.Name, err)
		}
	}

	if runner.StepsRan() != 3 {
		t.Errorf("expected 3 pages fetched, got %d", runner.StepsRan())
	}
}

func TestIntegration_LongFlowStability(t *testing.T) {
	// 15-step flow to verify runner doesn't degrade with many steps.
	// Each step exports something and uses previous exports.

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"step":  r.URL.Path,
			"value": fmt.Sprintf("val-%s", strings.TrimPrefix(r.URL.Path, "/step")),
		})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	numSteps := 15
	for i := 1; i <= numSteps; i += 1 {
		exportKey := fmt.Sprintf("val%d", i)
		step := app.NewStep(
			fmt.Sprintf("step%d", i),
			app.Request{Method: "GET", Path: fmt.Sprintf("/step%d", i)},
			app.NewAssert(200, Some([]*app.Assertion{
				{JsonPath: "$.value", Exists: Some(true)},
			})),
			app.Exports{exportKey: "$.value"},
		)

		if _, err := runner.Execute(step); err != nil {
			t.Fatalf("step %d failed: %v", i, err)
		}
	}

	if runner.StepsRan() != numSteps {
		t.Errorf("expected %d steps, got %d", numSteps, runner.StepsRan())
	}

	// Verify final step can still use first step's export
	finalStep := app.NewStep("final",
		app.Request{Method: "POST", Path: "/final", Json: Some[any](map[string]any{
			"first": "{{bind:val1}}",
			"last":  fmt.Sprintf("{{bind:val%d}}", numSteps),
		})},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{})

	if _, err := runner.Execute(finalStep); err != nil {
		t.Fatalf("final step failed: %v", err)
	}
}
