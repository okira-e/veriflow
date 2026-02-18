package config

import (
	"testing"

	"github.com/okira-e/veriflow/app"
	. "github.com/okira-e/veriflow/app/opt"
)

func TestCfgAddFlow(t *testing.T) {
	t.Run("add new flow successfully", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		flow := app.NewFlow("test-flow")
		err := cfg.AddFlow(flow)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if len(cfg.Flows) != 1 {
			t.Errorf("expected 1 flow, got %d", len(cfg.Flows))
		}

		retrievedFlow, ok := cfg.GetFlow("test-flow")
		if !ok {
			t.Error("flow not found after adding")
		}

		if retrievedFlow.Name != "test-flow" {
			t.Errorf("expected flow name 'test-flow', got '%s'", retrievedFlow.Name)
		}
	})

	t.Run("add duplicate flow returns error", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{app.NewFlow("existing-flow")},
		}
		cfg.buildFlowsIndex()

		duplicateFlow := app.NewFlow("existing-flow")
		err := cfg.AddFlow(duplicateFlow)

		if err == nil {
			t.Error("expected error for duplicate flow, got nil")
		}

		if len(cfg.Flows) != 1 {
			t.Errorf("expected 1 flow (original), got %d", len(cfg.Flows))
		}
	})

	t.Run("add multiple flows", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		flow1 := app.NewFlow("flow-1")
		flow2 := app.NewFlow("flow-2")
		flow3 := app.NewFlow("flow-3")

		_ = cfg.AddFlow(flow1)
		_ = cfg.AddFlow(flow2)
		_ = cfg.AddFlow(flow3)

		if len(cfg.Flows) != 3 {
			t.Errorf("expected 3 flows, got %d", len(cfg.Flows))
		}

		// Verify all flows are accessible
		if _, ok := cfg.GetFlow("flow-1"); !ok {
			t.Error("flow-1 not found")
		}
		if _, ok := cfg.GetFlow("flow-2"); !ok {
			t.Error("flow-2 not found")
		}
		if _, ok := cfg.GetFlow("flow-3"); !ok {
			t.Error("flow-3 not found")
		}
	})
}

func TestCfgRemoveFlow(t *testing.T) {
	t.Run("remove existing flow", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		flow1 := app.NewFlow("flow-1")
		flow2 := app.NewFlow("flow-2")

		_ = cfg.AddFlow(flow1)
		_ = cfg.AddFlow(flow2)

		err := cfg.RemoveFlow("flow-1")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if len(cfg.Flows) != 1 {
			t.Errorf("expected 1 flow remaining, got %d", len(cfg.Flows))
		}

		if _, ok := cfg.GetFlow("flow-1"); ok {
			t.Error("flow-1 should have been removed")
		}

		if _, ok := cfg.GetFlow("flow-2"); !ok {
			t.Error("flow-2 should still exist")
		}
	})

	t.Run("remove non-existent flow returns error", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}
		existingFlow := app.NewFlow("existing-flow")

		_ = cfg.AddFlow(existingFlow)

		err := cfg.RemoveFlow("non-existent")

		if err == nil {
			t.Error("expected error for non-existent flow, got nil")
		}

		if len(cfg.Flows) != 1 {
			t.Errorf("expected 1 flow (unchanged), got %d", len(cfg.Flows))
		}
	})

	t.Run("remove all flows", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}
		flow1 := app.NewFlow("flow-1")
		flow2 := app.NewFlow("flow-2")

		_ = cfg.AddFlow(flow1)
		_ = cfg.AddFlow(flow2)

		_ = cfg.RemoveFlow("flow-1")
		_ = cfg.RemoveFlow("flow-2")

		if len(cfg.Flows) != 0 {
			t.Errorf("expected 0 flows, got %d", len(cfg.Flows))
		}
	})
}

