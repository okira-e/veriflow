package run

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/okira-e/veriflow/app"
	"github.com/okira-e/veriflow/app/cli"
	"github.com/okira-e/veriflow/app/cliopts"
	"github.com/okira-e/veriflow/app/config"
	"github.com/okira-e/veriflow/app/engine"
	"github.com/okira-e/veriflow/app/logging"
	. "github.com/okira-e/veriflow/app/opt"
)

func makeTestFlow(name string, steps ...*app.Step) *app.Flow {
	flow := app.NewFlow(name)
	for _, step := range steps {
		flow.AddStep(step)
	}
	return flow
}

func makeTestStep(name, path string) *app.Step {
	return app.NewStep(name,
		app.Request{Method: "GET", Path: path},
		app.NewAssert(200, None[[]*app.Assertion]()),
		app.Exports{})
}

func makeFailingStep(name, path string) *app.Step {
	return app.NewStep(name,
		app.Request{Method: "GET", Path: path},
		app.NewAssert(200, Some([]*app.Assertion{
			{JsonPath: "$.value", Equals: Some("WRONG")},
		})),
		app.Exports{})
}

func TestShowServerResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": "test-123", "status": "success"})
	}))
	defer server.Close()

	cfg := &config.Cfg{BaseUrl: server.URL}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	step := makeTestStep("test-step", "/test")
	flow := makeTestFlow("test-flow", step)

	cliopts.Silent = true

	// With flag enabled
	opts := RunCmdOptions{
		TrimErrorResponse:   true,
		KeepGoing:           false,
		ShowServerResponses: true,
		Printer:             logging.NullPrinter{},
	}

	failures, _, err := executeSteps(runner, []cli.Target{{Flow: flow, Step: step}}, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}

	// With flag disabled
	runner2 := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})
	opts.ShowServerResponses = false

	failures2, _, err2 := executeSteps(runner2, []cli.Target{{Flow: flow, Step: step}}, opts)
	if err2 != nil {
		t.Fatalf("unexpected error: %v", err2)
	}
	if failures2 != 0 {
		t.Errorf("expected 0 failures, got %d", failures2)
	}
}

func TestSkipFlag_SingleStep(t *testing.T) {
	requestsHit := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsHit[r.URL.Path] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	flow := makeTestFlow("test-flow",
		makeTestStep("step1", "/step1"),
		makeTestStep("step2", "/step2"),
		makeTestStep("step3", "/step3"),
	)

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{flow}}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	// Skip step2
	skips := map[string]bool{"test-flow/step2": true}
	targets := []cli.Target{{Flow: flow, Step: nil}}
	stepsToRun := cli.FlattenTargets(targets, skips)

	cliopts.Silent = true
	opts := RunCmdOptions{Printer: logging.NullPrinter{}}

	failures, _, err := executeSteps(runner, stepsToRun, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}

	if !requestsHit["/step1"] {
		t.Error("step1 should have been executed")
	}
	if requestsHit["/step2"] {
		t.Error("step2 should have been skipped")
	}
	if !requestsHit["/step3"] {
		t.Error("step3 should have been executed")
	}
}

func TestSkipFlag_EntireFlow(t *testing.T) {
	requestsHit := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestsHit[r.URL.Path] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	flow1 := makeTestFlow("flow1", makeTestStep("step1", "/flow1/step1"))
	flow2 := makeTestFlow("flow2", makeTestStep("step1", "/flow2/step1"))

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{flow1, flow2}}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	// Skip flow2
	skips := map[string]bool{"flow2/step1": true}
	targets := []cli.Target{{Flow: flow1, Step: nil}, {Flow: flow2, Step: nil}}
	stepsToRun := cli.FlattenTargets(targets, skips)

	cliopts.Silent = true
	opts := RunCmdOptions{Printer: logging.NullPrinter{}}

	failures, _, err := executeSteps(runner, stepsToRun, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}

	if !requestsHit["/flow1/step1"] {
		t.Error("flow1 should have been executed")
	}
	if requestsHit["/flow2/step1"] {
		t.Error("flow2 should have been skipped")
	}
}

func TestParseSkips_InvalidFlow(t *testing.T) {
	cfg := &config.Cfg{
		BaseUrl: "http://localhost",
		Flows:   []*app.Flow{makeTestFlow("existing-flow")},
	}

	_, err := parseSkips(cfg, []string{"non-existent-flow"})
	if err == nil {
		t.Fatal("expected error for non-existent flow in skip")
	}
}