func TestCfgUpdateFlow(t *testing.T) {
	t.Run("update existing flow", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		originalFlow := app.NewFlow("test-flow")

		_ = cfg.AddFlow(originalFlow)

		// Create updated flow with same name but different steps
		updatedFlow := app.NewFlow("test-flow")
		step := app.NewStep("new-step",
			app.Request{Method: "POST", Path: "/test"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)
		_ = updatedFlow.AddStep(step)

		err := cfg.UpdateFlow(updatedFlow)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		retrievedFlow, ok := cfg.GetFlow("test-flow")
		if !ok {
			t.Fatal("flow not found after update")
		}

		if len(retrievedFlow.Steps) != 1 {
			t.Errorf("expected 1 step in updated flow, got %d", len(retrievedFlow.Steps))
		}

		if retrievedFlow.Steps[0].Name != "new-step" {
			t.Errorf("expected step name 'new-step', got '%s'", retrievedFlow.Steps[0].Name)
		}
	})
}

func TestCfgGetFlow(t *testing.T) {
	t.Run("get existing flow", func(t *testing.T) {

		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		flow := app.NewFlow("test-flow")

		_ = cfg.AddFlow(flow)

		retrievedFlow, ok := cfg.GetFlow("test-flow")

		if !ok {
			t.Error("expected flow to be found")
		}

		if retrievedFlow.Name != "test-flow" {
			t.Errorf("expected flow name 'test-flow', got '%s'", retrievedFlow.Name)
		}
	})

	t.Run("get non-existent flow", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}

		_, ok := cfg.GetFlow("non-existent")

		if ok {
			t.Error("expected flow not to be found")
		}
	})
}

func TestCfgGetTotalSteps(t *testing.T) {
	t.Run("calculate total steps across flows", func(t *testing.T) {
		flow1 := app.NewFlow("flow-1")
		step1 := app.NewStep("step1",
			app.Request{Method: "GET", Path: "/test"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)
		step2 := app.NewStep("step2",
			app.Request{Method: "POST", Path: "/test"},
			app.NewAssert(201, None[[]*app.Assertion]()),
			app.Exports{},
		)
		_ = flow1.AddStep(step1)
		_ = flow1.AddStep(step2)

		flow2 := app.NewFlow("flow-2")
		step3 := app.NewStep("step3",
			app.Request{Method: "GET", Path: "/other"},
			app.NewAssert(200, None[[]*app.Assertion]()),
			app.Exports{},
		)
		_ = flow2.AddStep(step3)

		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{},
		}
		_ = cfg.AddFlow(flow1)
		_ = cfg.AddFlow(flow2)

		totalSteps := cfg.GetTotalSteps()

		if totalSteps != 3 {
			t.Errorf("expected 3 total steps, got %d", totalSteps)
		}
	})

	t.Run("zero steps", func(t *testing.T) {
		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{app.NewFlow("empty-flow")},
		}

		totalSteps := cfg.GetTotalSteps()

		if totalSteps != 0 {
			t.Errorf("expected 0 total steps, got %d", totalSteps)
		}
	})
}

func TestCfgBuildFlowsIndex(t *testing.T) {
	t.Run("index is built correctly", func(t *testing.T) {
		flow1 := app.NewFlow("flow-1")
		flow2 := app.NewFlow("flow-2")
		flow3 := app.NewFlow("flow-3")

		cfg := &Cfg{
			ProjectName: "test",
			BaseUrl:     "http://localhost",
			Flows:       []*app.Flow{flow1, flow2, flow3},
		}

		cfg.buildFlowsIndex()

		// Verify index has correct size
		if len(cfg.flowsIdx) != 3 {
			t.Errorf("expected index size 3, got %d", len(cfg.flowsIdx))
		}

		// Verify each flow is in correct position
		if idx, ok := cfg.flowsIdx["flow-1"]; !ok || idx != 0 {
			t.Errorf("expected flow-1 at index 0, got index %d, ok=%v", idx, ok)
		}

		if idx, ok := cfg.flowsIdx["flow-2"]; !ok || idx != 1 {
			t.Errorf("expected flow-2 at index 1, got index %d, ok=%v", idx, ok)
		}

		if idx, ok := cfg.flowsIdx["flow-3"]; !ok || idx != 2 {
			t.Errorf("expected flow-3 at index 2, got index %d, ok=%v", idx, ok)
		}
	})
}