func TestKeepGoingFlag_StopsOnFirstFailure(t *testing.T) {
	stepsHit := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stepsHit[r.URL.Path] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"value": r.URL.Path})
	}))
	defer server.Close()

	flow := makeTestFlow("test-flow",
		makeTestStep("step1", "/step1"),
		makeFailingStep("step2-fails", "/step2"),
		makeTestStep("step3", "/step3"),
	)

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{flow}}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	targets := []cli.Target{{Flow: flow, Step: nil}}
	stepsToRun := cli.FlattenTargets(targets, nil)

	cliopts.Silent = true
	opts := RunCmdOptions{
		KeepGoing: false,
		Printer:   logging.NullPrinter{},
	}

	failures, _, _ := executeSteps(runner, stepsToRun, opts)

	if failures != 1 {
		t.Errorf("expected 1 failure, got %d", failures)
	}
	if !stepsHit["/step1"] {
		t.Error("step1 should have been executed")
	}
	if !stepsHit["/step2"] {
		t.Error("step2 should have been executed (and failed)")
	}
	if stepsHit["/step3"] {
		t.Error("step3 should NOT have been executed")
	}
}

func TestKeepGoingFlag_ContinuesOnFailure(t *testing.T) {
	stepsHit := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stepsHit[r.URL.Path] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"value": r.URL.Path})
	}))
	defer server.Close()

	flow := makeTestFlow("test-flow",
		makeTestStep("step1", "/step1"),
		makeFailingStep("step2-fails", "/step2"),
		makeFailingStep("step3-fails", "/step3"),
		makeTestStep("step4", "/step4"),
	)

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{flow}}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	targets := []cli.Target{{Flow: flow, Step: nil}}
	stepsToRun := cli.FlattenTargets(targets, nil)

	cliopts.Silent = true
	opts := RunCmdOptions{
		KeepGoing: true,
		Printer:   logging.NullPrinter{},
	}

	failures, _, _ := executeSteps(runner, stepsToRun, opts)

	if failures != 2 {
		t.Errorf("expected 2 failures, got %d", failures)
	}
	if !stepsHit["/step1"] {
		t.Error("step1 should have been executed")
	}
	if !stepsHit["/step2"] {
		t.Error("step2 should have been executed")
	}
	if !stepsHit["/step3"] {
		t.Error("step3 should have been executed")
	}
	if !stepsHit["/step4"] {
		t.Error("step4 should have been executed despite failures")
	}
}

func TestSkipFlag_MultipleSkips(t *testing.T) {
	stepsHit := make(map[string]bool)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stepsHit[r.URL.Path] = true
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	flow := makeTestFlow("test-flow",
		makeTestStep("step1", "/step1"),
		makeTestStep("step2", "/step2"),
		makeTestStep("step3", "/step3"),
		makeTestStep("step4", "/step4"),
	)

	cfg := &config.Cfg{BaseUrl: server.URL, Flows: []*app.Flow{flow}}
	runner := engine.NewRunner(engine.RunnerSettings{Cfg: cfg})

	// Skip step2 and step4
	skips := map[string]bool{
		"test-flow/step2": true,
		"test-flow/step4": true,
	}
	targets := []cli.Target{{Flow: flow, Step: nil}}
	stepsToRun := cli.FlattenTargets(targets, skips)

	cliopts.Silent = true
	opts := RunCmdOptions{Printer: logging.NullPrinter{}}

	failures, _, err := executeSteps(runner, stepsToRun, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if failures != 0 {
		t.Errorf("expected 0 failures, got %d", failures)
	}

	if !stepsHit["/step1"] {
		t.Error("step1 should have been executed")
	}
	if stepsHit["/step2"] {
		t.Error("step2 should have been skipped")
	}
	if !stepsHit["/step3"] {
		t.Error("step3 should have been executed")
	}
	if stepsHit["/step4"] {
		t.Error("step4 should have been skipped")
	}
}

func TestParseTarget(t *testing.T) {
	flow := makeTestFlow("my-flow", makeTestStep("my-step", "/test"))
	cfg := &config.Cfg{}
	cfg.AddFlow(flow)

	// Test whole flow
	target, err := cli.ParseTarget(cfg, "my-flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target.Flow.Name != "my-flow" {
		t.Error("wrong flow name")
	}
	if target.Step != nil {
		t.Error("step should be nil for whole flow")
	}

	// Test specific step
	target2, err := cli.ParseTarget(cfg, "my-flow/my-step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target2.Step.Name != "my-step" {
		t.Error("wrong step name")
	}

	// Test invalid
	_, err = cli.ParseTarget(cfg, "/invalid")
	if err == nil {
		t.Error("expected error for invalid target")
	}

	// Test non-existent flow
	_, err = cli.ParseTarget(cfg, "no-such-flow")
	if err == nil {
		t.Error("expected error for non-existent flow")
	}
}
